package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/repository"
	"github.com/alice101-dev/ai-code-reviewer/internal/security"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
	"github.com/alice101-dev/ai-code-reviewer/internal/testutil"
)

const testPassword = "correct-horse-battery"

type authFixture struct {
	db     *gorm.DB
	users  *repository.UserRepository
	tokens *security.TokenManager
	svc    *service.AuthService
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	db := testutil.NewPostgres(t)
	users := repository.NewUserRepository(db)
	tokens := security.NewTokenManager("test-secret-test-secret-test-secret", "wallet-service", "wallet-api", time.Minute)

	svc, err := service.NewAuthService(
		database.NewTxManager(db),
		users,
		repository.NewRefreshTokenRepository(db),
		security.NewPasswordHasher(bcrypt.MinCost),
		tokens,
		time.Hour,
	)
	require.NoError(t, err)

	return &authFixture{db: db, users: users, tokens: tokens, svc: svc}
}

func (f *authFixture) register(t *testing.T) *model.User {
	t.Helper()

	user, err := f.svc.Register(context.Background(), service.RegisterInput{
		Email:    "  " + uuid.NewString() + "@Example.COM ",
		Password: testPassword,
		FullName: " Jane Doe ",
	})
	require.NoError(t, err)
	return user
}

func TestAuthService_RegisterNormalizesAndRejectsDuplicates(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()

	user := f.register(t)
	assert.Equal(t, service.NormalizeEmail(user.Email), user.Email)
	assert.Equal(t, "Jane Doe", user.FullName)
	assert.NotEqual(t, testPassword, user.PasswordHash)

	_, err := f.svc.Register(ctx, service.RegisterInput{
		Email:    "  " + user.Email + "  ",
		Password: testPassword,
		FullName: "Duplicate",
	})
	require.ErrorIs(t, err, service.ErrEmailAlreadyRegistered)
}

func TestAuthService_Login(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	user := f.register(t)

	pair, err := f.svc.Login(ctx, user.Email, testPassword)
	require.NoError(t, err)

	subject, err := f.tokens.Parse(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, subject)
	assert.NotEmpty(t, pair.RefreshToken)

	_, err = f.svc.Login(ctx, user.Email, "wrong-password")
	require.ErrorIs(t, err, service.ErrInvalidCredentials)

	_, err = f.svc.Login(ctx, "missing@example.com", testPassword)
	require.ErrorIs(t, err, service.ErrInvalidCredentials)
}

func TestAuthService_RefreshRotatesTokenAndDetectsReuse(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	user := f.register(t)

	initial, err := f.svc.Login(ctx, user.Email, testPassword)
	require.NoError(t, err)

	rotated, err := f.svc.Refresh(ctx, initial.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, initial.RefreshToken, rotated.RefreshToken)

	_, err = f.svc.Refresh(ctx, initial.RefreshToken)
	require.ErrorIs(t, err, service.ErrInvalidRefreshToken)

	_, err = f.svc.Refresh(ctx, rotated.RefreshToken)
	require.ErrorIs(t, err, service.ErrInvalidRefreshToken)
}

func TestAuthService_LogoutRevokesOnlyOwnToken(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	alice := f.register(t)
	mallory := f.register(t)

	pair, err := f.svc.Login(ctx, alice.Email, testPassword)
	require.NoError(t, err)

	require.NoError(t, f.svc.Logout(ctx, mallory.ID, pair.RefreshToken))
	refreshed, err := f.svc.Refresh(ctx, pair.RefreshToken)
	require.NoError(t, err)

	require.NoError(t, f.svc.Logout(ctx, alice.ID, refreshed.RefreshToken))
	_, err = f.svc.Refresh(ctx, refreshed.RefreshToken)
	require.ErrorIs(t, err, service.ErrInvalidRefreshToken)
}

func TestAuthService_SuspendedUserCannotLogin(t *testing.T) {
	f := newAuthFixture(t)
	ctx := context.Background()
	user := f.register(t)

	require.NoError(t, f.db.Model(&model.User{}).Where("id = ?", user.ID).Update("status", model.UserStatusSuspended).Error)

	_, err := f.svc.Login(ctx, user.Email, testPassword)
	require.ErrorIs(t, err, service.ErrAccountSuspended)
}
