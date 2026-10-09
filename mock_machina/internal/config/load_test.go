package config_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func load(t *testing.T, files map[string]string) (*model.Project, config.Problems) {
	t.Helper()
	p, probs, err := config.LoadFS(testkit.MapFS(t, files))
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	return p, probs
}

func loadClean(t *testing.T, files map[string]string) *model.Project {
	t.Helper()
	p, probs := load(t, files)
	if len(probs) > 0 {
		t.Fatalf("LoadFS reported problems:\n%s", probs)
	}
	return p
}

func TestLoadFS_EmptyProject(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{})
	if len(p.Routes) != 0 {
		t.Errorf("Routes = %d, want 0", len(p.Routes))
	}
}

func TestLoadFS_ResourceFile(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/users.yaml": `
			owners: { backend: [ademola], frontend: [ada] }

			list:
			  route: GET /users
			  summary: List users, newest first
			  states:
			    success:
			      headers: { Cache-Control: no-store }
			      body: users.json
			    empty:
			      body: { users: [], nextPage: null }

			delete:
			  route: DELETE /users/{id}
			  status: agreed
			  owners: { backend: [tunde] }
			  active: deleted
			  states:
			    not_found:
			      status: 404
			    deleted:
			      status: 204
		`,
		"routes/users/users.json": `{"users":[{"id":"u_1"}]}`,
	})

	file := "routes/users.yaml"
	team := model.Owners{Backend: []string{"ademola"}, Frontend: []string{"ada"}}
	want := &model.Project{Config: model.DefaultConfig(), Routes: []*model.Route{
		{
			ID:     "users.delete",
			Method: model.MethodDelete,
			Path:   "/users/{id}",
			Status: model.StatusAgreed,
			Owners: model.Owners{Backend: []string{"tunde"}},
			Group:  "users",
			Active: "deleted",
			Mode:   model.ModeActive,
			Serve:  model.ServeMock,
			Dir:    "routes/users",
			Src:    model.Source{File: file, Line: 13},
			States: model.States{
				{Name: "not_found", State: &model.State{Status: 404, Src: model.Source{File: file, Line: 19}}},
				{Name: "deleted", State: &model.State{Status: 204, Src: model.Source{File: file, Line: 21}}},
			},
		},
		{
			ID:      "users.list",
			Method:  model.MethodGet,
			Path:    "/users",
			Summary: "List users, newest first",
			Owners:  team,
			Group:   "users",
			Active:  "success",
			Mode:    model.ModeActive,
			Serve:   model.ServeMock,
			Dir:     "routes/users",
			Src:     model.Source{File: file, Line: 3},
			States: model.States{
				{Name: "success", State: &model.State{
					Headers: map[string]string{"Cache-Control": "no-store"},
					Body:    model.Body{File: "users.json", Data: []byte(`{"users":[{"id":"u_1"}]}`), ContentType: "application/json"},
					Src:     model.Source{File: file, Line: 7},
				}},
				{Name: "empty", State: &model.State{
					Body: model.Body{Data: []byte(`{"users":[],"nextPage":null}`), ContentType: "application/json"},
					Src:  model.Source{File: file, Line: 10},
				}},
			},
		},
	}}
	if diff := cmp.Diff(want, p); diff != "" {
		t.Errorf("project (-want +got):\n%s", diff)
	}
}

func TestLoadFS_Examples(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/orgs.yaml": "members:\n  route: GET /orgs/{org}/members/{id}\n  examples: { org: acme, id: 42 }\n  states:\n    ok: {}\n",
	})
	want := map[string]string{"org": "acme", "id": "42"}
	if diff := cmp.Diff(want, p.Routes[0].Examples); diff != "" {
		t.Errorf("examples (-want +got):\n%s", diff)
	}
}

func TestLoadFS_FileDefaults(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/orders.yaml": `
			status: implemented
			x-team: payments
			get:
			  route: GET /orders/{id}
			  states:
			    ok: {}
			cancel:
			  route: POST /orders/{id}/cancel
			  status: draft
			  x-note: new
			  states:
			    ok: {}
		`,
	})
	get, _ := p.Route("orders.get")
	cancel, _ := p.Route("orders.cancel")
	if get.Status != model.StatusImplemented || cancel.Status != model.StatusDraft {
		t.Errorf("statuses = %q, %q; want the file default, then the route's own", get.Status, cancel.Status)
	}
	if diff := cmp.Diff([]model.Extension{{Key: "x-note", Value: "new"}}, cancel.Extensions); diff != "" {
		t.Errorf("route extensions (-want +got):\n%s", diff)
	}
	if get.Extensions != nil {
		t.Errorf("file-level x- fields leaked into a route: %v", get.Extensions)
	}
}

func TestLoadFS_ActiveDefaultsToTheFirstState(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/health.yaml": "get:\n  route: GET /health\n  states:\n    up: {}\n    down:\n      status: 503\n",
	})
	if got := p.Routes[0].Active; got != "up" {
		t.Errorf("Active = %q, want the first state, up", got)
	}
}

func TestLoadFS_Generated(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/users.yaml": `
			list:
			  route: GET /users
			  generated: true
			  states:
			    ok: {}
			get:
			  route: GET /users/{id}
			  generated: false
			  states:
			    found: {}
			    gone:
			      status: 410
			      generated: true
		`,
	})
	list, _ := p.Route("users.list")
	get, _ := p.Route("users.get")
	found, _ := get.States.Get("found")
	gone, _ := get.States.Get("gone")
	if !list.Generated || get.Generated || found.Generated || !gone.Generated {
		t.Errorf("generated: list %v, get %v, found %v, gone %v; want true, false, false, true",
			list.Generated, get.Generated, found.Generated, gone.Generated)
	}
}

func TestLoadFS_Serve(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/users.yaml": "list:\n  route: GET /users\n  states:\n    ok: {}\nget:\n  route: GET /users/{id}\n  serve: proxy\n  states:\n    ok: {}\nput:\n  route: PUT /users/{id}\n  serve: mock\n  states:\n    ok: {}\n",
	})
	for id, want := range map[string]model.Serve{"users.list": model.ServeMock, "users.get": model.ServeProxy, "users.put": model.ServeMock} {
		if r, _ := p.Route(id); r.Serve != want {
			t.Errorf("%s: Serve = %q, want %q", id, r.Serve, want)
		}
	}
}

func TestLoadFS_Jitter(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/r.yaml": route("ok:\n      latency: { base: 2s, jitter: 500ms }")})
	st, _ := p.Routes[0].States.Get("ok")
	if st.Latency != 2*time.Second || st.Jitter != 500*time.Millisecond {
		t.Errorf("latency %v, jitter %v", st.Latency, st.Jitter)
	}
}

func TestLoadFS_Faults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		yaml string
		want model.Fault
	}{
		{"fault: reset", model.Fault{Type: model.FaultReset, Rate: 1}},
		{"fault: { type: truncated, rate: 0.25, after: 2s }", model.Fault{Type: model.FaultTruncated, Rate: 0.25, After: 2 * time.Second}},
		{"fault: { type: timeout }", model.Fault{Type: model.FaultTimeout, Rate: 1}},
	}
	for _, tc := range tests {
		p := loadClean(t, map[string]string{"routes/r.yaml": route("ok:\n      " + tc.yaml)})
		st, _ := p.Routes[0].States.Get("ok")
		if st.Fault == nil || *st.Fault != tc.want {
			t.Errorf("%s: fault = %+v, want %+v", tc.yaml, st.Fault, tc.want)
		}
	}
}

func TestLoadFS_Set(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/r.yaml": route(`ok:
      set: { signed_in: true, tries: 3, user: "{{ body.email }}", plan: pro, token: null }`)})
	st, _ := p.Routes[0].States.Get("ok")
	want := []model.Assignment{
		{Name: "signed_in", Value: true},
		{Name: "tries", Value: 3.0},
		{Name: "user", Value: "{{ body.email }}"},
		{Name: "plan", Value: "pro"},
		{Name: "token", Value: nil},
	}
	if diff := cmp.Diff(want, st.Set); diff != "" {
		t.Errorf("set (-want +got):\n%s", diff)
	}
}

func TestLoadFS_CRUD(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/cart.yaml": "items:\n  route: CRUD /cart/items\n  states:\n    ok: {}\nsaved:\n  route: CRUD /saved\n  crud: { collection: saved_items, idField: sku }\n  states:\n    ok: {}\n",
		"data/items.json":  `[{"id":"i_1","name":"Tea"}]`,
	})
	items, _ := p.Route("cart.items")
	saved, _ := p.Route("cart.saved")
	if items.Method != model.MethodCRUD || items.CRUD == nil || items.CRUD.Collection != "items" || items.CRUD.IDField != "id" || string(items.CRUD.Data) != `[{"id":"i_1","name":"Tea"}]` {
		t.Errorf("items: method %q, crud %+v", items.Method, items.CRUD)
	}
	if saved.CRUD == nil || saved.CRUD.Collection != "saved_items" || saved.CRUD.IDField != "sku" || saved.CRUD.Data != nil {
		t.Errorf("saved: crud %+v", saved.CRUD)
	}
}

func TestLoadFS_ValidateRequestOff(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/r.yaml": withState("validateRequest: false")})
	if st, _ := p.Routes[0].States.Get("ok"); !st.SkipRequestValidation {
		t.Error("validateRequest: false wasn't read")
	}
}

func TestLoadFS_TemplateOff(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/r.yaml": withState(`template: false
      body: { raw: "{{ anything goes }}" }`)})
	st, _ := p.Routes[0].States.Get("ok")
	if !st.Verbatim {
		t.Error("template: false didn't mark the state verbatim")
	}
}

func TestLoadFS_Latency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		yaml string
		want time.Duration
	}{
		{"latency: 800ms", 800 * time.Millisecond},
		{"latency: 1.5s", 1500 * time.Millisecond},
		{"latency: 1m", time.Minute},
		{"latency: 0s", 0},
		{"latency: { base: 2s, jitter: 500ms }", 2 * time.Second},
		{"latency: { base: 2s }", 2 * time.Second},
		{"status: 200", 0},
	}
	for _, tc := range tests {
		p := loadClean(t, map[string]string{"routes/r.yaml": route("ok:\n      " + tc.yaml)})
		st, _ := p.Routes[0].States.Get("ok")
		if st.Latency != tc.want {
			t.Errorf("%s: Latency = %v, want %v", tc.yaml, st.Latency, tc.want)
		}
	}
}

func TestLoadFS_InlineBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"keeps key order", `{ b: 1, a: 2 }`, `{"b":1,"a":2}`},
		{"nested", `{ user: { id: u_1, tags: [a, b] } }`, `{"user":{"id":"u_1","tags":["a","b"]}}`},
		{"list", `[1, 2, 3]`, `[1,2,3]`},
		{"scalars", `{ s: hi, n: 1.5, t: true, z: null, q: "42" }`, `{"s":"hi","n":1.5,"t":true,"z":null,"q":"42"}`},
		{"escapes strings", `{ s: "say \"hi\"" }`, `{"s":"say \"hi\""}`},
		{"empty mapping", `{}`, `{}`},
		{"alias", `{ a: &v [1, 2], b: *v }`, `{"a":[1,2],"b":[1,2]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := loadClean(t, map[string]string{"routes/r.yaml": route("ok:\n      body: " + tc.yaml)})
			st, _ := p.Routes[0].States.Get("ok")
			if got := string(st.Body.Data); got != tc.want {
				t.Errorf("body = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestLoadFS_BodyContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct{ file, want string }{
		{"a.json", "application/json"},
		{"a.txt", "text/plain; charset=utf-8"},
		{"a.html", "text/html; charset=utf-8"},
		{"a.xml", "application/xml"},
		{"a.JSON", "application/json"},
		{"a.png", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			p := loadClean(t, map[string]string{
				"routes/r.yaml":       route("ok:\n      body: " + tc.file),
				"routes/r/" + tc.file: "x",
			})
			st, _ := p.Routes[0].States.Get("ok")
			if st.Body.ContentType != tc.want {
				t.Errorf("content type of %s = %q, want %q", tc.file, st.Body.ContentType, tc.want)
			}
		})
	}
}

func TestLoadFS_BodyFromASharedFolder(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/r.yaml":    route("ok:\n      body: ../../shared/user.json"),
		"shared/user.json": `{"id":"u_1"}`,
	})
	st, _ := p.Routes[0].States.Get("ok")
	if string(st.Body.Data) != `{"id":"u_1"}` {
		t.Errorf("body = %q, want the shared file", st.Body.Data)
	}
}

