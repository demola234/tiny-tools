package schema_test

import (
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/schema"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	schemas := []*model.Schema{
		{Name: "User", JSON: []byte(`{"type":"object","required":["id","email","age","role","joined","tags"],"properties":{
			"id":{"type":"string","pattern":"^u_"},"email":{"type":"string","format":"email"},
			"age":{"type":"integer","minimum":18,"maximum":120},"role":{"enum":["admin","member"]},
			"joined":{"type":"string","format":"date-time"},"tags":{"type":"array","minItems":2,"items":{"type":"string","minLength":3}},
			"nick":{"type":"string"}}}`)},
		{Name: "Page", JSON: []byte(`{"type":"object","required":["users","next"],"properties":{"users":{"type":"array","items":{"$ref":"User"}},"next":{"type":["integer","null"],"exclusiveMinimum":0}}}`)},
		{Name: "Pet", JSON: []byte(`{"type":"object","required":["name"],"properties":{"name":{"type":"string","example":"Rex"}},"example":{"name":"Fido","kind":"dog"}}`)},
		{Name: "Node", JSON: []byte(`{"type":"object","required":["child"],"properties":{"child":{"$ref":"Node"}}}`)},
		{Name: "Tree", JSON: []byte(`{"type":"object","required":["name","children"],"properties":{"name":{"type":"string"},"children":{"type":"array","items":{"$ref":"Tree"}}}}`)},
		{Name: "Merged", JSON: []byte(`{"allOf":[{"type":"object","required":["a"],"properties":{"a":{"const":1}}},{"type":"object","required":["b"],"properties":{"b":{"type":"boolean"}}}]}`)},
		{Name: "Either", JSON: []byte(`{"oneOf":[{"type":"string","format":"uuid"},{"type":"integer"}]}`)},
	}
	tests := []struct {
		name string
		ref  model.SchemaRef
		want string
	}{
		{"object with formats", model.SchemaRef{Name: "User"}, `{"id":"u_1","email":"user@example.com","age":18,"role":"admin","joined":"2026-01-01T00:00:00Z","tags":["xxx","xxx"]}`},
		{"references and nullable", model.SchemaRef{Name: "Page"}, `{"users":[{"id":"u_1","email":"user@example.com","age":18,"role":"admin","joined":"2026-01-01T00:00:00Z","tags":["xxx","xxx"]}],"next":1}`},
		{"example wins", model.SchemaRef{Name: "Pet"}, `{"name":"Fido","kind":"dog"}`},
		{"recursion stops", model.SchemaRef{Name: "Node"}, `{"child":{"child":{"child":{"child":{"child":{"child":{}}}}}}}`},
		{"recursive lists end empty", model.SchemaRef{Name: "Tree"}, `{"name":"string","children":[{"name":"string","children":[{"name":"string","children":[]}]}]}`},
		{"allOf merges", model.SchemaRef{Name: "Merged"}, `{"a":1,"b":true}`},
		{"oneOf takes the first", model.SchemaRef{Name: "Either"}, `"00000000-0000-4000-8000-000000000000"`},
		{"inline", model.SchemaRef{Inline: &model.Schema{JSON: []byte(`{"type":"array","items":{"$ref":"Pet"}}`)}}, `[{"name":"Fido","kind":"dog"}]`},
	}
	for _, tc := range tests {
		got, err := schema.Generate(schemas, tc.ref)
		if err != nil || string(got) != tc.want {
			t.Errorf("%s: %s, %v\nwant %s", tc.name, got, err, tc.want)
		}
	}
	a, _ := schema.Generate(schemas, model.SchemaRef{Name: "Page"})
	b, _ := schema.Generate(schemas, model.SchemaRef{Name: "Page"})
	if string(a) != string(b) {
		t.Error("generation isn't the same every time")
	}
}
