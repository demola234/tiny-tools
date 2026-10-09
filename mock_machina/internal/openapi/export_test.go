package openapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestExport_RoundTripsTheCorpus(t *testing.T) {
	t.Parallel()

	files, _ := filepath.Glob(testkit.Path(t, "specs", "*.yaml"))
	for _, f := range files {
		first := importSpec(t, filepath.Base(f))
		exported, err := openapi.Export(&model.Project{Routes: first.Routes, Schemas: first.Schemas}, openapi.Info{Title: first.Title})
		if err != nil {
			t.Fatalf("%s: Export: %v", filepath.Base(f), err)
		}
		second, err := openapi.Import(exported)
		if err != nil {
			t.Fatalf("%s: reimport: %v\n%s", filepath.Base(f), err, exported)
		}
		a, _ := config.RenderContract(first.Routes, first.Schemas)
		b, _ := config.RenderContract(second.Routes, second.Schemas)
		if diff := cmp.Diff(text(a), text(b)); diff != "" {
			t.Errorf("%s: round trip changed the contract (-first +second):\n%s", filepath.Base(f), diff)
		}
	}
}

func text(files map[string][]byte) map[string]string {
	out := make(map[string]string, len(files))
	for k, v := range files {
		out[k] = string(v)
	}
	return out
}

type flatState struct {
	Name, Body, Headers, Set string
	Status                   int
	Latency, Jitter          time.Duration
	Fault                    string
	Verbatim, SkipValidation bool
}

type flatRoute struct {
	ID, Method, Path, Summary, Status, Active, Mode, Serve, Rules, Owners string
	States                                                                []flatState
}

func compact(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func flatten(t *testing.T, routes []*model.Route) []flatRoute {
	t.Helper()
	out := make([]flatRoute, 0, len(routes))
	for _, r := range routes {
		out = append(out, flattenRoute(r))
	}
	slices.SortFunc(out, func(a, b flatRoute) int { return strings.Compare(a.ID, b.ID) })
	return out
}

func flattenRoute(r *model.Route) flatRoute {
	fr := flatRoute{
		ID: r.ID, Method: string(r.Method), Path: r.Path, Summary: r.Summary, Status: string(r.Status.Effective()),
		Active: r.Active, Mode: string(r.Mode), Serve: string(r.Serve), Owners: compact(r.Owners), Rules: rulesText(r.Rules),
	}
	if fr.Mode == "" {
		fr.Mode = string(model.ModeActive)
	}
	if fr.Serve == "" {
		fr.Serve = string(model.ServeMock)
	}
	for _, ns := range r.States {
		fr.States = append(fr.States, flattenState(ns))
	}
	return fr
}

func rulesText(rules []model.Rule) string {
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		conds := make([]string, 0, len(rule.When))
		for _, c := range rule.When {
			conds = append(conds, string(c.Scope)+"."+c.Key+" "+string(c.Op)+" "+compact(c.Value)+compact(c.Values))
		}
		out = append(out, rule.State+": "+strings.Join(conds, ", "))
	}
	return strings.Join(out, "; ")
}

func flattenState(ns model.NamedState) flatState {
	st := ns.State
	body := ""
	if len(st.Body.Data) > 0 {
		var v any
		if json.Unmarshal(st.Body.Data, &v) == nil {
			body = compact(v)
		} else {
			body = string(st.Body.Data)
		}
	}
	fault := ""
	if st.Fault != nil {
		fault = compact(st.Fault)
	}
	set := make([]string, 0, len(st.Set))
	for _, a := range st.Set {
		set = append(set, a.Name+"="+compact(a.Value))
	}
	return flatState{
		Name: ns.Name, Body: body, Headers: compact(st.Headers), Set: strings.Join(set, ","), Status: st.EffectiveStatus(),
		Latency: st.Latency, Jitter: st.Jitter, Fault: fault, Verbatim: st.Verbatim, SkipValidation: st.SkipRequestValidation,
	}
}

func TestExport_KeepsMockMachinaFields(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), ".mockmachina")
	for name, content := range map[string]string{
		"routes/session.yaml": `owners: { backend: [ademola], frontend: [ada] }
status: agreed
create:
  route: POST /session
  summary: Sign in
  serve: mock
  rules:
    - when: { body.password: { ne: secret }, header.x-test: { exists: true } }
      state: wrong_password
  states:
    signed_in:
      status: 201
      headers: { Set-Cookie: session=s1 }
      latency: { base: 1s, jitter: 200ms }
      set: { signed_in: true }
      body: { token: "{{ uuid }}" }
    wrong_password: { status: 401, validateRequest: false, body: { error: wrong } }
    flaky: { fault: { type: reset, rate: 0.5, after: 2s } }
    raw: { template: false, body: { t: "{{ not a template }}" } }
pay:
  route: POST /pay
  mode: sequential
  active: done
  states:
    busy: { status: 503 }
    done: { status: 201 }
`,
	} {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	original, probs, err := config.Load(dir)
	if err != nil || probs.HasErrors() {
		t.Fatalf("load: %v %v", probs, err)
	}
	exported, err := openapi.Export(original, openapi.Info{Title: "Shop"})
	if err != nil {
		t.Fatal(err)
	}
	back, err := openapi.Import(exported)
	if err != nil {
		t.Fatalf("reimport: %v\n%s", err, exported)
	}
	if diff := cmp.Diff(flatten(t, original.Routes), flatten(t, back.Routes)); diff != "" {
		t.Errorf("export lost something (-original +reimported):\n%s\n%s", diff, exported)
	}
}

func TestExport_Golden(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "awkward-3.1.yaml")
	exported, err := openapi.Export(&model.Project{Routes: res.Routes, Schemas: res.Schemas}, openapi.Info{Title: "Awkward shop", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	testkit.Golden(t, exported, "export/awkward.yaml")
}
