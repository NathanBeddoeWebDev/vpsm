package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testNetError struct {
	timeout   bool
	temporary bool
}

func (e testNetError) Error() string   { return "net error" }
func (e testNetError) Timeout() bool   { return e.timeout }
func (e testNetError) Temporary() bool { return e.temporary }

func TestDo_RetriesOnRetryableError(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{MaxAttempts: 3}, IsRetryable, func() error {
		attempts++
		return testNetError{timeout: true}
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_NoRetryOnNonRetryable(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{MaxAttempts: 3}, IsRetryable, func() error {
		attempts++
		return errors.New("boom")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_SucceedsAfterRetry(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{MaxAttempts: 3}, IsRetryable, func() error {
		attempts++
		if attempts == 1 {
			return testNetError{temporary: true}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempts := 0
	err := Do(ctx, Config{MaxAttempts: 3}, IsRetryable, func() error {
		attempts++
		return testNetError{timeout: true}
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts != 0 {
		t.Fatalf("expected 0 attempts, got %d", attempts)
	}
}

func TestIsRetryable_ContextDeadline(t *testing.T) {
	if !IsRetryable(context.DeadlineExceeded) {
		t.Fatal("expected context deadline to be retryable")
	}
}

func TestBackoffDelay_NoBaseDelay(t *testing.T) {
	if delay := backoffDelay(0, time.Second, 1); delay != 0 {
		t.Fatalf("expected zero delay, got %v", delay)
	}
}
