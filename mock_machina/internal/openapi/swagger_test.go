package openapi_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
)

func TestImport_Swagger2(t *testing.T) {
	t.Parallel()

	res := importSpec(t, "petstore-2.0.yaml")
	if res.Version != "Swagger 2.0" || res.BasePath != "/v1" {
		t.Errorf("version %q, base path %q", res.Version, res.BasePath)
	}
	list := route(t, res, "pets.list")
	if list.Path != "/v1/pets" || len(list.Request.Query) != 1 || inlineJSON(&list.Request.Query[0].Schema) != `{"type":"integer","format":"int32"}` {
		t.Errorf("list: path %s, query %+v", list.Path, list.Request.Query)
	}
	responses := make([]string, 0, len(list.Responses))
	for _, r := range list.Responses {
		responses = append(responses, r.Status+" "+r.Schema.Name)
	}
	if diff := cmp.Diff([]string{"200 Pets", "default Error"}, responses); diff != "" {
		t.Errorf("responses (-want +got):\n%s", diff)
	}
	get := route(t, res, "pets.get")
	if len(get.Request.Params) != 1 || get.Request.Params[0].Name != "petId" || !get.Request.Params[0].Required {
		t.Errorf("get params %+v", get.Request.Params)
	}
	if res.Notes[0] != "converted from Swagger 2.0" {
		t.Errorf("notes %q", res.Notes)
	}
}

func TestImport_Swagger2Conversions(t *testing.T) {
	t.Parallel()

	spec := `swagger: "2.0"
info: { title: t, version: "1" }
basePath: /api
produces: [application/json]
paths:
  /things:
    post:
      consumes: [application/json]
      parameters:
        - { name: body, in: body, required: true, schema: { $ref: "#/definitions/Thing" } }
        - { name: tags, in: query, type: array, items: { type: string } }
        - { name: X-Trace, in: header, type: string, required: true }
      responses:
        "201":
          description: made
          schema: { $ref: "#/definitions/Thing" }
          examples: { application/json: { id: t_1, name: Rope } }
    put:
      consumes: [application/x-www-form-urlencoded]
      parameters:
        - { name: name, in: formData, type: string }
      responses:
        "204": { description: done }
definitions:
  Thing:
    type: object
    required: [name]
    properties:
      id: { type: string }
      name: { type: string }
      parent: { $ref: "#/definitions/Thing" }
`
	res, err := openapi.Import([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	create := route(t, res, "things.create")
	if create.Request.Body == nil || create.Request.Body.Name != "Thing" {
		t.Errorf("body param = %+v", create.Request.Body)
	}
	if inlineJSON(&create.Request.Query[0].Schema) != `{"type":"array","items":{"type":"string"}}` || !create.Request.Headers[0].Required {
		t.Errorf("query %+v headers %+v", create.Request.Query, create.Request.Headers)
	}
	if diff := cmp.Diff([]string{`created {"id":"t_1","name":"Rope"}`}, states(create)); diff != "" {
		t.Errorf("states (-want +got):\n%s", diff)
	}
	if got := string(res.Schemas[0].JSON); got != `{"type":"object","required":["name"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"parent":{"$ref":"Thing"}}}` {
		t.Errorf("Thing = %s", got)
	}
	want := []string{
		"converted from Swagger 2.0",
		"all routes start with /api, the path of the first server",
		"things.replace: request body as application/x-www-form-urlencoded isn't checked",
	}
	if diff := cmp.Diff(want, res.Notes); diff != "" {
		t.Errorf("notes (-want +got):\n%s", diff)
	}
}
