package service_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/repository"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
	"github.com/alice101-dev/ai-code-reviewer/internal/testutil"
)

var testLimits = service.PaymentLimits{
	MaxTopUpAmount:    1_000_000,
	MaxTransferAmount: 500_000,
	MaxBalance:        2_000_000,
}

type paymentFixture struct {
	db           *gorm.DB
	users        *repository.UserRepository
	wallets      *repository.WalletRepository
	transactions *repository.TransactionRepository
	walletSvc    *service.WalletService
	paymentSvc   *service.PaymentService
}

func newPaymentFixture(t *testing.T) *paymentFixture {
	t.Helper()

	db := testutil.NewPostgres(t)
	users := repository.NewUserRepository(db)
	wallets := repository.NewWalletRepository(db)
	transactions := repository.NewTransactionRepository(db)

	return &paymentFixture{
		db:           db,
		users:        users,
		wallets:      wallets,
		transactions: transactions,
		walletSvc:    service.NewWalletService(wallets, transactions, []string{"IDR", "USD"}),
		paymentSvc:   service.NewPaymentService(database.NewTxManager(db), wallets, transactions, testLimits),
	}
}

func (f *paymentFixture) createUser(t *testing.T) *model.User {
	t.Helper()

	user := &model.User{
		Email:        uuid.NewString() + "@example.com",
		PasswordHash: "unused",
		FullName:     "Test User",
		Status:       model.UserStatusActive,
	}
	require.NoError(t, f.users.Create(context.Background(), user))
	return user
}

func (f *paymentFixture) createWallet(t *testing.T, userID uuid.UUID, currency string, balance int64) *model.Wallet {
	t.Helper()

	ctx := context.Background()
	wallet, err := f.walletSvc.Create(ctx, userID, currency)
	require.NoError(t, err)

	for remaining := balance; remaining > 0; {
		amount := min(remaining, testLimits.MaxTopUpAmount)
		_, err := f.paymentSvc.TopUp(ctx, service.TopUpInput{
			UserID:         userID,
			WalletID:       wallet.ID,
			Amount:         amount,
			IdempotencyKey: "seed-" + uuid.NewString(),
		})
		require.NoError(t, err)
		remaining -= amount
	}

	return wallet
}

func (f *paymentFixture) balance(t *testing.T, wallet *model.Wallet) int64 {
	t.Helper()

	current, err := f.wallets.FindByIDForUser(context.Background(), wallet.ID, wallet.UserID)
	require.NoError(t, err)
	return current.Balance
}

func TestPaymentService_TransferMovesFundsAndRecordsLedger(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	source := f.createWallet(t, alice.ID, "IDR", 100_000)
	destination := f.createWallet(t, bob.ID, "IDR", 0)

	result, err := f.paymentSvc.Transfer(ctx, service.TransferInput{
		UserID:              alice.ID,
		SourceWalletID:      source.ID,
		DestinationWalletID: destination.ID,
		Amount:              40_000,
		Description:         "  dinner  ",
		IdempotencyKey:      "transfer-1",
	})
	require.NoError(t, err)
	assert.False(t, result.Replayed)
	assert.Equal(t, "dinner", result.Transaction.Description)

	assert.Equal(t, int64(60_000), f.balance(t, source))
	assert.Equal(t, int64(40_000), f.balance(t, destination))

	sourceActivity, err := f.walletSvc.ListActivity(ctx, alice.ID, source.ID, service.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, sourceActivity.Items, 2)
	assert.Equal(t, int64(2), sourceActivity.TotalItems)

	latest := sourceActivity.Items[0]
	assert.Equal(t, model.EntryDirectionDebit, latest.Direction)
	assert.Equal(t, int64(60_000), latest.BalanceAfter)
	require.NotNil(t, latest.CounterpartyWalletID)
	assert.Equal(t, destination.ID, *latest.CounterpartyWalletID)

	destinationActivity, err := f.walletSvc.ListActivity(ctx, bob.ID, destination.ID, service.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, destinationActivity.Items, 1)
	assert.Equal(t, model.EntryDirectionCredit, destinationActivity.Items[0].Direction)
	require.NotNil(t, destinationActivity.Items[0].CounterpartyWalletID)
	assert.Equal(t, source.ID, *destinationActivity.Items[0].CounterpartyWalletID)
}

