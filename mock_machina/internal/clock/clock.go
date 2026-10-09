package clock

import (
	"context"
	"time"
)

type Clock interface {
	Sleep(ctx context.Context, d time.Duration) error
	Now() time.Time
}

type Real struct{}

func (Real) Now() time.Time { return time.Now() }

func (Real) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
