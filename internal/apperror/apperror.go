package apperror

import "errors"

type Code string

const (
	CodeInvalidArgument    Code = "invalid_argument"
	CodeUnauthenticated    Code = "unauthenticated"
	CodePermissionDenied   Code = "permission_denied"
	CodeNotFound           Code = "not_found"
	CodeMethodNotAllowed   Code = "method_not_allowed"
	CodeConflict           Code = "conflict"
	CodeUnprocessable      Code = "unprocessable_entity"
	CodeRateLimited        Code = "rate_limited"
	CodePayloadTooLarge    Code = "payload_too_large"
	CodeInternal           Code = "internal"
	CodeServiceUnavailable Code = "service_unavailable"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
