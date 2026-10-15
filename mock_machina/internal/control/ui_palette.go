package control

import (
	"bytes"
	"net/http"
	"strings"
)

const paletteLimit = 12

var views = []struct{ label, path, key string }{
	{"Overview", "", "o"},
	{"Routes", "routes", "r"},
	{"Traffic", "traffic", "t"},
	{"Data", "data", "d"},
	{"Scenarios", "scenarios", "s"},
	{"Coverage", "coverage", "c"},
}

type paletteItem struct {
	Href   string
	Label  string
	Method string
	Detail string
}

func (a *API) palette(w http.ResponseWriter, req *http.Request) {
	q := strings.ToLower(strings.TrimSpace(req.FormValue("q")))
	var items []paletteItem
	for _, v := range views {
		if strings.Contains(strings.ToLower(v.label), q) {
			items = append(items, paletteItem{Href: uiPrefix + v.path, Label: v.label, Detail: "view"})
		}
	}
	for _, r := range a.project.Load().Routes {
		if len(items) == paletteLimit {
			break
		}
		path := r.Path
		if r.Operation != "" {
			path = r.Operation
		}
		if strings.Contains(strings.ToLower(r.ID+" "+string(r.Method)+" "+path), q) {
			items = append(items, paletteItem{Href: uiPrefix + "routes/" + r.ID, Label: path, Method: string(r.Method), Detail: r.ID})
		}
	}
	var b bytes.Buffer
	if err := pages["overview"].ExecuteTemplate(&b, "palette", items); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = b.WriteTo(w)
}
