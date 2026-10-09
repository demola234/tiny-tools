package diff_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

type stateSpec struct {
	name        string
	status      int
	contentType string
}

func route(id string, method model.Method, path string, states ...stateSpec) *model.Route {
	r := &model.Route{
		ID: id, Method: method, Path: path,
		Src: model.Source{File: "routes/" + id + ".yaml", Line: 3},
	}
	for _, s := range states {
		r.States = append(r.States, model.NamedState{
			Name:  s.name,
			State: &model.State{Status: s.status, Body: model.Body{ContentType: s.contentType}},
		})
	}
	if len(r.States) > 0 {
		r.Active = r.States[0].Name
	}
	return r
}

func project(routes ...*model.Route) *model.Project { return &model.Project{Routes: routes} }

func ok(name string) stateSpec { return stateSpec{name: name, contentType: "application/json"} }

func messages(changes []diff.Change) []string {
	out := make([]string, len(changes))
	for i, c := range changes {
		out[i] = c.Severity.String() + " " + c.Text()
	}
	return out
}

func TestCompare(t *testing.T) {
	t.Parallel()

	list := func(states ...stateSpec) *model.Route {
		return route("users.list", model.MethodGet, "/users", states...)
	}
	tests := []struct {
		name       string
		base, head *model.Project
		want       []string
	}{
		{
			name: "no changes",
			base: project(list(ok("success"))),
			head: project(list(ok("success"))),
			want: []string{},
		},
		{
			name: "route removed",
			base: project(list(ok("success"))),
			head: project(),
			want: []string{"breaking users.list: route removed (GET /users)"},
		},
		{
			name: "route added",
			base: project(),
			head: project(list(ok("success"))),
			want: []string{"safe users.list: route added (GET /users)"},
		},
		{
			name: "method or path changed",
			base: project(route("users.get", model.MethodGet, "/users/{id}", ok("found"))),
			head: project(route("users.get", model.MethodGet, "/members/{id}", ok("found"))),
			want: []string{"breaking users.get: now answers GET /members/{id}, was GET /users/{id}"},
		},
		{
			name: "renaming a path parameter isn't a change",
			base: project(route("users.get", model.MethodGet, "/users/{id}", ok("found"))),
			head: project(route("users.get", model.MethodGet, "/users/{userId}", ok("found"))),
			want: []string{},
		},
		{
			name: "route renamed",
			base: project(list(ok("success"))),
			head: project(route("users.all", model.MethodGet, "/users", ok("success"))),
			want: []string{"warning users.all: renamed from users.list (GET /users)"},
		},
		{
			name: "state removed",
			base: project(list(ok("success"), ok("empty"))),
			head: project(list(ok("success"))),
			want: []string{`breaking users.list: state "empty" removed`},
		},
		{
			name: "state added",
			base: project(list(ok("success"))),
			head: project(list(ok("success"), ok("slow"))),
			want: []string{`safe users.list: state "slow" added`},
		},
		{
			name: "status changes class",
			base: project(list(ok("success"))),
			head: project(list(stateSpec{"success", 404, "application/json"})),
			want: []string{`breaking users.list: state "success" now returns 404, was 200`},
		},
		{
			name: "status changes within a class",
			base: project(route("users.create", model.MethodPost, "/users", stateSpec{"created", 201, "application/json"})),
			head: project(route("users.create", model.MethodPost, "/users", stateSpec{"created", 200, "application/json"})),
			want: []string{`warning users.create: state "created" now returns 200, was 201`},
		},
		{
			name: "content type changed",
			base: project(list(ok("success"))),
			head: project(list(stateSpec{"success", 0, "text/plain; charset=utf-8"})),
			want: []string{`breaking users.list: state "success" now returns text/plain; charset=utf-8, was application/json`},
		},
		{
			name: "body removed",
			base: project(list(ok("success"))),
			head: project(list(stateSpec{"success", 0, ""})),
			want: []string{`breaking users.list: state "success" now returns no body, was application/json`},
		},
		{
			name: "default state changed",
			base: project(list(ok("success"), ok("empty"))),
			head: func() *model.Project {
				r := list(ok("success"), ok("empty"))
				r.Active = "empty"
				return project(r)
			}(),
			want: []string{`info users.list: default state now "empty", was "success"`},
		},
		{
			name: "breaking changes sort first",
			base: project(list(ok("success"), ok("empty")), route("users.get", model.MethodGet, "/users/{id}", ok("found"))),
			head: project(list(ok("success"), ok("slow")), route("users.add", model.MethodPost, "/users", ok("created"))),
			want: []string{
				"breaking users.get: route removed (GET /users/{id})",
				`breaking users.list: state "empty" removed`,
				`safe users.add: route added (POST /users)`,
				`safe users.list: state "slow" added`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := messages(diff.Compare(tc.base, tc.head))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("changes (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompare_MetadataIsInformational(t *testing.T) {
	t.Parallel()

	base := route("users.list", model.MethodGet, "/users", ok("success"))
	head := route("users.list", model.MethodGet, "/users", ok("success"))
	head.Summary = "List users, newest first"
	head.Status = model.StatusAgreed
	head.Owners = model.Owners{Backend: []string{"ademola"}}
	want := []string{
		"info users.list: owners changed",
		"info users.list: status now agreed, was draft",
		`info users.list: summary now "List users, newest first"`,
	}
	if d := cmp.Diff(want, messages(diff.Compare(project(base), project(head)))); d != "" {
		t.Errorf("changes (-want +got):\n%s", d)
	}
}

func TestCompare_ChangesCarryWhereToLook(t *testing.T) {
	t.Parallel()

	base := project(route("users.list", model.MethodGet, "/users", ok("success"), ok("empty")))
	head := project(route("users.list", model.MethodGet, "/users", ok("success")))
	head.Routes[0].Src = model.Source{File: "routes/users.yaml", Line: 12}
	got := diff.Compare(base, head)
	if len(got) != 1 || got[0].Route != "users.list" || got[0].State != "empty" || got[0].Src != head.Routes[0].Src {
		t.Errorf("change = %+v, want route users.list, state empty, at the head route's file and line", got)
	}

	removed := diff.Compare(base, project())
	if removed[0].Src != base.Routes[0].Src {
		t.Errorf("removed route points at %+v, want the base route's location", removed[0].Src)
	}
}

func TestSeverity_String(t *testing.T) {
	t.Parallel()

	for s, want := range map[diff.Severity]string{
		diff.Breaking: "breaking", diff.Warning: "warning", diff.Info: "info", diff.Safe: "safe",
	} {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", s, got, want)
		}
	}
}

func TestCompare_ChangesCarryTheRoutesOwners(t *testing.T) {
	t.Parallel()

	base := project(route("users.list", model.MethodGet, "/users", ok("success"), ok("empty")))
	base.Routes[0].Owners = model.Owners{Backend: []string{"old"}}
	head := project(route("users.list", model.MethodGet, "/users", ok("success")))
	head.Routes[0].Owners = model.Owners{Backend: []string{"ademola"}, Frontend: []string{"ada"}}
	got := diff.Compare(base, head)
	if diff := cmp.Diff(head.Routes[0].Owners, got[0].Owners); diff != "" {
		t.Errorf("owners (-want +got):\n%s", diff)
	}
	removed := diff.Compare(base, project())
	if diff := cmp.Diff(base.Routes[0].Owners, removed[0].Owners); diff != "" {
		t.Errorf("removed route's owners (-want +got):\n%s", diff)
	}
}

func TestCompare_BehaviourIsInformational(t *testing.T) {
	t.Parallel()

	rule := model.Rule{State: "success", When: []model.Condition{{Scope: model.ScopePath, Key: "id", Op: model.OpEq, Value: "u_1"}}}
	base := route("users.list", model.MethodGet, "/users", ok("success"))
	head := route("users.list", model.MethodGet, "/users", ok("success"))
	head.Mode, head.Rules = model.ModeRules, []model.Rule{rule}
	head.States[0].State.Fault = &model.Fault{Type: model.FaultReset, Rate: 0.5}
	want := []string{
		"info users.list: mode now rules, was active",
		"info users.list: rules changed",
		`info users.list: state "success" now fails with reset (rate 0.5)`,
	}
	base.Mode = model.ModeActive
	if d := cmp.Diff(want, messages(diff.Compare(project(base), project(head)))); d != "" {
		t.Errorf("changes (-want +got):\n%s", d)
	}

	same := route("users.list", model.MethodGet, "/users", ok("success"))
	same.Mode, same.Rules = model.ModeRules, []model.Rule{rule}
	if got := diff.Compare(project(same), project(head)); len(got) != 1 {
		t.Errorf("identical rules reported: %v", messages(got))
	}
}
