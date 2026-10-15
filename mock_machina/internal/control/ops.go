package control

import (
	"encoding/json"
	"net/http"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type failure struct {
	status int
	body   problem
}

func (f *failure) writeJSON(w http.ResponseWriter) { writeJSON(w, f.status, f.body) }

type routeView struct {
	ID        string   `json:"id"`
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Operation string   `json:"operation,omitempty"`
	Mode      string   `json:"mode"`
	Active    string   `json:"active"`
	States    []string `json:"states"`
	Pinned    string   `json:"pinned,omitempty"`
}

func (a *API) view(r *model.Route) routeView {
	pinned, _ := a.opts.Memory.Pinned(r.ID)
	return routeView{
		ID: r.ID, Method: string(r.Method), Path: r.Path, Operation: r.Operation,
		Mode: string(r.Mode), Active: r.Active, States: r.States.Names(), Pinned: pinned,
	}
}

func (a *API) routeViews() []routeView {
	rs := a.project.Load().Routes
	views := make([]routeView, len(rs))
	for i, r := range rs {
		views[i] = a.view(r)
	}
	return views
}

func (a *API) lookupRoute(id string) (*model.Route, *failure) {
	p := a.project.Load()
	if r, ok := p.Route(id); ok {
		return r, nil
	}
	ids := make([]string, len(p.Routes))
	for i, r := range p.Routes {
		ids[i] = r.ID
	}
	return nil, &failure{http.StatusNotFound, problem{Error: "unknown_route", Route: id, Hint: hint(id, ids)}}
}

func (a *API) setState(id, state string, persist bool) (routeView, *failure) {
	r, f := a.lookupRoute(id)
	if f != nil {
		return routeView{}, f
	}
	if _, ok := r.States.Get(state); !ok {
		names := r.States.Names()
		return routeView{}, &failure{http.StatusBadRequest, problem{Error: "unknown_state", Route: r.ID, State: state, Valid: names, Hint: hint(state, names)}}
	}
	if !persist {
		a.opts.Memory.Pin(r.ID, state)
		return a.view(r), nil
	}
	if _, err := config.SetActive(a.opts.Dir, r.ID, state); err != nil {
		return routeView{}, &failure{http.StatusConflict, problem{Error: "not_saved", Detail: err.Error()}}
	}
	a.opts.Memory.Unpin(r.ID)
	v := a.view(r)
	v.Active = state
	return v, nil
}

func (a *API) unpinRoute(id string) (routeView, *failure) {
	r, f := a.lookupRoute(id)
	if f != nil {
		return routeView{}, f
	}
	a.opts.Memory.Unpin(r.ID)
	return a.view(r), nil
}

func (a *API) resetAll() {
	a.opts.Memory.Reset()
	a.events.clear()
}

type requestsView struct {
	Total    int     `json:"total"`
	Errors   int     `json:"errors"`
	Requests []event `json:"requests"`
}

func (a *API) requestsView() requestsView {
	total, errs := a.events.counts()
	return requestsView{Total: total, Errors: errs, Requests: a.events.recent()}
}

type routeCoverage struct {
	ID       string   `json:"id"`
	Method   string   `json:"method"`
	Path     string   `json:"path"`
	Served   []string `json:"served"`
	Unserved []string `json:"unserved"`
}

type coverageView struct {
	Served  int             `json:"served"`
	Total   int             `json:"total"`
	Percent int             `json:"percent"`
	Routes  []routeCoverage `json:"routes"`
}

func (a *API) coverageView() coverageView {
	rs := a.project.Load().Routes
	v := coverageView{Routes: make([]routeCoverage, len(rs))}
	for i, r := range rs {
		rc := routeCoverage{ID: r.ID, Method: string(r.Method), Path: r.Path, Served: []string{}, Unserved: []string{}}
		for _, name := range r.States.Names() {
			if a.opts.Memory.Served(r.ID, name) {
				rc.Served = append(rc.Served, name)
			} else {
				rc.Unserved = append(rc.Unserved, name)
			}
		}
		v.Served += len(rc.Served)
		v.Total += len(rc.Served) + len(rc.Unserved)
		v.Routes[i] = rc
	}
	if v.Total > 0 {
		v.Percent = v.Served * 100 / v.Total
	}
	return v
}

type crudCollection struct {
	cfg    *model.CRUD
	routes []string
}

func (a *API) crudCollections() ([]string, map[string]*crudCollection) {
	var names []string
	byName := map[string]*crudCollection{}
	for _, r := range a.project.Load().Routes {
		if r.CRUD == nil {
			continue
		}
		c, ok := byName[r.CRUD.Collection]
		if !ok {
			c = &crudCollection{cfg: r.CRUD}
			byName[r.CRUD.Collection] = c
			names = append(names, r.CRUD.Collection)
		}
		c.routes = append(c.routes, string(r.Method)+" "+r.Path)
	}
	return names, byName
}

type collectionView struct {
	Name    string   `json:"name"`
	IDField string   `json:"id_field"`
	Seed    string   `json:"seed,omitempty"`
	Routes  []string `json:"routes"`
	Count   int      `json:"count"`
}

type collectionDetail struct {
	collectionView
	Items []json.RawMessage `json:"items"`
}

func viewCollection(c *crudCollection, count int) collectionView {
	v := collectionView{Name: c.cfg.Collection, IDField: c.cfg.IDField, Routes: c.routes, Count: count}
	if c.cfg.Data != nil {
		v.Seed = "data/" + c.cfg.Collection + ".json"
	}
	return v
}

func (a *API) collectionViews() []collectionView {
	names, byName := a.crudCollections()
	views := make([]collectionView, len(names))
	for i, name := range names {
		c := byName[name]
		views[i] = viewCollection(c, len(a.opts.Memory.Items(c.cfg)))
	}
	return views
}

func (a *API) lookupCollection(name string) (*crudCollection, *failure) {
	names, byName := a.crudCollections()
	if c, ok := byName[name]; ok {
		return c, nil
	}
	return nil, &failure{http.StatusNotFound, problem{Error: "unknown_collection", Collection: name, Hint: hint(name, names)}}
}

func (a *API) collectionDetail(name string) (collectionDetail, *failure) {
	c, f := a.lookupCollection(name)
	if f != nil {
		return collectionDetail{}, f
	}
	items := a.opts.Memory.Items(c.cfg)
	return collectionDetail{viewCollection(c, len(items)), items}, nil
}

func (a *API) removeItem(name, id string) *failure {
	c, f := a.lookupCollection(name)
	if f != nil {
		return f
	}
	if !a.opts.Memory.DeleteItem(c.cfg, id) {
		return &failure{http.StatusNotFound, problem{Error: "unknown_item", Collection: c.cfg.Collection, ID: id}}
	}
	return nil
}

type scenarioState struct {
	Route string `json:"route"`
	State string `json:"state"`
}

type scenarioView struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	States      []scenarioState `json:"states"`
}

type scenariosView struct {
	Applied   string         `json:"applied,omitempty"`
	Scenarios []scenarioView `json:"scenarios"`
}

func (a *API) scenariosView() scenariosView {
	list := a.project.Load().Scenarios
	v := scenariosView{Applied: a.opts.Memory.Applied(), Scenarios: make([]scenarioView, len(list))}
	for i, s := range list {
		states := make([]scenarioState, len(s.States))
		for j, st := range s.States {
			states[j] = scenarioState(st)
		}
		v.Scenarios[i] = scenarioView{Name: s.Name, Title: s.Title, Description: s.Description, States: states}
	}
	return v
}

func (a *API) applyScenario(name string) (scenariosView, *failure) {
	p := a.project.Load()
	s, ok := p.Scenario(name)
	if !ok {
		names := make([]string, len(p.Scenarios))
		for i, s := range p.Scenarios {
			names[i] = s.Name
		}
		return scenariosView{}, &failure{http.StatusNotFound, problem{Error: "unknown_scenario", Scenario: name, Hint: hint(name, names)}}
	}
	a.opts.Memory.Apply(s.Name, s.States)
	return a.scenariosView(), nil
}
