package testkit_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// fakeTB records failures instead of failing the real test, so helpers that
// report failures can themselves be tested. Methods it doesn't override
// belong to the nil embedded TB and panic if a helper starts using them.
type fakeTB struct {
	testing.TB

	failed bool
	fatal  bool
	msgs   []string
}

func (*fakeTB) Helper() {}

func (f *fakeTB) Errorf(format string, args ...any) {
	f.failed = true
	f.msgs = append(f.msgs, fmt.Sprintf(format, args...))
}

// Fatalf stops the helper the way testing.T does: by ending its goroutine.
func (f *fakeTB) Fatalf(format string, args ...any) {
	f.Errorf(format, args...)
	f.fatal = true
	runtime.Goexit()
}

func (f *fakeTB) output() string { return strings.Join(f.msgs, "\n") }

// runFake calls fn with a fakeTB on its own goroutine, so Fatalf can end it.
func runFake(fn func(tb testing.TB)) *fakeTB {
	f := &fakeTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(f)
	}()
	<-done
	return f
}
