package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const refreshTokenBytes = 32

var ErrInvalidToken = errors.New("invalid token")

type TokenManager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

func NewTokenManager(secret, issuer, audience string, ttl time.Duration) *TokenManager {
	return &TokenManager{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
		now:      time.Now,
	}
}

func (m *TokenManager) TTL() time.Duration {
	return m.ttl
}

func (m *TokenManager) Issue(userID uuid.UUID) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(m.ttl)

	jti, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate token id: %w", err)
	}

	claims := jwt.RegisteredClaims{
		ID:        jti.String(),
		Subject:   userID.String(),
		Issuer:    m.issuer,
		Audience:  jwt.ClaimStrings{m.audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return signed, expiresAt, nil
}

func (m *TokenManager) Parse(raw string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims

	_, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) {
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid subject", ErrInvalidToken)
	}

	return userID, nil
}

func GenerateRefreshToken() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
