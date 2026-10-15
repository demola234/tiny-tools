package control

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

const (
	uiPrefix = prefix + "ui/"
	pageCSP  = "default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
)

//go:embed ui
var uiFiles embed.FS

var pages = parsePages("overview", "routes", "traffic", "data", "scenarios", "coverage")

func parsePages(names ...string) map[string]*template.Template {
	layout := template.Must(template.ParseFS(uiFiles, "ui/layout.html", "ui/rows.html"))
	out := make(map[string]*template.Template, len(names))
	for _, name := range names {
		out[name] = template.Must(template.Must(layout.Clone()).ParseFS(uiFiles, "ui/"+name+".html"))
	}
	return out
}

func staticFiles() http.Handler {
	sub, err := fs.Sub(uiFiles, "ui/static")
	if err != nil {
		panic(err)
	}
	files := http.StripPrefix(uiPrefix+"static/", http.FileServerFS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/") {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, req)
	})
}

func (a *API) mountUI() {
	a.mux.Handle("GET "+uiPrefix+"static/", staticFiles())
	a.mux.HandleFunc("GET "+uiPrefix+"{$}", a.overviewPage)
	a.mux.HandleFunc("GET "+uiPrefix+"traffic", a.trafficView)
	a.mux.HandleFunc("GET "+uiPrefix+"data", a.dataView)
	a.mux.HandleFunc("GET "+uiPrefix+"data/{name}", a.dataView)
	a.mux.HandleFunc("POST "+uiPrefix+"data/{name}/{id}/delete", a.deleteFromUI)
	a.mux.HandleFunc("POST "+uiPrefix+"reset", a.resetFromUI)
	a.mux.HandleFunc("GET "+uiPrefix+"scenarios", a.scenariosPage)
	a.mux.HandleFunc("GET "+uiPrefix+"coverage", a.coveragePage)
	a.mux.HandleFunc("GET "+uiPrefix+"palette", a.palette)
	a.mux.HandleFunc("POST "+uiPrefix+"login", a.login)
	a.mux.HandleFunc("POST "+uiPrefix+"scenarios/{name}/apply", a.applyFromUI)
	a.mux.HandleFunc("GET "+uiPrefix+"events", a.uiEvents)
	a.mux.HandleFunc("GET "+uiPrefix+"routes", a.routesView)
	a.mux.HandleFunc("GET "+uiPrefix+"routes/{id}", a.routeView)
	a.mux.HandleFunc("POST "+uiPrefix+"routes/{id}/pin", a.pinFromUI)
	a.mux.HandleFunc("POST "+uiPrefix+"routes/{id}/unpin", a.unpinFromUI)
	a.mux.HandleFunc("POST "+uiPrefix+"routes/{id}/default", a.defaultFromUI)
}

func guardUI(w http.ResponseWriter, req *http.Request) bool {
	h := w.Header()
	h.Set("Content-Security-Policy", pageCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Header.Get("HX-Request") == "true" || public(req) {
		return true
	}
	writeJSON(w, http.StatusForbidden, problem{Error: "forbidden", Detail: "dashboard actions only come from the dashboard page"})
	return false
}

type navItem struct {
	Label   string
	Href    string
	Count   string
	Current bool
}

type groupItem struct {
	Name  string
	Count int
}

type page struct {
	Base   string
	Self   string
	Events string
	Name   string
	Title  string
	Host   string
	GRPC   string
	Proxy  string
	Nav    []navItem
	Groups []groupItem
}

func (a *API) page(req *http.Request, title string) page {
	p := a.project.Load()
	name := a.opts.Name
	if name == "" {
		name = "MockMachina"
	}
	total, _ := a.events.counts()
	counts := []string{
		"", strconv.Itoa(len(p.Routes)), strconv.Itoa(total), strconv.Itoa(len(a.collectionViews())),
		strconv.Itoa(len(p.Scenarios)), strconv.Itoa(a.coverageView().Percent) + "%",
	}
	pg := page{Base: uiPrefix, Self: currentPage(req), Events: uiPrefix + "events", Name: name, Title: title, Host: req.Host, GRPC: a.grpc, Proxy: p.Config.Proxy}
	for i, v := range views {
		pg.Nav = append(pg.Nav, navItem{Label: v.label, Href: uiPrefix + v.path, Count: counts[i], Current: v.label == title})
	}
	for _, r := range p.Routes {
		if n := len(pg.Groups); n > 0 && pg.Groups[n-1].Name == r.Group {
			pg.Groups[n-1].Count++
			continue
		}
		pg.Groups = append(pg.Groups, groupItem{Name: r.Group, Count: 1})
	}
	return pg
}

func currentPage(req *http.Request) string {
	u := req.URL
	if req.Method != http.MethodGet {
		parsed, err := url.Parse(req.Header.Get("HX-Current-URL"))
		if err != nil || !strings.HasPrefix(parsed.Path, uiPrefix) {
			return path.Dir(req.URL.Path)
		}
		u = parsed
	}
	if u.RawQuery == "" {
		return u.Path
	}
	return u.Path + "?" + u.RawQuery
}

func render(w http.ResponseWriter, name string, status int, data any) {
	var b bytes.Buffer
	if err := pages[name].ExecuteTemplate(&b, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}

const overviewRows = 8

type pinView struct {
	ID     string
	Method string
	Path   string
	State  string
}

type overviewPage struct {
	page
	Routes   int
	Requests int
	Errors   int
	Coverage coverageView
	Scenario string
	Pins     []pinView
	Rows     []trafficRow
}

func (a *API) overviewPage(w http.ResponseWriter, req *http.Request) {
	p := a.project.Load()
	op := overviewPage{page: a.page(req, "Overview"), Routes: len(p.Routes), Coverage: a.coverageView(), Scenario: a.opts.Memory.Applied()}
	op.Requests, op.Errors = a.events.counts()
	for _, r := range p.Routes {
		if state, ok := a.opts.Memory.Pinned(r.ID); ok {
			op.Pins = append(op.Pins, pinView{ID: r.ID, Method: string(r.Method), Path: r.Path, State: state})
		}
	}
	recent := a.events.recent()
	for _, e := range recent[:min(len(recent), overviewRows)] {
		op.Rows = append(op.Rows, rowFor(e))
	}
	render(w, "overview", http.StatusOK, op)
}
