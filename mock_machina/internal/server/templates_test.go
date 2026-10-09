package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/seed"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func templateProject() *model.Project {
	return &model.Project{Routes: []*model.Route{
		{
			ID: "users.get", Method: model.MethodGet, Path: "/users/{id}", Active: "found",
			States: model.States{
				{Name: "found", State: &model.State{
					Headers: map[string]string{"X-User": "user-{{ path.id }}"},
					Body:    jsonBody(`{"id":"{{ path.id }}","page":"{{ query.page }}","call":"{{ call }}","token":"{{ uuid }}"}`),
				}},
				{Name: "raw", State: &model.State{Verbatim: true, Body: jsonBody(`{"id":"{{ path.id }}"}`)}},
				{Name: "text", State: &model.State{Body: model.Body{Data: []byte("hello {{ path.id }}"), ContentType: "text/plain; charset=utf-8"}}},
			},
		},
		{
			ID: "echo.create", Method: model.MethodPost, Path: "/echo", Active: "ok",
			States: model.States{{Name: "ok", State: &model.State{Status: 201, Body: jsonBody(`{"got":"{{ body.name }}","qty":"{{ body.qty }}"}`)}}},
		},
	}}
}

func get2(t *testing.T, h http.Handler, target, state string) *httptest.ResponseRecorder {
	t.Helper()
	header := map[string]string{}
	if state != "" {
		header["X-Mock-State"] = state
	}
	return do(t, h, "GET", target, header)
}

func TestTemplates_Render(t *testing.T) {
	t.Parallel()

	h, err := server.New(templateProject(), server.Options{Seed: seed.New(1)})
	if err != nil {
		t.Fatal(err)
	}
	rec := get2(t, h, "/users/u_7?page=3", "")
	body := rec.Body.String()
	if !strings.HasPrefix(body, `{"id":"u_7","page":"3","call":1,"token":"`) {
		t.Errorf("body = %s", body)
	}
	if got := rec.Header().Get("X-User"); got != "user-u_7" {
		t.Errorf("X-User = %q", got)
	}
	if body := get2(t, h, "/users/u_7", "raw").Body.String(); body != `{"id":"{{ path.id }}"}` {
		t.Errorf("template: false body = %s", body)
	}
	if body := get2(t, h, "/users/u_7", "text").Body.String(); body != "hello u_7" {
		t.Errorf("text body = %q", body)
	}
	req := httptest.NewRequestWithContext(t.Context(), "POST", "/echo", strings.NewReader(`{"name":"Ada","qty":2}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 || rec.Body.String() != `{"got":"Ada","qty":2}` {
		t.Errorf("echo = %d %s", rec.Code, rec.Body)
	}
}

func TestTemplates_AreSeeded(t *testing.T) {
	t.Parallel()

	tokens := func(s uint64) string {
		h, err := server.New(templateProject(), server.Options{Seed: seed.New(s)})
		if err != nil {
			t.Fatal(err)
		}
		all := make([]string, 0, 3)
		for range 3 {
			all = append(all, get2(t, h, "/users/u_1", "").Body.String())
		}
		return strings.Join(all, "\n")
	}
	if a, b := tokens(5), tokens(5); a != b {
		t.Errorf("same seed, different bodies:\n%s\n%s", a, b)
	}
	if a, b := tokens(5), tokens(6); a == b {
		t.Error("another seed gave the same uuids")
	}
}

func TestUsesRandomness(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		p    *model.Project
		want bool
	}{
		"plain states":    {usersProject(), false},
		"random mode":     {modeProject(model.ModeRandom), true},
		"uuid template":   {templateProject(), true},
		"rules":           {rulesProject(), false},
		"sequential mode": {modeProject(model.ModeSequential), false},
		"fault rate":      {faultProject(model.Fault{Type: model.FaultReset, Rate: 0.5}), true},
		"certain fault":   {faultProject(model.Fault{Type: model.FaultReset, Rate: 1}), false},
	} {
		if got := server.UsesRandomness(tc.p); got != tc.want {
			t.Errorf("%s: UsesRandomness = %v, want %v", name, got, tc.want)
		}
	}
}
