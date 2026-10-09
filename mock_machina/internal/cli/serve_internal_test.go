package cli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
)

func TestServe_ReturnsAcceptErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := serve(ctx, failingListener{}, http.NotFoundHandler())
	if !errors.Is(err, errAccept) {
		t.Errorf("serve() = %v, want the listener's error", err)
	}
}

var errAccept = errors.New("accept failed")

type failingListener struct{ net.Listener }

func (failingListener) Accept() (net.Conn, error) { return nil, errAccept }
func (failingListener) Close() error              { return nil }
func (failingListener) Addr() net.Addr            { return &net.TCPAddr{} }
