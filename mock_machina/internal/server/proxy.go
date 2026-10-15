package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type proxy struct {
	target   *url.URL
	reverse  *httputil.ReverseProxy
	reporter func() func(Request)
}

func newProxy(target *url.URL, opts Options) *proxy {
	p := &proxy{target: target, reporter: opts.reporter}
	p.reverse = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			pr.Out.Header.Del(stateHeader)
			pr.Out.Header.Del(scenarioHeader)
		},
		ModifyResponse: func(res *http.Response) error {
			if !opts.DisableCORS {
				for name := range res.Header {
					if strings.HasPrefix(name, "Access-Control-") {
						res.Header.Del(name)
					}
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			sw, _ := w.(*statusWriter)
			if errors.Is(err, context.Canceled) {
				sw.status, sw.problem = clientClosedRequest, ProblemClientLeft
				return
			}
			sw.problem = ProblemBackend
			http.Error(w, "couldn't reach the backend at "+target.String()+": "+err.Error(), http.StatusBadGateway)
		},
	}
	return p
}

func (p *proxy) serve(w http.ResponseWriter, req *http.Request, route string) {
	report := p.reporter()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	p.reverse.ServeHTTP(sw, req)
	report(Request{
		Method: req.Method, Path: req.URL.RequestURI(), Status: sw.status,
		Route: route, Problem: sw.problem, Proxied: true,
	})
}

func asksForState(req *http.Request) bool {
	return req.Header.Get(stateHeader) != "" || req.URL.Query().Get(stateQuery) != ""
}

type statusWriter struct {
	http.ResponseWriter
	status  int
	problem Problem
	wrote   bool
}

func (w *statusWriter) WriteHeader(status int) {
	if !w.wrote {
		w.status, w.wrote = status, true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
