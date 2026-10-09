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

const cartData = `[{"id":"i_1","name":"Tea","qty":1},{"id":"i_2","name":"Milk","qty":2}]`

func crudProject(data string) *model.Project {
	var raw []byte
	if data != "" {
		raw = []byte(data)
	}
	return &model.Project{Routes: []*model.Route{{
		ID: "cart.items", Method: model.MethodCRUD, Path: "/cart/items", Active: "ok",
		CRUD: &model.CRUD{Collection: "items", IDField: "id", Data: raw},
		States: model.States{
			{Name: "ok", State: &model.State{}},
			{Name: "down", State: &model.State{Status: 503, Body: jsonBody(`{"error":"unavailable"}`)}},
		},
	}}}
}

type crudStep struct {
	method, target, body string
	state                string
	code                 int
	want                 string
}

func runSteps(t *testing.T, h http.Handler, steps []crudStep) {
	t.Helper()
	for i, s := range steps {
		req := httptest.NewRequestWithContext(t.Context(), s.method, s.target, strings.NewReader(s.body))
		if s.state != "" {
			req.Header.Set("X-Mock-State", s.state)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != s.code || rec.Body.String() != s.want {
			t.Errorf("step %d %s %s = %d %s\nwant %d %s", i+1, s.method, s.target, rec.Code, rec.Body, s.code, s.want)
		}
	}
}

func TestCRUD_Operations(t *testing.T) {
	t.Parallel()

	h, err := server.New(crudProject(cartData), server.Options{Seed: seed.New(1)})
	if err != nil {
		t.Fatal(err)
	}
	runSteps(t, h, []crudStep{
		{"GET", "/cart/items", "", "", 200, cartData},
		{"GET", "/cart/items/i_1", "", "", 200, `{"id":"i_1","name":"Tea","qty":1}`},
		{"GET", "/cart/items/i_9", "", "", 404, `{"error":"not_found","id":"i_9"}` + "\n"},
		{"POST", "/cart/items", `{"id":"i_1"}`, "", 409, `{"error":"conflict","id":"i_1"}` + "\n"},
		{"POST", "/cart/items", `"bread"`, "", 400, `{"error":"invalid_body","message":"the body must be a JSON object"}` + "\n"},
		{"PATCH", "/cart/items/i_1", `{"qty":5,"note":"x"}`, "", 200, `{"id":"i_1","name":"Tea","qty":5,"note":"x"}`},
		{"PUT", "/cart/items/i_2", `{"name":"Oat milk"}`, "", 200, `{"id":"i_2","name":"Oat milk"}`},
		{"PUT", "/cart/items/i_9", `{"name":"x"}`, "", 404, `{"error":"not_found","id":"i_9"}` + "\n"},
		{"DELETE", "/cart/items/i_1", "", "", 204, ""},
		{"GET", "/cart/items/i_1", "", "", 404, `{"error":"not_found","id":"i_1"}` + "\n"},
		{"GET", "/cart/items?name=Oat%20milk", "", "", 200, `[{"id":"i_2","name":"Oat milk"}]`},
		{"GET", "/cart/items?name=Tea", "", "", 200, `[]`},
		{"GET", "/cart/items", "", "down", 503, `{"error":"unavailable"}`},
	})
}

func TestCRUD_CreateGivesAnID(t *testing.T) {
	t.Parallel()

	h, err := server.New(crudProject(""), server.Options{Seed: seed.New(1)})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), "POST", "/cart/items", strings.NewReader(`{"name":"Bread"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != 201 || !strings.HasPrefix(body, `{"id":"`) || !strings.HasSuffix(body, `","name":"Bread"}`) {
		t.Fatalf("POST = %d %s", rec.Code, body)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(body, `{"id":"`), `","name":"Bread"}`)
	if got := rec.Header().Get("Location"); got != "/cart/items/"+id {
		t.Errorf("Location = %q", got)
	}
	runSteps(t, h, []crudStep{{"GET", "/cart/items", "", "", 200, "[" + body + "]"}})
}

func TestCRUD_MemoryAndReset(t *testing.T) {
	t.Parallel()

	mem := server.NewMemory()
	h, _ := server.New(crudProject(cartData), server.Options{Memory: mem})
	runSteps(t, h, []crudStep{{"DELETE", "/cart/items/i_1", "", "", 204, ""}})

	reloaded, _ := server.New(crudProject(cartData), server.Options{Memory: mem})
	runSteps(t, reloaded, []crudStep{{"GET", "/cart/items/i_1", "", "", 404, `{"error":"not_found","id":"i_1"}` + "\n"}})

	edited := `[{"id":"i_1","name":"Coffee"}]`
	reset, _ := server.New(crudProject(edited), server.Options{Memory: mem})
	runSteps(t, reset, []crudStep{{"GET", "/cart/items", "", "", 200, edited}})
}
