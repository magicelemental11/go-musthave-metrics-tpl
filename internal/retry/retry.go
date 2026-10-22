package retry

import (
	"context"
	"fmt"
	"time"
)

var DefaultDelays = []time.Duration{time.Second, 3 * time.Second, 5 * time.Second}

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
