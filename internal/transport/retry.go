package transport

import (
	"context"
	"errors"
	"fmt"
)

func RetryExchange(ctx context.Context, count int, fn func() error) error {
	if count < 1 {
		count = 1
	}
	var lastErr error
	for attempt := 1; attempt <= count; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		return nil
	}
	return fmt.Errorf("exchange failed after %d attempt(s): %w", count, lastErr)
}

func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}
