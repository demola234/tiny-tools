package control

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/demola234/tiny-tools/mock_machina/internal/clock"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
	"github.com/demola234/tiny-tools/mock_machina/internal/suggest"
)

const prefix = model.ControlPrefix

var endpoints = []string{
	"GET " + prefix + "routes",
	"GET " + prefix + "routes/{id}",
	"PUT " + prefix + "routes/{id}/state",
	"DELETE " + prefix + "routes/{id}/state",
	"POST " + prefix + "reset",
	"GET " + prefix + "events",
	"GET " + prefix + "requests",
	"GET " + prefix + "coverage",
	"GET " + prefix + "collections",
	"GET " + prefix + "collections/{name}",
	"DELETE " + prefix + "collections/{name}/{id}",
	"GET " + prefix + "scenarios",
	"POST " + prefix + "scenarios/{name}/apply",
}

type Options struct {
	Name   string
	Dir    string
	Memory *server.Memory
	Token  string
	Clock  clock.Clock
}

type API struct {
	opts    Options
	project atomic.Pointer[model.Project]
	events  *hub
	mux     *http.ServeMux
	grpc    string
}

func New(p *model.Project, opts Options) *API {
	if opts.Clock == nil {
		opts.Clock = clock.Real{}
	}
	a := &API{opts: opts, events: newHub(opts.Clock), mux: http.NewServeMux()}
	a.project.Store(p)
	handlers := []http.HandlerFunc{a.routes, a.route, a.pin, a.unpin, a.reset, a.stream, a.requests, a.coverage, a.collections, a.collection, a.deleteItem, a.scenarios, a.apply}
	for i, pattern := range endpoints {
		a.mux.HandleFunc(pattern, handlers[i])
	}
	a.mountUI()
	a.mux.HandleFunc(prefix, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, problem{Error: "not_found", Endpoints: endpoints})
	})
	return a
}

func (a *API) Swap(p *model.Project) {
	a.project.Store(p)
	a.events.reload()
}

func (a *API) SetGRPC(addr string) { a.grpc = addr }

func (a *API) Publish(r server.Request) { a.events.publish(r) }

func (a *API) Close() { a.events.close() }

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if code, msg := a.refuse(r); code != 0 {
		if strings.HasPrefix(r.URL.Path, uiPrefix) {
			lockedPage(w, code, msg, "")
			return
		}
		if code == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", "Bearer")
		}
		writeJSON(w, code, problem{Error: "forbidden", Detail: msg})
		return
	}
	if strings.HasPrefix(r.URL.Path, uiPrefix) && !guardUI(w, r) {
		return
	}
	a.mux.ServeHTTP(w, r)
}

func (a *API) refuse(r *http.Request) (int, string) {
	if origin := r.Header.Get("Origin"); origin != "" {
		if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
			return http.StatusForbidden, "the control API doesn't answer other sites"
		}
	}
	if loopback(r.RemoteAddr) && loopback(r.Host) || public(r) {
		return 0, ""
	}
	if a.opts.Token == "" {
		return http.StatusForbidden, "the control API only answers this machine; set MOCKMACHINA_CONTROL_TOKEN to allow others"
	}
	if strings.HasPrefix(r.URL.Path, uiPrefix) && a.sessionCookie(r) {
		return 0, ""
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.opts.Token)) != 1 {
		return http.StatusUnauthorized, `send "Authorization: Bearer <token>"; start prints the token`
	}
	return 0, ""
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.Trim(addr, "[]")
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type problem struct {
	Error      string   `json:"error"`
	Route      string   `json:"route,omitempty"`
	Collection string   `json:"collection,omitempty"`
	Scenario   string   `json:"scenario,omitempty"`
	ID         string   `json:"id,omitempty"`
	State      string   `json:"state,omitempty"`
	Valid      []string `json:"valid,omitempty"`
	Hint       string   `json:"hint,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	Endpoints  []string `json:"endpoints,omitempty"`
}

func (a *API) routes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.routeViews())
}

func (a *API) route(w http.ResponseWriter, req *http.Request) {
	r, f := a.lookupRoute(req.PathValue("id"))
	if f != nil {
		f.writeJSON(w)
		return
	}
	writeJSON(w, http.StatusOK, a.view(r))
}

type pinRequest struct {
	State   string `json:"state"`
	Persist bool   `json:"persist"`
}

func (a *API) pin(w http.ResponseWriter, req *http.Request) {
	if _, f := a.lookupRoute(req.PathValue("id")); f != nil {
		f.writeJSON(w)
		return
	}
	var body pinRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.State == "" {
		writeJSON(w, http.StatusBadRequest, problem{Error: "bad_request", Detail: `send {"state": "<name>"}`})
		return
	}
	v, f := a.setState(req.PathValue("id"), body.State, body.Persist)
	if f != nil {
		f.writeJSON(w)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) unpin(w http.ResponseWriter, req *http.Request) {
	v, f := a.unpinRoute(req.PathValue("id"))
	if f != nil {
		f.writeJSON(w)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) reset(w http.ResponseWriter, _ *http.Request) {
	a.resetAll()
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) requests(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.requestsView())
}

func (a *API) stream(w http.ResponseWriter, req *http.Request) {
	a.serveEvents(w, req, func(m message) (string, bool) {
		data, _ := json.Marshal(m.data)
		return string(data), true
	})
}

func (a *API) serveEvents(w http.ResponseWriter, req *http.Request, format func(message) (string, bool)) {
	events, stop := a.events.subscribe()
	defer stop()
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_ = rc.Flush()
	for {
		select {
		case <-req.Context().Done():
			return
		case m, open := <-events:
			if !open {
				return
			}
			data, ok := format(m)
			if !ok {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", m.name, data); err != nil {
				return
			}
			_ = rc.Flush()
		}
	}
}

func (a *API) coverage(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.coverageView())
}

func (a *API) collections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]collectionView{"collections": a.collectionViews()})
}

func (a *API) collection(w http.ResponseWriter, req *http.Request) {
	v, f := a.collectionDetail(req.PathValue("name"))
	if f != nil {
		f.writeJSON(w)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) deleteItem(w http.ResponseWriter, req *http.Request) {
	if f := a.removeItem(req.PathValue("name"), req.PathValue("id")); f != nil {
		f.writeJSON(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) scenarios(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.scenariosView())
}

func (a *API) apply(w http.ResponseWriter, req *http.Request) {
	v, f := a.applyScenario(req.PathValue("name"))
	if f != nil {
		f.writeJSON(w)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func hint(name string, names []string) string {
	if s, ok := suggest.Closest(name, names); ok {
		return `did you mean "` + s + `"?`
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
