package httperr

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Ithildur/EiluneKit/http/response"
)

// Error is a transport-level HTTP error response.
type Error struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return http.StatusText(e.Status)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type ErrorLogger interface {
	Error(msg string, err error, attrs ...slog.Attr)
}

func Wrap(status int, code, message string, cause error) *Error {
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

func InvalidRequest(cause error) *Error {
	return Wrap(http.StatusBadRequest, "invalid_request", "invalid request", cause)
}

func Unauthorized(cause error) *Error {
	return Wrap(http.StatusUnauthorized, "unauthorized", "unauthorized", cause)
}

func Forbidden(cause error) *Error {
	return Wrap(http.StatusForbidden, "forbidden", "forbidden", cause)
}

func NotFound(cause error) *Error {
	return Wrap(http.StatusNotFound, "not_found", "resource not found", cause)
}

func BodyTooLarge(cause error) *Error {
	return Wrap(http.StatusRequestEntityTooLarge, "body_too_large", "request body too large", cause)
}

func InvalidMetrics(cause error) *Error {
	return Wrap(http.StatusUnprocessableEntity, "invalid_metrics", "invalid metrics", cause)
}

func InvalidStaticPayload(cause error) *Error {
	return Wrap(http.StatusUnprocessableEntity, "invalid_static_payload", "invalid static payload", cause)
}

func ServiceUnavailable(cause error) *Error {
	return Wrap(http.StatusServiceUnavailable, "service_unavailable", "service unavailable", cause)
}

func Internal(cause error) *Error {
	return Wrap(http.StatusInternalServerError, "internal_error", "internal error", cause)
}

// Write writes a JSON HTTP error payload.
func Write(w http.ResponseWriter, status int, code, message string) {
	response.WriteJSONError(w, status, code, message)
}

// TryWrite serializes err if it is or wraps an HTTP transport error.
func TryWrite(w http.ResponseWriter, err error) bool {
	var httpErr *Error
	if !errors.As(err, &httpErr) {
		return false
	}
	Write(w, httpErr.Status, httpErr.Code, httpErr.Message)
	return true
}

func WriteOrInternal(w http.ResponseWriter, logger ErrorLogger, err error) {
	if TryWrite(w, err) {
		return
	}
	if logger != nil {
		logger.Error("unexpected error", err)
	}
	Write(w, http.StatusInternalServerError, "internal_error", "internal error")
}
