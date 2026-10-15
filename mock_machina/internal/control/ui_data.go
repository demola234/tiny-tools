package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type dataTable struct {
	collectionView
	Endpoints []string
	Columns   []string
	Rows      []dataRow
}

type dataRow struct {
	ID     string
	Delete string
	Cells  []string
}

type dataPage struct {
	page
	Collections []collectionView
	Current     string
	Table       *dataTable
	Error       string
}

var crudEndpoints = []string{"GET %s", "GET %s/{id}", "POST %s", "PUT %s/{id}", "PATCH %s/{id}", "DELETE %s/{id}"}

func fields(item json.RawMessage) ([]string, map[string]string) {
	dec := json.NewDecoder(bytes.NewReader(item))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil
	}
	var keys []string
	values := map[string]string{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		key, _ := tok.(string)
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			break
		}
		keys = append(keys, key)
		values[key] = rawText(v)
	}
	return keys, values
}

func rawText(v json.RawMessage) string {
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return string(v)
}

func (a *API) table(c *crudCollection) *dataTable {
	items := a.opts.Memory.Items(c.cfg)
	t := &dataTable{collectionView: viewCollection(c, len(items)), Columns: []string{c.cfg.IDField}}
	for _, route := range c.routes {
		path := strings.TrimPrefix(route, "CRUD ")
		for _, e := range crudEndpoints {
			t.Endpoints = append(t.Endpoints, strings.Replace(e, "%s", path, 1))
		}
	}
	seen := map[string]bool{c.cfg.IDField: true}
	values := make([]map[string]string, len(items))
	for i, item := range items {
		keys, vals := fields(item)
		values[i] = vals
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				t.Columns = append(t.Columns, k)
			}
		}
	}
	for _, vals := range values {
		id := vals[c.cfg.IDField]
		row := dataRow{ID: id, Delete: uiPrefix + "data/" + url.PathEscape(c.cfg.Collection) + "/" + url.PathEscape(id) + "/delete"}
		for _, col := range t.Columns {
			row.Cells = append(row.Cells, vals[col])
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func (a *API) showData(w http.ResponseWriter, req *http.Request, name string, f *failure) {
	dp := dataPage{page: a.page(req, "Data"), Collections: a.collectionViews()}
	if f != nil {
		dp.Error = failureText(f)
	}
	if name == "" && len(dp.Collections) > 0 {
		name = dp.Collections[0].Name
	}
	if name == "" {
		render(w, "data", http.StatusOK, dp)
		return
	}
	c, missing := a.lookupCollection(name)
	if missing != nil {
		dp.Error = failureText(missing)
		render(w, "data", http.StatusNotFound, dp)
		return
	}
	dp.Current, dp.Table = name, a.table(c)
	render(w, "data", http.StatusOK, dp)
}

func (a *API) dataView(w http.ResponseWriter, req *http.Request) {
	a.showData(w, req, req.PathValue("name"), nil)
}

func (a *API) deleteFromUI(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	a.showData(w, req, name, a.removeItem(name, req.PathValue("id")))
}

func (a *API) resetFromUI(w http.ResponseWriter, _ *http.Request) {
	a.resetAll()
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}