func TestPaymentService_TransferValidation(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	aliceIDR := f.createWallet(t, alice.ID, "IDR", 10_000)
	aliceUSD := f.createWallet(t, alice.ID, "USD", 10_000)
	bobIDR := f.createWallet(t, bob.ID, "IDR", 10_000)

	tests := []struct {
		name    string
		input   service.TransferInput
		wantErr error
	}{
		{
			name:    "insufficient funds",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: aliceIDR.ID, DestinationWalletID: bobIDR.ID, Amount: 10_001},
			wantErr: service.ErrInsufficientFunds,
		},
		{
			name:    "source wallet owned by another user",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: bobIDR.ID, DestinationWalletID: aliceIDR.ID, Amount: 1},
			wantErr: service.ErrWalletNotFound,
		},
		{
			name:    "unknown destination wallet",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: aliceIDR.ID, DestinationWalletID: uuid.New(), Amount: 1},
			wantErr: service.ErrDestinationWalletNotFound,
		},
		{
			name:    "currency mismatch",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: aliceIDR.ID, DestinationWalletID: aliceUSD.ID, Amount: 1},
			wantErr: service.ErrCurrencyMismatch,
		},
		{
			name:    "same wallet",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: aliceIDR.ID, DestinationWalletID: aliceIDR.ID, Amount: 1},
			wantErr: service.ErrSameWallet,
		},
		{
			name:    "amount above limit",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: aliceIDR.ID, DestinationWalletID: bobIDR.ID, Amount: testLimits.MaxTransferAmount + 1},
			wantErr: service.ErrAmountOutOfRange,
		},
		{
			name:    "non positive amount",
			input:   service.TransferInput{UserID: alice.ID, SourceWalletID: aliceIDR.ID, DestinationWalletID: bobIDR.ID, Amount: 0},
			wantErr: service.ErrAmountOutOfRange,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.IdempotencyKey = fmt.Sprintf("validation-%d", i)
			_, err := f.paymentSvc.Transfer(ctx, tt.input)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	assert.Equal(t, int64(10_000), f.balance(t, aliceIDR))
	assert.Equal(t, int64(10_000), f.balance(t, aliceUSD))
	assert.Equal(t, int64(10_000), f.balance(t, bobIDR))
}

func TestPaymentService_TransferRejectsInactiveWallet(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	source := f.createWallet(t, alice.ID, "IDR", 10_000)
	destination := f.createWallet(t, bob.ID, "IDR", 0)

	require.NoError(t, f.db.Model(&model.Wallet{}).Where("id = ?", destination.ID).Update("status", model.WalletStatusFrozen).Error)

	_, err := f.paymentSvc.Transfer(ctx, service.TransferInput{
		UserID:              alice.ID,
		SourceWalletID:      source.ID,
		DestinationWalletID: destination.ID,
		Amount:              1_000,
		IdempotencyKey:      "frozen-destination",
	})
	require.ErrorIs(t, err, service.ErrWalletNotActive)
	assert.Equal(t, int64(10_000), f.balance(t, source))
}

func TestPaymentService_TransferIsIdempotent(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	source := f.createWallet(t, alice.ID, "IDR", 100_000)
	destination := f.createWallet(t, bob.ID, "IDR", 0)

	input := service.TransferInput{
		UserID:              alice.ID,
		SourceWalletID:      source.ID,
		DestinationWalletID: destination.ID,
		Amount:              30_000,
		IdempotencyKey:      "transfer-idempotent",
	}

	first, err := f.paymentSvc.Transfer(ctx, input)
	require.NoError(t, err)
	assert.False(t, first.Replayed)

	second, err := f.paymentSvc.Transfer(ctx, input)
	require.NoError(t, err)
	assert.True(t, second.Replayed)
	assert.Equal(t, first.Transaction.ID, second.Transaction.ID)

	assert.Equal(t, int64(70_000), f.balance(t, source))
	assert.Equal(t, int64(30_000), f.balance(t, destination))

	input.Amount = 1
	_, err = f.paymentSvc.Transfer(ctx, input)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyReused)
}

