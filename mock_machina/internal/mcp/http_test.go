package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/demola234/tiny-tools/mock_machina/internal/mcp"
)

type withHeader struct {
	header, value string
}

func (h withHeader) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set(h.header, h.value)
	return http.DefaultTransport.RoundTrip(r)
}

func serveHTTP(t *testing.T, token string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mcp.Handler(mcp.Options{Dir: shop(t), Version: "v0.0.0-test"}, token))
	t.Cleanup(srv.Close)
	return srv
}

func connectHTTP(t *testing.T, endpoint string, rt http.RoundTripper) (*sdk.ClientSession, error) {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Transport: rt},
		MaxRetries: -1,
	}, nil)
	if err == nil {
		t.Cleanup(func() { _ = cs.Close() })
	}
	return cs, err
}

func listsRoutes(t *testing.T, cs *sdk.ClientSession) {
	t.Helper()
	if res := call(t, cs, "list_routes", nil); res.IsError {
		t.Errorf("list_routes: %s", text(res))
	}
}

func TestHandler_WithoutToken(t *testing.T) {
	t.Parallel()

	srv := serveHTTP(t, "")
	cs, err := connectHTTP(t, srv.URL+"/mcp", http.DefaultTransport)
	if err != nil {
		t.Fatal(err)
	}
	listsRoutes(t, cs)
}

func TestHandler_Token(t *testing.T) {
	t.Parallel()

	srv := serveHTTP(t, "s3cret")
	cs, err := connectHTTP(t, srv.URL+"/mcp", withHeader{"Authorization", "Bearer s3cret"})
	if err != nil {
		t.Fatalf("with the bearer token: %v", err)
	}
	listsRoutes(t, cs)

	cs, err = connectHTTP(t, srv.URL+"/mcp/s3cret", http.DefaultTransport)
	if err != nil {
		t.Fatalf("with the token in the path: %v", err)
	}
	listsRoutes(t, cs)

	if _, err := connectHTTP(t, srv.URL+"/mcp", http.DefaultTransport); err == nil {
		t.Error("connected without the token")
	}
}

func status(t *testing.T, srv *httptest.Server, path string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if h, ok := headers["Host"]; ok {
		req.Host = h
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestHandler_Refusals(t *testing.T) {
	t.Parallel()

	open, locked := serveHTTP(t, ""), serveHTTP(t, "s3cret")
	tests := []struct {
		name    string
		srv     *httptest.Server
		path    string
		headers map[string]string
		want    int
	}{
		{"no token", locked, "/mcp", nil, http.StatusUnauthorized},
		{"wrong token", locked, "/mcp", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"wrong path token", locked, "/mcp/nope", nil, http.StatusNotFound},
		{"other path", open, "/", nil, http.StatusNotFound},
		{"path token without a token set", open, "/mcp/x", nil, http.StatusNotFound},
		{"website in a browser", open, "/mcp", map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"DNS rebinding without a token", open, "/mcp", map[string]string{"Host": "evil.example"}, http.StatusForbidden},
		{"tunnel with the token", locked, "/mcp", map[string]string{"Host": "abc.tunnel.example", "Authorization": "Bearer s3cret"}, http.StatusOK},
	}
	for _, tc := range tests {
		if got := status(t, tc.srv, tc.path, tc.headers); got != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, got, tc.want)
		}
	}
}