func TestLoadFS_EmptyStateAndBody(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{"routes/r.yaml": route("ok:\n    gone:\n      status: 204\n      body:")})
	for _, name := range []string{"ok", "gone"} {
		st, found := p.Routes[0].States.Get(name)
		if !found || st.Body.Data != nil {
			t.Errorf("state %q = %+v, %v; want a state with no body", name, st, found)
		}
	}
}

func TestLoadFS_SortsRoutesByID(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/zeta.yaml":  "get:\n  route: GET /zeta\n  states:\n    ok: {}\n",
		"routes/alpha.yaml": "list:\n  route: GET /alpha\n  states:\n    ok: {}\nget:\n  route: GET /alpha/{id}\n  states:\n    ok: {}\n",
	})
	got := make([]string, 0, len(p.Routes))
	for _, r := range p.Routes {
		got = append(got, r.ID)
	}
	if diff := cmp.Diff([]string{"alpha.get", "alpha.list", "zeta.get"}, got); diff != "" {
		t.Errorf("route order (-want +got):\n%s", diff)
	}
}

func TestLoadFS_SkipsHiddenFilesAndBodyFolders(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"routes/.DS_Store":         "junk",
		"routes/.draft.yaml":       "not: [valid",
		"routes/README.md":         "notes",
		"routes/health/notes.json": "{}",
		"routes/health.yaml":       "get:\n  route: GET /health\n  states:\n    ok: {}\n",
		"routes/.health.yaml.swp":  "editor swap file",
	})
	if len(p.Routes) != 1 || p.Routes[0].ID != "health.get" {
		t.Errorf("Routes = %v, want only health.get", p.Routes)
	}
}

func TestLoad_FromDisk(t *testing.T) {
	t.Parallel()

	p, probs, err := config.Load(testkit.Path(t, "projects", "users", config.DirName))
	if err != nil || len(probs) > 0 {
		t.Fatalf("Load(users) = %v, %v", probs, err)
	}
	if len(p.Routes) != 1 || p.Routes[0].ID != "users.list" {
		t.Errorf("Routes = %v, want users.list", p.Routes)
	}
}

func TestLoadFS_RoutesIsAFile(t *testing.T) {
	t.Parallel()

	p, _, err := config.LoadFS(testkit.MapFS(t, map[string]string{"routes": "not a folder"}))
	if err == nil || p != nil {
		t.Errorf("LoadFS with routes as a file = %v, %v; want nil and an error", p, err)
	}
}

func TestLoad_MissingFolder(t *testing.T) {
	t.Parallel()

	p, _, err := config.Load(filepath.Join(t.TempDir(), "nope"))
	if !errors.Is(err, fs.ErrNotExist) || p != nil {
		t.Errorf("Load(missing) = %v, %v; want nil and fs.ErrNotExist", p, err)
	}
}

func route(states string) string {
	return "get:\n  route: GET /r\n  states:\n    " + states + "\n"
}
