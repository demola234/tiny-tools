package server

import "net/http"

const preflightMaxAge = "600"

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")

		method := r.Header.Get("Access-Control-Request-Method")
		if r.Method != http.MethodOptions || method == "" {
			h.Set("Access-Control-Expose-Headers", stateHeader+", "+routeHeader)
			next.ServeHTTP(w, r)
			return
		}
		h.Add("Vary", "Access-Control-Request-Method")
		h.Add("Vary", "Access-Control-Request-Headers")
		h.Set("Access-Control-Allow-Methods", method)
		if headers := r.Header.Get("Access-Control-Request-Headers"); headers != "" {
			h.Set("Access-Control-Allow-Headers", headers)
		}
		h.Set("Access-Control-Max-Age", preflightMaxAge)
		w.WriteHeader(http.StatusNoContent)
	})
}