func TestPaymentService_ConcurrentTransfersNeverOverdraw(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	source := f.createWallet(t, alice.ID, "IDR", 1_000)
	destination := f.createWallet(t, bob.ID, "IDR", 0)

	const attempts = 25
	var (
		wg        sync.WaitGroup
		succeeded atomic.Int64
		rejected  atomic.Int64
	)

	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.paymentSvc.Transfer(ctx, service.TransferInput{
				UserID:              alice.ID,
				SourceWalletID:      source.ID,
				DestinationWalletID: destination.ID,
				Amount:              100,
				IdempotencyKey:      fmt.Sprintf("concurrent-%d", i),
			})
			switch {
			case err == nil:
				succeeded.Add(1)
			case assert.ErrorIs(t, err, service.ErrInsufficientFunds):
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(10), succeeded.Load())
	assert.Equal(t, int64(attempts-10), rejected.Load())
	assert.Equal(t, int64(0), f.balance(t, source))
	assert.Equal(t, int64(1_000), f.balance(t, destination))
}

func TestPaymentService_ConcurrentRequestsWithSameIdempotencyKeyApplyOnce(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 0)

	const attempts = 10
	var (
		wg       sync.WaitGroup
		created  atomic.Int64
		replayed atomic.Int64
	)

	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := f.paymentSvc.TopUp(ctx, service.TopUpInput{
				UserID:         alice.ID,
				WalletID:       wallet.ID,
				Amount:         5_000,
				IdempotencyKey: "same-key-topup",
			})
			if !assert.NoError(t, err) {
				return
			}
			if result.Replayed {
				replayed.Add(1)
			} else {
				created.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(1), created.Load())
	assert.Equal(t, int64(attempts-1), replayed.Load())
	assert.Equal(t, int64(5_000), f.balance(t, wallet))
}

func TestPaymentService_TopUpEnforcesLimits(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", testLimits.MaxBalance-100)

	_, err := f.paymentSvc.TopUp(ctx, service.TopUpInput{
		UserID: alice.ID, WalletID: wallet.ID, Amount: 101, IdempotencyKey: "over-balance",
	})
	require.ErrorIs(t, err, service.ErrBalanceLimitExceeded)

	_, err = f.paymentSvc.TopUp(ctx, service.TopUpInput{
		UserID: alice.ID, WalletID: wallet.ID, Amount: testLimits.MaxTopUpAmount + 1, IdempotencyKey: "over-topup",
	})
	require.ErrorIs(t, err, service.ErrAmountOutOfRange)

	_, err = f.paymentSvc.TopUp(ctx, service.TopUpInput{
		UserID: bob.ID, WalletID: wallet.ID, Amount: 1, IdempotencyKey: "foreign-wallet",
	})
	require.ErrorIs(t, err, service.ErrWalletNotFound)

	assert.Equal(t, testLimits.MaxBalance-100, f.balance(t, wallet))
}

func TestWalletService_CreateRejectsDuplicateAndUnsupportedCurrency(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)

	wallet, err := f.walletSvc.Create(ctx, alice.ID, " idr ")
	require.NoError(t, err)
	assert.Equal(t, "IDR", wallet.Currency)

	_, err = f.walletSvc.Create(ctx, alice.ID, "IDR")
	require.ErrorIs(t, err, service.ErrWalletAlreadyExists)

	_, err = f.walletSvc.Create(ctx, alice.ID, "EUR")
	require.ErrorIs(t, err, service.ErrUnsupportedCurrency)
}

func TestWalletService_ListActivityRequiresOwnership(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 1_000)

	_, err := f.walletSvc.ListActivity(ctx, bob.ID, wallet.ID, service.Pagination{Page: 1, PageSize: 10})
	require.ErrorIs(t, err, service.ErrWalletNotFound)

	_, err = f.walletSvc.Get(ctx, bob.ID, wallet.ID)
	require.ErrorIs(t, err, service.ErrWalletNotFound)
}
