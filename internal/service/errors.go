package service

import "github.com/alice101-dev/ai-code-reviewer/internal/apperror"

var (
	ErrEmailAlreadyRegistered = apperror.New(apperror.CodeConflict, "email is already registered")
	ErrPasswordTooLong        = apperror.New(apperror.CodeInvalidArgument, "password must not exceed 72 bytes")
	ErrInvalidCredentials     = apperror.New(apperror.CodeUnauthenticated, "invalid email or password")
	ErrInvalidRefreshToken    = apperror.New(apperror.CodeUnauthenticated, "refresh token is invalid or expired")
	ErrAccountSuspended       = apperror.New(apperror.CodePermissionDenied, "account is suspended")
	ErrUserNotFound           = apperror.New(apperror.CodeNotFound, "user not found")

	ErrWalletNotFound            = apperror.New(apperror.CodeNotFound, "wallet not found")
	ErrDestinationWalletNotFound = apperror.New(apperror.CodeNotFound, "destination wallet not found")
	ErrWalletAlreadyExists       = apperror.New(apperror.CodeConflict, "wallet for this currency already exists")
	ErrUnsupportedCurrency       = apperror.New(apperror.CodeInvalidArgument, "currency is not supported")
	ErrWalletNotActive           = apperror.New(apperror.CodeUnprocessable, "wallet is not active")
	ErrInsufficientFunds         = apperror.New(apperror.CodeUnprocessable, "insufficient funds")
	ErrCurrencyMismatch          = apperror.New(apperror.CodeUnprocessable, "source and destination wallets must use the same currency")
	ErrSameWallet                = apperror.New(apperror.CodeInvalidArgument, "source and destination wallets must be different")
	ErrAmountOutOfRange          = apperror.New(apperror.CodeInvalidArgument, "amount is outside the allowed range")
	ErrBalanceLimitExceeded      = apperror.New(apperror.CodeUnprocessable, "wallet balance limit exceeded")
	ErrIdempotencyKeyReused      = apperror.New(apperror.CodeConflict, "idempotency key was already used for a different request")
	ErrInvalidTimeRange          = apperror.New(apperror.CodeInvalidArgument, "from must be earlier than to")
	ErrWalletHasBalance          = apperror.New(apperror.CodeUnprocessable, "wallet balance must be zero before it can be closed")
)
