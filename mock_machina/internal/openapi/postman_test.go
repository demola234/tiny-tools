package openapi_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func TestImport_Postman(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(testkit.Path(t, "specs", "shop.postman_collection.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := openapi.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "Postman Collection v2.1" || res.Title != "Shop" {
		t.Errorf("version %q, title %q", res.Version, res.Title)
	}
	ids := make([]string, 0, len(res.Routes))
	for _, r := range res.Routes {
		ids = append(ids, r.ID+" "+string(r.Method)+" "+r.Path+" "+r.Summary)
	}
	want := []string{"users.list GET /users List users", "users.get GET /users/{id} Get user", "orders.create POST /users/{userId}/orders Create order"}
	if diff := cmp.Diff(want, ids); diff != "" {
		t.Errorf("routes (-want +got):\n%s", diff)
	}
	list := route(t, res, "users.list")
	if diff := cmp.Diff([]string{
		`some_users {"users":[{"id":"u_1","name":"Ada","age":36,"nick":null},{"id":"u_2","name":"Tunde","age":29,"nick":"T"}],"next":2}`,
		`no_users {"users":[],"next":null}`,
		`session_expired {"error":"session expired"}`,
	}, states(list)); diff != "" {
		t.Errorf("states (-want +got):\n%s", diff)
	}
	if len(list.Request.Query) != 1 || list.Request.Query[0].Name != "page" {
		t.Errorf("query %+v", list.Request.Query)
	}
	ok := inlineJSON(&list.Responses[0].Schema)
	wantSchema := `{"type":"object","required":["users","next"],"properties":{"users":{"type":"array","items":{"type":"object","required":["id","name","age","nick"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"age":{"type":"integer"},"nick":{"type":["string","null"]}}}},"next":{"type":["integer","null"]}},"x-mockmachina-inferred":true}`
	if ok != wantSchema {
		t.Errorf("inferred 200 schema:\n%s\nwant\n%s", ok, wantSchema)
	}
	orders := route(t, res, "orders.create")
	if diff := cmp.Diff([]string{`created {"id":"o_1","total":12.5}`, "plain_text oops"}, states(orders)); diff != "" {
		t.Errorf("order states (-want +got):\n%s", diff)
	}
	if st, _ := orders.States.Get("plain_text"); st.Body.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("plain text body type %q", st.Body.ContentType)
	}
}

func TestExport_Postman(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "awkward-3.1.yaml")
	p := &model.Project{Routes: res.Routes, Schemas: res.Schemas}
	collection, environment, err := openapi.ExportPostman(p, openapi.Info{Title: "Awkward"})
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Info struct{ Name, Schema string } `json:"info"`
		Item []struct {
			Name string `json:"name"`
			Item []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Request     struct {
					Method string `json:"method"`
					Header []struct{ Key, Value string }
					URL    struct {
						Raw string `json:"raw"`
					} `json:"url"`
				} `json:"request"`
				Response []struct {
					Name string `json:"name"`
					Code int    `json:"code"`
					Body string `json:"body"`
				} `json:"response"`
			} `json:"item"`
		} `json:"item"`
	}
	if err := json.Unmarshal(collection, &c); err != nil {
		t.Fatal(err)
	}
	if c.Info.Name != "Awkward" || !strings.Contains(c.Info.Schema, "v2.1.0") || len(c.Item) != 2 || c.Item[0].Name != "orders" {
		t.Fatalf("collection = %+v", c.Info)
	}
	first := c.Item[0].Item[0]
	if first.Name != "orders.list · some_orders" || first.Request.Method != http.MethodGet || first.Request.URL.Raw != "{{baseUrl}}/v1/orders" {
		t.Errorf("first request = %+v", first)
	}
	if len(first.Request.Header) == 0 || first.Request.Header[0].Key != "X-Mock-State" || first.Request.Header[0].Value != "some_orders" {
		t.Errorf("headers = %+v", first.Request.Header)
	}
	if len(first.Response) != 1 || first.Response[0].Code != 200 || !strings.HasPrefix(first.Response[0].Body, `{"orders"`) {
		t.Errorf("saved response = %+v", first.Response)
	}
	checkEnvironment(t, environment)

	back, err := openapi.Import(collection)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Routes {
		again := route(t, back, r.ID)
		if diff := cmp.Diff(r.States.Names(), again.States.Names()); diff != "" {
			t.Errorf("%s states after a Postman round trip (-want +got):\n%s", r.ID, diff)
		}
	}
}

func checkEnvironment(t *testing.T, environment []byte) {
	t.Helper()
	var env struct {
		Name   string `json:"name"`
		Values []struct {
			Key, Value string
			Enabled    bool
		} `json:"values"`
	}
	if err := json.Unmarshal(environment, &env); err != nil {
		t.Fatal(err)
	}
	if env.Name != "Awkward mock" || len(env.Values) != 1 || env.Values[0].Key != "baseUrl" || env.Values[0].Value != "http://localhost:4001" {
		t.Errorf("environment = %+v", env)
	}
}
