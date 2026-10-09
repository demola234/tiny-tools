package diff_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/diff"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func named(name, js string) *model.Schema { return &model.Schema{Name: name, JSON: []byte(js)} }

func inline(js string) model.SchemaRef {
	return model.SchemaRef{Inline: &model.Schema{JSON: []byte(js)}}
}

func contract(schemas []*model.Schema, req *model.Request, responses ...model.Response) *model.Project {
	r := route("users.list", model.MethodPost, "/users", ok("success"))
	r.Request, r.Responses = req, responses
	p := project(r)
	p.Schemas = schemas
	return p
}

func TestCompare_ResponseFields(t *testing.T) {
	t.Parallel()

	base := contract([]*model.Schema{
		named("User", `{"type":"object","required":["id","email"],"properties":{"id":{"type":"string"},"email":{"type":"string"},"age":{"type":"integer"},"nick":{"type":"string"}}}`),
		named("Page", `{"type":"object","properties":{"users":{"type":"array","items":{"$ref":"User"}}}}`),
	}, nil, model.Response{Status: "200", Schema: model.SchemaRef{Name: "Page"}})
	head := contract([]*model.Schema{
		named("User", `{"type":"object","required":["id"],"properties":{"id":{"type":"integer"},"email":{"type":"string"},"nick":{"type":["string","null"]},"avatar":{"type":"string"}}}`),
		named("Page", `{"allOf":[{"type":"object","properties":{"users":{"type":"array","items":{"$ref":"User"}}}}]}`),
	}, nil, model.Response{Status: "200", Schema: model.SchemaRef{Name: "Page"}})
	want := []string{
		`breaking users.list: response 200: users[].age removed`,
		`breaking users.list: response 200: users[].id is now integer, was string`,
		`breaking users.list: response 200: users[].nick can now be null`,
		`warning users.list: response 200: users[].email is no longer required, so apps may not get it`,
		`safe users.list: response 200: users[].avatar added`,
	}
	if d := cmp.Diff(want, messages(diff.Compare(base, head))); d != "" {
		t.Errorf("changes (-want +got):\n%s", d)
	}
}

func TestCompare_RequestFields(t *testing.T) {
	t.Parallel()

	param := func(name string, required bool, js string) model.Param {
		return model.Param{Name: name, Required: required, Schema: inline(js)}
	}
	base := contract(nil, &model.Request{
		Query: []model.Param{param("page", false, `{"type":"integer"}`), param("q", false, `{"type":"string"}`)},
		Body:  &model.SchemaRef{Inline: &model.Schema{JSON: []byte(`{"type":"object","required":["email"],"properties":{"email":{"type":"string"},"name":{"type":["string","null"]},"old":{"type":"string"}}}`)}},
	})
	head := contract(nil, &model.Request{
		Query:   []model.Param{param("page", true, `{"type":"integer"}`)},
		Headers: []model.Param{param("X-Tenant", true, `{"type":"string"}`), param("X-Trace", false, `{"type":"string"}`)},
		Body:    &model.SchemaRef{Inline: &model.Schema{JSON: []byte(`{"type":"object","required":["email","password"],"properties":{"email":{"type":"string"},"name":{"type":"string"},"password":{"type":"string"},"note":{"type":"string"}}}`)}},
	})
	want := []string{
		`breaking users.list: request body: name no longer accepts null`,
		`breaking users.list: request body: password is now required`,
		`breaking users.list: request headers: X-Tenant is now required`,
		`breaking users.list: request query: page is now required`,
		`info users.list: request body: old removed`,
		`info users.list: request query: q removed`,
		`safe users.list: request body: note added`,
		`safe users.list: request headers: X-Trace added`,
	}
	if d := cmp.Diff(want, messages(diff.Compare(base, head))); d != "" {
		t.Errorf("changes (-want +got):\n%s", d)
	}
}

func TestCompare_SameSchemasNoChanges(t *testing.T) {
	t.Parallel()

	schemas := []*model.Schema{named("Node", `{"type":"object","properties":{"next":{"$ref":"Node"},"v":{"type":"string"}}}`)}
	a := contract(schemas, nil, model.Response{Status: "200", Schema: model.SchemaRef{Name: "Node"}})
	b := contract(schemas, nil, model.Response{Status: "200", Schema: model.SchemaRef{Name: "Node"}})
	if got := diff.Compare(a, b); len(got) != 0 {
		t.Errorf("identical recursive schemas reported %v", messages(got))
	}
}
