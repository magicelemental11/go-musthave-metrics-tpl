// Package retry re-runs operations that fail with transient errors.
package retry

import (
	"context"
	"fmt"
	"time"
)

// DefaultDelays are the pauses between attempts: 1s, 3s and 5s.
var DefaultDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

// Do calls fn and, while isRetriable reports its error as transient, calls it again after each
// of delays. It returns the last error, or early if ctx is cancelled while waiting.
func Do(ctx context.Context, delays []time.Duration, fn func() error, isRetriable func(error) bool) error {
	err := fn()
	for _, delay := range delays {
		if err == nil || !isRetriable(err) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%w (last error: %v)", ctx.Err(), err)
		case <-timer.C:
		}
		err = fn()
	}
	return err
}
