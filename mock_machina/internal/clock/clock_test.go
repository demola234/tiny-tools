package clock_test

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
)

func TestReal_SleepWaitsExactly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		if err := (clock.Real{}).Sleep(t.Context(), time.Hour); err != nil {
			t.Fatalf("Sleep() = %v, want nil", err)
		}
		if got := time.Since(start); got != time.Hour {
			t.Errorf("slept %v, want exactly 1h", got)
		}
	})
}

func TestReal_SleepEndsWhenCancelled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		start := time.Now()
		time.AfterFunc(time.Minute, cancel)
		err := (clock.Real{}).Sleep(ctx, time.Hour)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Sleep() = %v, want context.Canceled", err)
		}
		if got := time.Since(start); got != time.Minute {
			t.Errorf("returned after %v, want 1m", got)
		}
	})
}

func TestReal_SleepAlreadyCancelled(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		start := time.Now()
		if err := (clock.Real{}).Sleep(ctx, 0); !errors.Is(err, context.Canceled) {
			t.Errorf("Sleep() = %v, want context.Canceled", err)
		}
		if got := time.Since(start); got != 0 {
			t.Errorf("returned after %v, want immediately", got)
		}
	})
}

func TestReal_Now(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		if got := (clock.Real{}).Now(); !got.Equal(time.Now()) {
			t.Errorf("Now() = %v, want %v", got, time.Now())
		}
	})
}
