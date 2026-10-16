package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestManager(now time.Time) *TokenManager {
	m := NewTokenManager(testSecret, "wallet-service", "wallet-api", 15*time.Minute)
	m.now = func() time.Time { return now }
	return m
}

func TestTokenManager_IssueAndParse(t *testing.T) {
	now := time.Now()
	m := newTestManager(now)
	userID := uuid.New()

	token, expiresAt, err := m.Issue(userID)
	require.NoError(t, err)
	assert.WithinDuration(t, now.Add(15*time.Minute), expiresAt, time.Second)

	parsed, err := m.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, userID, parsed)
}

func TestTokenManager_ParseRejectsInvalidTokens(t *testing.T) {
	now := time.Now()
	m := newTestManager(now)
	userID := uuid.New()

	valid, _, err := m.Issue(userID)
	require.NoError(t, err)

	expired, _, err := newTestManager(now.Add(-time.Hour)).Issue(userID)
	require.NoError(t, err)

	otherSecret, _, err := NewTokenManager("ffffffffffffffffffffffffffffffff", "wallet-service", "wallet-api", time.Minute).Issue(userID)
	require.NoError(t, err)

	otherAudience, _, err := NewTokenManager(testSecret, "wallet-service", "admin-api", time.Minute).Issue(userID)
	require.NoError(t, err)

	otherIssuer, _, err := NewTokenManager(testSecret, "someone-else", "wallet-api", time.Minute).Issue(userID)
	require.NoError(t, err)

	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    "wallet-service",
		Audience:  jwt.ClaimStrings{"wallet-api"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	invalidSubject, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "not-a-uuid",
		Issuer:    "wallet-service",
		Audience:  jwt.ClaimStrings{"wallet-api"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString([]byte(testSecret))
	require.NoError(t, err)

	tests := map[string]string{
		"expired":         expired,
		"wrong secret":    otherSecret,
		"wrong audience":  otherAudience,
		"wrong issuer":    otherIssuer,
		"alg none":        unsigned,
		"invalid subject": invalidSubject,
		"tampered":        valid[:len(valid)-2] + "xx",
		"garbage":         "not.a.jwt",
	}

	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := m.Parse(token)
			require.ErrorIs(t, err, ErrInvalidToken)
		})
	}
}

func TestRefreshTokenGenerationAndHashing(t *testing.T) {
	first, err := GenerateRefreshToken()
	require.NoError(t, err)
	second, err := GenerateRefreshToken()
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.Len(t, HashToken(first), 64)
	assert.Equal(t, HashToken(first), HashToken(first))
	assert.NotEqual(t, HashToken(first), HashToken(second))
}

func TestPasswordHasher(t *testing.T) {
	h := NewPasswordHasher(4)

	hash, err := h.Hash("s3cret-password")
	require.NoError(t, err)

	ok, err := h.Verify(hash, "s3cret-password")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = h.Verify(hash, "wrong-password")
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = h.Verify("not-a-bcrypt-hash", "s3cret-password")
	require.Error(t, err)
}
