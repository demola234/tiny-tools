package mcp

import (
	"crypto/subtle"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const endpoint = "/mcp"

func Handler(opts Options, token string) http.Handler {
	server := New(opts)
	h := http.NewCrossOriginProtection().Handler(
		sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{
			DisableLocalhostProtection: token != "",
		}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == endpoint && (token == "" || same(r.Header.Get("Authorization"), "Bearer "+token)):
			h.ServeHTTP(w, r)
		case r.URL.Path == endpoint:
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "this MockMachina MCP server needs its token", http.StatusUnauthorized)
		case token != "" && same(r.URL.Path, endpoint+"/"+token):
			h.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func same(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
