package ollama

import (
	"errors"
	"fmt"
	"time"
)

const maxRequestAttempts = 3

var requestRetryDelays = []time.Duration{
	0,
	2 * time.Second,
	5 * time.Second,
}

type retryableError struct {
	err error
}

func (e *retryableError) Error() string {
	return e.err.Error()
}

func (e *retryableError) Unwrap() error {
	return e.err
}

func markRetryable(
	err error,
) error {
	if err == nil {
		return nil
	}

	return &retryableError{
		err: err,
	}
}

func isRetryableError(
	err error,
) bool {
	var target *retryableError
	return errors.As(err, &target)
}

func (c *Client) retryOperation(
	operation func() error,
) error {
	var lastErr error
	for attempt := 0; attempt < maxRequestAttempts; attempt++ {
		if attempt > 0 {
			c.recordRetryAttempt()
			time.Sleep(requestRetryDelays[attempt])
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		if !isRetryableError(err) {
			return err
		}
	}

	return fmt.Errorf(
		"operation failed after %d attempts: %w",
		maxRequestAttempts,
		lastErr,
	)
}

func isRetryableStatus(
	statusCode int,
) bool {
	switch statusCode {
	case 408,
		425,
		429,
		500,
		502,
		503,
		504:
		return true

	default:
		return false
	}
}
