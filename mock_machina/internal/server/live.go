package server

import (
	"net/http"
	"sync/atomic"
)

type Live struct {
	current atomic.Pointer[http.Handler]
}

func NewLive(h http.Handler) *Live {
	l := &Live{}
	l.Swap(h)
	return l
}

func (l *Live) Swap(h http.Handler) { l.current.Store(&h) }

func (l *Live) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*l.current.Load()).ServeHTTP(w, r)
}
