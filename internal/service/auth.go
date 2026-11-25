package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/repository"
	"github.com/alice101-dev/ai-code-reviewer/internal/security"
)

type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type AuthService struct {
	tx            TxRunner
	users         UserStore
	refreshTokens RefreshTokenStore
	hasher        PasswordHasher
	tokens        AccessTokenIssuer
	refreshTTL    time.Duration
	dummyHash     string
	now           func() time.Time
}

func NewAuthService(
	tx TxRunner,
	users UserStore,
	refreshTokens RefreshTokenStore,
	hasher PasswordHasher,
	tokens AccessTokenIssuer,
	refreshTTL time.Duration,
) (*AuthService, error) {
	dummyHash, err := hasher.Hash(uuid.NewString())
	if err != nil {
		return nil, fmt.Errorf("prepare dummy password hash: %w", err)
	}

	return &AuthService{
		tx:            tx,
		users:         users,
		refreshTokens: refreshTokens,
		hasher:        hasher,
		tokens:        tokens,
		refreshTTL:    refreshTTL,
		dummyHash:     dummyHash,
		now:           time.Now,
	}, nil
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	if len(in.Password) > security.MaxPasswordBytes {
		return nil, ErrPasswordTooLong
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:        NormalizeEmail(in.Email),
		PasswordHash: hash,
		FullName:     strings.TrimSpace(in.FullName),
		Status:       model.UserStatusActive,
	}

	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, ErrEmailAlreadyRegistered
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, err := s.users.FindByEmail(ctx, NormalizeEmail(email))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			_, _ = s.hasher.Verify(s.dummyHash, password)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	ok, err := s.hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive() {
		return nil, ErrAccountSuspended
	}

	return s.issueTokenPair(ctx, user.ID)
}

func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*TokenPair, error) {
	var (
		pair          *TokenPair
		reuseDetected bool
	)

	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		now := s.now().UTC()

		token, err := s.refreshTokens.FindByHashForUpdate(ctx, security.HashToken(rawToken))
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrInvalidRefreshToken
			}
			return fmt.Errorf("find refresh token: %w", err)
		}

		if token.IsRevoked() {
			if err := s.refreshTokens.RevokeAllForUser(ctx, token.UserID, now); err != nil {
				return fmt.Errorf("revoke user refresh tokens: %w", err)
			}
			reuseDetected = true
			return nil
		}

		if token.IsExpired(now) {
			return ErrInvalidRefreshToken
		}

		user, err := s.users.FindByID(ctx, token.UserID)
		if err != nil {
			return fmt.Errorf("find refresh token owner: %w", err)
		}
		if !user.IsActive() {
			return ErrAccountSuspended
		}

		if err := s.refreshTokens.Revoke(ctx, token.ID, now); err != nil {
			return fmt.Errorf("revoke refresh token: %w", err)
		}

		pair, err = s.issueTokenPair(ctx, user.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if reuseDetected {
		return nil, ErrInvalidRefreshToken
	}

	return pair, nil
}

func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID, rawToken string) error {
	if err := s.refreshTokens.RevokeByHash(ctx, userID, security.HashToken(rawToken), s.now().UTC()); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (s *AuthService) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID uuid.UUID) (*TokenPair, error) {
	accessToken, accessExpiresAt, err := s.tokens.Issue(userID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := security.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshExpiresAt := s.now().UTC().Add(s.refreshTTL)
	record := &model.RefreshToken{
		UserID:    userID,
		TokenHash: security.HashToken(refreshToken),
		ExpiresAt: refreshExpiresAt,
	}
	if err := s.refreshTokens.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
