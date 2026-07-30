package notify

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"time"
)

var (
	ErrInvalidConfig = errors.New("invalid notify config")
	ErrStoredConfig  = errors.New("invalid stored notify config")
)

type DeliveryClass string

const (
	DeliveryRetry   DeliveryClass = "retry"
	DeliveryBlocked DeliveryClass = "blocked"
)

type DeliveryFailure struct {
	Class      DeliveryClass
	Code       string
	RetryAfter time.Duration
}

type deliveryError struct {
	failure DeliveryFailure
	err     error
}

func (e *deliveryError) Error() string {
	return e.err.Error()
}

func (e *deliveryError) Unwrap() error {
	return e.err
}

func retryError(code string, retryAfter time.Duration, err error) error {
	return classifiedError(DeliveryRetry, code, retryAfter, err)
}

func blockedError(code string, err error) error {
	return classifiedError(DeliveryBlocked, code, 0, err)
}

func classifiedError(class DeliveryClass, code string, retryAfter time.Duration, err error) error {
	if err == nil {
		err = errors.New("notification delivery failed")
	}
	return &deliveryError{
		failure: DeliveryFailure{
			Class:      class,
			Code:       code,
			RetryAfter: max(retryAfter, 0),
		},
		err: err,
	}
}

func Classify(err error) DeliveryFailure {
	if err == nil {
		return DeliveryFailure{}
	}

	var classified *deliveryError
	if errors.As(err, &classified) {
		return classified.failure
	}
	if errors.Is(err, ErrInvalidConfig) || errors.Is(err, ErrStoredConfig) {
		return DeliveryFailure{Class: DeliveryBlocked, Code: "invalid_config"}
	}

	var smtpErr *textproto.Error
	if errors.As(err, &smtpErr) {
		class := DeliveryBlocked
		if smtpErr.Code >= 400 && smtpErr.Code < 500 {
			class = DeliveryRetry
		}
		return DeliveryFailure{
			Class: class,
			Code:  fmt.Sprintf("smtp_%d", smtpErr.Code),
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return DeliveryFailure{Class: DeliveryRetry, Code: "timeout"}
	}
	if errors.Is(err, context.Canceled) {
		return DeliveryFailure{Class: DeliveryRetry, Code: "canceled"}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return DeliveryFailure{Class: DeliveryRetry, Code: "network"}
	}
	return DeliveryFailure{Class: DeliveryRetry, Code: "send_failed"}
}

func ErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	runes := []rune(strings.Join(strings.Fields(err.Error()), " "))
	if len(runes) > 512 {
		runes = runes[:512]
	}
	return string(runes)
}
