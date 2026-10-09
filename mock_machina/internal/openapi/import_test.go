package openapi_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
	"github.com/demola234/tiny-tools/mock_machina/internal/testkit"
)

func importSpec(t *testing.T, name string) *openapi.Result {
	t.Helper()
	data, err := os.ReadFile(testkit.Path(t, "specs", name))
	if err != nil {
		t.Fatal(err)
	}
	res, err := openapi.Import(data)
	if err != nil {
		t.Fatalf("Import(%s): %v", name, err)
	}
	return res
}

func route(t *testing.T, res *openapi.Result, id string) *model.Route {
	t.Helper()
	i := slices.IndexFunc(res.Routes, func(r *model.Route) bool { return r.ID == id })
	if i < 0 {
		t.Fatalf("no route %s", id)
	}
	return res.Routes[i]
}

func states(r *model.Route) []string {
	out := make([]string, 0, len(r.States))
	for _, ns := range r.States {
		body := string(ns.State.Body.Data)
		if ns.State.Body.Generate {
			body = "generate"
		}
		out = append(out, ns.Name+" "+strings.TrimSpace(body))
	}
	return out
}

func inlineJSON(ref *model.SchemaRef) string {
	if ref == nil || ref.Inline == nil {
		return ""
	}
	return string(ref.Inline.JSON)
}

func TestImport_Awkward(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "awkward-3.1.yaml")
	if res.Title != "Awkward shop" || res.Version != "3.1.0" || res.BasePath != "/v1" {
		t.Errorf("title %q, version %q, base path %q", res.Title, res.Version, res.BasePath)
	}
	ids := make([]string, 0, len(res.Routes))
	for _, r := range res.Routes {
		ids = append(ids, r.ID+" "+string(r.Method)+" "+r.Path)
	}
	wantIDs := []string{
		"orders.list GET /v1/orders", "orders.create POST /v1/orders",
		"orders.get GET /v1/orders/{orderId}", "orders.delete DELETE /v1/orders/{orderId}",
		"tree.list GET /v1/categories/{id}/tree",
	}
	if diff := cmp.Diff(wantIDs, ids); diff != "" {
		t.Errorf("routes (-want +got):\n%s", diff)
	}
}

func TestImport_AwkwardList(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "awkward-3.1.yaml")
	list := route(t, res, "orders.list")
	if list.Summary != "List orders" || list.Active != "some_orders" {
		t.Errorf("list: summary %q, active %q", list.Summary, list.Active)
	}
	if diff := cmp.Diff([]string{
		`some_orders {"orders":[{"id":"o_1","total":12.5,"note":null,"lines":[]}],"next":2}`,
		`no_orders {"orders":[],"next":null}`,
		"server_error generate",
	}, states(list)); diff != "" {
		t.Errorf("list states (-want +got):\n%s", diff)
	}
	if s, _ := list.States.Get("server_error"); s.Status != 500 {
		t.Errorf("default became status %d, want 500", s.Status)
	}
	all := slices.Concat(list.Request.Query, list.Request.Headers)
	params := make([]string, 0, len(all))
	for _, p := range all {
		params = append(params, p.Name+" "+inlineJSON(&p.Schema)+" "+map[bool]string{true: "required", false: "optional"}[p.Required])
	}
	if diff := cmp.Diff([]string{
		`page {"type":"integer","minimum":1} optional`,
		`status {"type":"string","enum":["open","closed"]} optional`,
		`X-Request-Id {"type":"string"} required`,
	}, params); diff != "" {
		t.Errorf("list params (-want +got):\n%s", diff)
	}
	responses := make([]string, 0, len(list.Responses))
	for _, r := range list.Responses {
		responses = append(responses, r.Status+" "+r.Schema.Name)
	}
	if diff := cmp.Diff([]string{"200 OrderList", "default Problem"}, responses); diff != "" {
		t.Errorf("list responses (-want +got):\n%s", diff)
	}
	del := route(t, res, "orders.delete")
	if len(del.Responses) != 1 || del.Responses[0].Status != "204" || inlineJSON(&del.Responses[0].Schema) != "{}" {
		t.Errorf("a response without content should be documented with {}: %+v", del.Responses)
	}
}

