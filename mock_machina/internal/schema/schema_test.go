package schema_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/schema"
)

func set(t *testing.T, schemas map[string]string) *schema.Set {
	t.Helper()
	list := make([]*model.Schema, 0, len(schemas))
	for name, js := range schemas {
		list = append(list, &model.Schema{Name: name, JSON: []byte(js)})
	}
	s, probs := schema.New(list)
	if len(probs) > 0 {
		t.Fatalf("problems: %v", probs)
	}
	return s
}

const user = `{"type":"object","required":["id","name"],"additionalProperties":false,"properties":{
	"id":{"type":"string","pattern":"^u_"},"name":{"type":"string","minLength":1},
	"email":{"type":"string","format":"email"},"age":{"type":"integer","minimum":0,"maximum":150},
	"status":{"enum":["open","closed"]},"nick":{"type":["string","null"]},"tags":{"type":"array","minItems":1}}}`

func TestValidate(t *testing.T) {
	t.Parallel()

	s := set(t, map[string]string{
		"User":     user,
		"UserList": `{"type":"object","required":["users"],"properties":{"users":{"type":"array","items":{"$ref":"User"}}}}`,
	})
	tests := []struct {
		name, schema, body string
		want               []string
	}{
		{"valid", "UserList", `{"users":[{"id":"u_1","name":"Ada","nick":null}]}`, nil},
		{"missing field", "UserList", `{"users":[{"id":"u_1"}]}`, []string{`users[0]: missing required field "name"`}},
		{"missing at the root", "UserList", `{}`, []string{`body: missing required field "users"`}},
		{"wrong type", "User", `{"id":"u_1","name":"Ada","age":"forty"}`, []string{`age: should be an integer, but is a string`}},
		{"format", "UserList", `{"users":[{"id":"u_1","name":"A"},{"id":"u_2","name":"B","email":"amaka@"}]}`, []string{`users[1].email: "amaka@" isn't a valid email`}},
		{"pattern", "User", `{"id":"x1","name":"A"}`, []string{`id: "x1" doesn't match the pattern ^u_`}},
		{"min length", "User", `{"id":"u_1","name":""}`, []string{`name: has 0 characters; the schema wants at least 1`}},
		{"minimum", "User", `{"id":"u_1","name":"A","age":-1}`, []string{`age: -1 is less than the minimum, 0`}},
		{"maximum", "User", `{"id":"u_1","name":"A","age":151}`, []string{`age: 151 is more than the maximum, 150`}},
		{"enum", "User", `{"id":"u_1","name":"A","status":"pending"}`, []string{`status: "pending" isn't one of: "open", "closed"`}},
		{"extra field", "User", `{"id":"u_1","name":"A","shade":"red"}`, []string{`body: has a field the schema doesn't allow: "shade"`}},
		{"min items", "User", `{"id":"u_1","name":"A","tags":[]}`, []string{`tags: has 0 items; the schema wants at least 1`}},
		{"several, in order", "User", `{"id":"x","name":"","age":"x"}`, []string{
			`age: should be an integer, but is a string`,
			`id: "x" doesn't match the pattern ^u_`,
			`name: has 0 characters; the schema wants at least 1`,
		}},
		{"not JSON", "User", `oops`, []string{`body: isn't valid JSON`}},
	}
	for _, tc := range tests {
		sc, _ := s.Schema(tc.schema)
		if diff := cmp.Diff(tc.want, s.Validate(sc, []byte(tc.body))); diff != "" {
			t.Errorf("%s (-want +got):\n%s", tc.name, diff)
		}
	}
}

func TestInline(t *testing.T) {
	t.Parallel()

	s := set(t, map[string]string{"User": user})
	sc, err := s.Inline("routes/users.yaml:4", []byte(`{"type":"array","items":{"$ref":"User"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Validate(sc, []byte(`[{"id":"u_1"}]`)); !cmp.Equal(got, []string{`[0]: missing required field "name"`}) {
		t.Errorf("Validate = %v", got)
	}
}
