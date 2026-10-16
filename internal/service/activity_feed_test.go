package service_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
)

func (f *paymentFixture) seedTopUps(t *testing.T, wallet *model.Wallet, at func(i int) time.Time, count int) {
	t.Helper()

	ctx := context.Background()
	for i := range count {
		result, err := f.paymentSvc.TopUp(ctx, service.TopUpInput{
			UserID:         wallet.UserID,
			WalletID:       wallet.ID,
			Amount:         int64(i + 1),
			IdempotencyKey: fmt.Sprintf("feed-%s-%d", wallet.ID, i),
		})
		require.NoError(t, err)
		require.NoError(t, f.db.Model(&model.LedgerEntry{}).
			Where("transaction_id = ?", result.Transaction.ID).
			Update("created_at", at(i)).Error)
	}
}

func (f *paymentFixture) walkFeed(t *testing.T, wallet *model.Wallet, filter model.ActivityFilter, limit int) ([][]int64, []model.WalletActivity) {
	t.Helper()

	var (
		pages  [][]int64
		items  []model.WalletActivity
		cursor string
	)
	for range 50 {
		feed, err := f.walletSvc.ListActivityFeed(context.Background(), wallet.UserID, wallet.ID, filter, service.CursorPage{Cursor: cursor, Limit: limit})
		require.NoError(t, err)

		page := make([]int64, 0, len(feed.Items))
		for _, item := range feed.Items {
			page = append(page, item.Amount)
		}
		pages = append(pages, page)
		items = append(items, feed.Items...)

		if feed.NextCursor == "" {
			return pages, items
		}
		cursor = feed.NextCursor
	}

	t.Fatal("feed did not terminate")
	return nil, nil
}

func TestWalletService_ActivityFeedWalksEveryEntryOnce(t *testing.T) {
	f := newPaymentFixture(t)

	alice := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 0)

	base := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	f.seedTopUps(t, wallet, func(i int) time.Time { return base.Add(time.Duration(i) * time.Minute) }, 5)

	pages, _ := f.walkFeed(t, wallet, model.ActivityFilter{}, 2)

	assert.Equal(t, [][]int64{{5, 4}, {3, 2}, {1}}, pages)
}

func TestWalletService_ActivityFeedLastFullPageHasNoCursor(t *testing.T) {
	f := newPaymentFixture(t)

	alice := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 0)

	base := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	f.seedTopUps(t, wallet, func(i int) time.Time { return base.Add(time.Duration(i) * time.Minute) }, 4)

	pages, _ := f.walkFeed(t, wallet, model.ActivityFilter{}, 2)

	assert.Equal(t, [][]int64{{4, 3}, {2, 1}}, pages)
}

func TestWalletService_ActivityFeedBreaksTimestampTiesByEntryID(t *testing.T) {
	f := newPaymentFixture(t)

	alice := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 0)

	sameInstant := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	f.seedTopUps(t, wallet, func(int) time.Time { return sameInstant }, 5)

	_, items := f.walkFeed(t, wallet, model.ActivityFilter{}, 2)

	require.Len(t, items, 5)
	seen := make(map[uuid.UUID]struct{}, len(items))
	for i, item := range items {
		seen[item.EntryID] = struct{}{}
		if i > 0 {
			assert.Greater(t, items[i-1].EntryID.String(), item.EntryID.String(), "entries must be ordered by id descending")
		}
	}
	assert.Len(t, seen, 5)
}

func TestWalletService_ActivityFeedAppliesFilters(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 0)
	peer := f.createWallet(t, bob.ID, "IDR", 0)

	base := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	f.seedTopUps(t, wallet, func(i int) time.Time { return base.Add(time.Duration(i) * time.Minute) }, 4)

	result, err := f.paymentSvc.Transfer(ctx, service.TransferInput{
		UserID: alice.ID, SourceWalletID: wallet.ID, DestinationWalletID: peer.ID, Amount: 7, IdempotencyKey: "feed-transfer",
	})
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&model.LedgerEntry{}).
		Where("transaction_id = ?", result.Transaction.ID).
		Update("created_at", base.Add(time.Hour)).Error)

	tests := []struct {
		name   string
		filter model.ActivityFilter
		want   [][]int64
	}{
		{name: "top-ups only", filter: model.ActivityFilter{Type: model.TransactionTypeTopUp}, want: [][]int64{{4, 3}, {2, 1}}},
		{name: "debits only", filter: model.ActivityFilter{Direction: model.EntryDirectionDebit}, want: [][]int64{{7}}},
		{name: "time window", filter: model.ActivityFilter{From: base.Add(time.Minute), To: base.Add(3 * time.Minute)}, want: [][]int64{{3, 2}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages, _ := f.walkFeed(t, wallet, tt.filter, 2)
			assert.Equal(t, tt.want, pages)
		})
	}
}

func TestWalletService_ActivityFeedRejectsInvalidRequests(t *testing.T) {
	f := newPaymentFixture(t)
	ctx := context.Background()

	alice := f.createUser(t)
	bob := f.createUser(t)
	wallet := f.createWallet(t, alice.ID, "IDR", 1_000)
	now := time.Now().UTC()

	encode := func(raw string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(raw))
	}

	tests := []struct {
		name    string
		userID  uuid.UUID
		filter  model.ActivityFilter
		cursor  string
		wantErr error
	}{
		{name: "cursor is not base64", userID: alice.ID, cursor: "not base64!", wantErr: service.ErrInvalidCursor},
		{name: "cursor without separator", userID: alice.ID, cursor: encode("2026-03-01T09:00:00Z"), wantErr: service.ErrInvalidCursor},
		{name: "cursor with bad timestamp", userID: alice.ID, cursor: encode("yesterday|" + uuid.NewString()), wantErr: service.ErrInvalidCursor},
		{name: "cursor with bad entry id", userID: alice.ID, cursor: encode("2026-03-01T09:00:00Z|42"), wantErr: service.ErrInvalidCursor},
		{name: "empty time range", userID: alice.ID, filter: model.ActivityFilter{From: now, To: now}, wantErr: service.ErrInvalidTimeRange},
		{name: "wallet owned by another user", userID: bob.ID, wantErr: service.ErrWalletNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := f.walletSvc.ListActivityFeed(ctx, tt.userID, wallet.ID, tt.filter, service.CursorPage{Cursor: tt.cursor, Limit: 10})
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
