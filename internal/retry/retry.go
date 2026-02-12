package retry

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"
)

// Predicate determines whether an error should be retried.
type Predicate func(error) bool

// Config controls retry behavior.
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultConfig returns the default retry configuration.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}
}

// Do executes fn with retries using the provided config.
func Do(ctx context.Context, config Config, shouldRetry Predicate, fn func() error) error {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 1
	}
	if shouldRetry == nil {
		shouldRetry = IsRetryable
	}

	var err error
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err = fn()
		if err == nil {
			return nil
		}
		if attempt == config.MaxAttempts || !shouldRetry(err) {
			return err
		}

		delay := backoffDelay(config.BaseDelay, config.MaxDelay, attempt)
		if delay <= 0 {
			continue
		}
		if !sleep(ctx, delay) {
			return ctx.Err()
		}
	}

	return err
}

// IsRetryable determines whether an error is likely transient.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}

	return false
}

func backoffDelay(base, max time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	if attempt < 1 {
		attempt = 1
	}

	delay := base << (attempt - 1)
	if max > 0 && delay > max {
		delay = max
	}

	jitterMax := int64(delay)
	if jitterMax <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(jitterMax + 1))
}

func sleep(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
