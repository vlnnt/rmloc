package ollama

import (
	"context"
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

	return errors.As(
		err,
		&target,
	)
}

func (c *Client) retryOperation(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var lastErr error
	for attempt := 0; attempt < maxRequestAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		if attempt > 0 {
			if err := waitForRetry(
				ctx,
				requestRetryDelays[attempt],
			); err != nil {
				return err
			}
		}

		err := operation(ctx)
		if err == nil {
			return nil
		}

		if errors.Is(
			err,
			context.Canceled,
		) ||
			errors.Is(
				err,
				context.DeadlineExceeded,
			) {
			return err
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		lastErr = err

		if !isRetryableError(err) {
			return err
		}
	}

	if lastErr == nil {
		return fmt.Errorf(
			"operation failed without an error",
		)
	}

	return fmt.Errorf(
		"operation failed after %d attempts: %w",
		maxRequestAttempts,
		lastErr,
	)
}

func waitForRetry(
	ctx context.Context,
	delay time.Duration,
) error {
	if delay <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
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
