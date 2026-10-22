package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

var (
	errTemp     = errors.New("temporary")
	errFatal    = errors.New("fatal")
	shortDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
)

func isTemp(err error) bool { return errors.Is(err, errTemp) }

func TestDoSucceedsFirstTry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), shortDelays, func() error { calls++; return nil }, isTemp)
	if err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestDoRetriesUntilSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), shortDelays, func() error {
		calls++
		if calls < 3 {
			return errTemp
		}
		return nil
	}, isTemp)
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestDoGivesUpAfterThreeRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), shortDelays, func() error { calls++; return errTemp }, isTemp)
	if !errors.Is(err, errTemp) || calls != 4 {
		t.Fatalf("err=%v calls=%d, want 4 calls", err, calls)
	}
}

func TestDoDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	err := Do(context.Background(), shortDelays, func() error { calls++; return errFatal }, isTemp)
	if !errors.Is(err, errFatal) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestDoStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := Do(ctx, []time.Duration{time.Hour}, func() error { calls++; cancel(); return errTemp }, isTemp)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
