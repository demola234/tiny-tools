package server_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func faultProject(f model.Fault) *model.Project {
	return &model.Project{Routes: []*model.Route{{
		ID: "pay.create", Method: model.MethodPost, Path: "/pay", Active: "flaky",
		States: model.States{{Name: "flaky", State: &model.State{Status: 201, Fault: &f, Body: jsonBody(`{"receipt":"r_1234567890"}`)}}},
	}}}
}

func faultServer(t *testing.T, f model.Fault) (*httptest.Server, <-chan server.Request) {
	t.Helper()
	got := make(chan server.Request, 1)
	h, err := server.New(faultProject(f), server.Options{Report: func(r server.Request) { got <- r }})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, got
}

func postTo(t *testing.T, client *http.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return client.Do(req)
}

func TestFault_Reset(t *testing.T) {
	t.Parallel()

	srv, got := faultServer(t, model.Fault{Type: model.FaultReset, Rate: 1})
	res, err := postTo(t, srv.Client(), srv.URL+"/pay")
	if err == nil {
		_ = res.Body.Close()
		t.Fatal("the request succeeded; want a reset connection")
	}
	if runtime.GOOS != "windows" && !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, io.EOF) {
		t.Errorf("error = %v; want a connection reset", err)
	}
	if r := <-got; r.Fault != model.FaultReset {
		t.Errorf("reported %+v", r)
	}
}

func TestFault_Truncated(t *testing.T) {
	t.Parallel()

	srv, got := faultServer(t, model.Fault{Type: model.FaultTruncated, Rate: 1})
	res, err := postTo(t, srv.Client(), srv.URL+"/pay")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if !errors.Is(err, io.ErrUnexpectedEOF) || string(body) != `{"receipt":"r` || res.StatusCode != http.StatusCreated {
		t.Errorf("status %d, body %q, error %v; want half the body then an unexpected EOF", res.StatusCode, body, err)
	}
	if r := <-got; r.Fault != model.FaultTruncated {
		t.Errorf("reported %+v", r)
	}
}

func TestFault_Timeout(t *testing.T) {
	t.Parallel()

	srv, got := faultServer(t, model.Fault{Type: model.FaultTimeout, Rate: 1})
	client := srv.Client()
	client.Timeout = 100 * time.Millisecond
	res, err := postTo(t, client, srv.URL+"/pay")
	if err == nil {
		_ = res.Body.Close()
		t.Fatal("the request got an answer; want none")
	}
	var netErr interface{ Timeout() bool }
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("error = %v; want the client's timeout", err)
	}
	if r := <-got; r.Fault != model.FaultTimeout {
		t.Errorf("reported %+v", r)
	}
}

func TestFault_After(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		h, err := server.New(faultProject(model.Fault{Type: model.FaultTruncated, Rate: 1, After: 2 * time.Second}), server.Options{Clock: clock.Real{}})
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		rec := do(t, h, http.MethodPost, "/pay", nil)
		if got := time.Since(start); got != 2*time.Second {
			t.Errorf("failed after %v, want 2s", got)
		}
		if rec.Body.String() != `{"receipt":"r` {
			t.Errorf("body = %q", rec.Body)
		}
	})
}

func TestFault_ExplicitStateWithoutAFaultIsServed(t *testing.T) {
	t.Parallel()

	p := faultProject(model.Fault{Type: model.FaultReset, Rate: 1})
	p.Routes[0].States = append(p.Routes[0].States, model.NamedState{Name: "ok", State: &model.State{Status: 201}})
	h, err := server.New(p, server.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, http.MethodPost, "/pay", map[string]string{"X-Mock-State": "ok"}); rec.Code != 201 {
		t.Errorf("ok state = %d", rec.Code)
	}
}
