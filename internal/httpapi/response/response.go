package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/alice101-dev/ai-code-reviewer/internal/apperror"
)

const RequestIDHeader = "X-Request-ID"

type envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      apperror.Code    `json:"code"`
	Message   string           `json:"message"`
	RequestID string           `json:"request_id,omitempty"`
	Details   []FieldViolation `json:"details,omitempty"`
}

type FieldViolation struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Data: data})
}

func OKWithMeta(c *gin.Context, data, meta any) {
	c.JSON(http.StatusOK, envelope{Data: data, Meta: meta})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Data: data})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Error(c *gin.Context, err error) {
	appErr, ok := apperror.As(err)
	if !ok || appErr.Code == apperror.CodeInternal {
		_ = c.Error(err)
		abort(c, http.StatusInternalServerError, ErrorBody{
			Code:    apperror.CodeInternal,
			Message: "internal server error",
		})
		return
	}

	if appErr.Err != nil {
		_ = c.Error(appErr.Err)
	}

	abort(c, StatusFor(appErr.Code), ErrorBody{
		Code:    appErr.Code,
		Message: appErr.Message,
	})
}

func BindError(c *gin.Context, err error) {
	bindError(c, err, "request body is malformed")
}

func QueryError(c *gin.Context, err error) {
	bindError(c, err, "query parameters are malformed")
}

func bindError(c *gin.Context, err error, malformedMessage string) {
	var (
		maxBytesErr    *http.MaxBytesError
		validationErrs validator.ValidationErrors
	)

	switch {
	case errors.As(err, &maxBytesErr):
		abort(c, http.StatusRequestEntityTooLarge, ErrorBody{
			Code:    apperror.CodePayloadTooLarge,
			Message: "request body is too large",
		})
	case errors.As(err, &validationErrs):
		details := make([]FieldViolation, 0, len(validationErrs))
		for _, fe := range validationErrs {
			details = append(details, FieldViolation{Field: fe.Field(), Rule: fe.Tag()})
		}
		abort(c, http.StatusBadRequest, ErrorBody{
			Code:    apperror.CodeInvalidArgument,
			Message: "request validation failed",
			Details: details,
		})
	default:
		abort(c, http.StatusBadRequest, ErrorBody{
			Code:    apperror.CodeInvalidArgument,
			Message: malformedMessage,
		})
	}
}

func StatusFor(code apperror.Code) int {
	switch code {
	case apperror.CodeInvalidArgument:
		return http.StatusBadRequest
	case apperror.CodeUnauthenticated:
		return http.StatusUnauthorized
	case apperror.CodePermissionDenied:
		return http.StatusForbidden
	case apperror.CodeNotFound:
		return http.StatusNotFound
	case apperror.CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case apperror.CodeConflict:
		return http.StatusConflict
	case apperror.CodeUnprocessable:
		return http.StatusUnprocessableEntity
	case apperror.CodeRateLimited:
		return http.StatusTooManyRequests
	case apperror.CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperror.CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func abort(c *gin.Context, status int, body ErrorBody) {
	body.RequestID = c.Writer.Header().Get(RequestIDHeader)
	c.AbortWithStatusJSON(status, errorEnvelope{Error: body})
}