func TestImport_AwkwardOthers(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "awkward-3.1.yaml")
	create := route(t, res, "orders.create")
	if create.Summary != "Create an order." {
		t.Errorf("create summary %q", create.Summary)
	}
	if got := inlineJSON(create.Request.Body); got != `{"type":"object","required":["lines"],"properties":{"lines":{"type":"array","minItems":1,"items":{"$ref":"Line"}}}}` {
		t.Errorf("create body schema %s", got)
	}
	if diff := cmp.Diff([]string{"created generate", `invalid {"error":"cart is empty"}`}, states(create)); diff != "" {
		t.Errorf("create states (-want +got):\n%s", diff)
	}

	get := route(t, res, "orders.get")
	if len(get.Request.Params) != 1 || get.Request.Params[0].Name != "orderId" || !get.Request.Params[0].Required {
		t.Errorf("get params %+v", get.Request.Params)
	}
	if diff := cmp.Diff([]string{"success generate", "not_found generate"}, states(get)); diff != "" {
		t.Errorf("get states (-want +got):\n%s", diff)
	}
	del := route(t, res, "orders.delete")
	if diff := cmp.Diff([]string{"no_content "}, states(del)); diff != "" || del.States[0].State.Status != 204 {
		t.Errorf("delete states (-want +got):\n%s", diff)
	}
}

func TestImport_AwkwardSchemasAndNotes(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "awkward-3.1.yaml")
	names := make([]string, 0, len(res.Schemas))
	for _, s := range res.Schemas {
		names = append(names, s.Name)
	}
	if diff := cmp.Diff([]string{"Card", "Category", "Line", "Order", "OrderList", "Problem"}, names); diff != "" {
		t.Errorf("schemas (-want +got):\n%s", diff)
	}
	order := res.Schemas[slices.Index(names, "Order")]
	if !strings.Contains(string(order.JSON), `"items":{"$ref":"Line"}`) || !strings.Contains(string(order.JSON), `{"$ref":"Card"}`) {
		t.Errorf("Order refs weren't rewritten: %s", order.JSON)
	}
	if diff := cmp.Diff([]string{
		`all routes start with /v1, the path of the first server`,
		`orders.delete: cookie parameter "session" isn't checked`,
	}, res.Notes); diff != "" {
		t.Errorf("notes (-want +got):\n%s", diff)
	}
}

func TestImport_OpenAPI30Conversions(t *testing.T) {
	t.Parallel()

	spec := `openapi: 3.0.3
info: { title: t, version: "1" }
paths: {}
components:
  schemas:
    Price:
      type: object
      properties:
        amount: { type: number, minimum: 0, exclusiveMinimum: true }
        discount: { type: number, maximum: 1, exclusiveMaximum: false }
        note: { type: string, nullable: true }
        any: { nullable: true }
`
	res, err := openapi.Import([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"object","properties":{"amount":{"type":"number","exclusiveMinimum":0},"discount":{"type":"number","maximum":1},"note":{"type":["string","null"]},"any":{}}}`
	if got := string(res.Schemas[0].JSON); got != want {
		t.Errorf("Price =\n%s\nwant\n%s", got, want)
	}
}

func TestImport_Errors(t *testing.T) {
	t.Parallel()

	for spec, want := range map[string]string{
		"not: [yaml":                  "isn't valid YAML or JSON",
		"info: { title: t }\n":        "isn't an OpenAPI or Swagger document (no openapi or swagger field)",
		"openapi: 4.0.0\npaths: {}\n": `OpenAPI 4.0.0 isn't supported (3.0 and 3.1 are)`,
	} {
		if _, err := openapi.Import([]byte(spec)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Import(%q) error = %v, want %q", spec, err, want)
		}
	}
}

func TestImport_Corpus(t *testing.T) {
	t.Parallel()

	files, _ := filepath.Glob(testkit.Path(t, "specs", "*.yaml"))
	if len(files) < 4 {
		t.Fatalf("corpus has %d 3.x specs", len(files))
	}
	for _, f := range files {
		res := importSpec(t, filepath.Base(f))
		if len(res.Routes) == 0 {
			t.Errorf("%s: no routes", filepath.Base(f))
		}
	}
}
