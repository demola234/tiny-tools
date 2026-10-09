package testkit_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

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

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.Errorf(format, args...)
	f.fatal = true
	runtime.Goexit()
}

func (f *fakeTB) output() string { return strings.Join(f.msgs, "\n") }

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
