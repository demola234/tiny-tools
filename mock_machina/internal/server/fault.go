package server

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
)

const maxHold = 5 * time.Minute

func fires(f *model.Fault, src seed.Source, route string, call int) bool {
	if f == nil {
		return false
	}
	if f.Rate >= 1 {
		return true
	}
	return src.Stream(route, "fault", strconv.Itoa(call)).Float64() < f.Rate
}

func injectFault(w http.ResponseWriter, req *http.Request, f *model.Fault, status int, body []byte, clk clock.Clock) {
	if clk.Sleep(req.Context(), f.After) != nil {
		return
	}
	switch f.Type {
	case model.FaultTimeout:
		_ = clk.Sleep(req.Context(), maxHold)
		hangUp(w, false)
	case model.FaultReset:
		hangUp(w, true)
	case model.FaultTruncated:
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(status)
		_, _ = w.Write(body[:len(body)/2])
		_ = http.NewResponseController(w).Flush()
		hangUp(w, false)
	}
}

func hangUp(w http.ResponseWriter, reset bool) {
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return
	}
	if tcp, ok := conn.(*net.TCPConn); ok && reset {
		_ = tcp.SetLinger(0)
	}
	_ = conn.Close()
}
