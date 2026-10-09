package config_test

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
)

const contractRoutes = `list:
  route: GET /users/{org}/members
  request:
    params: { org: { type: string, pattern: "^o_" } }
    query:
      page: { type: integer, minimum: 1 }
      q: { type: string, required: true }
    headers: { Authorization: { type: string, required: true } }
  responses:
    200: UserList
    401: { type: object, properties: { error: { type: string } } }
  states:
    success: { body: { users: [{ id: u_1, name: Ada }] } }
    unauthorized: { status: 401, body: { error: expired } }
create:
  route: POST /users
  request:
    body: User
  responses:
    201: User
    default: Error
  states:
    created: { status: 201, body: { id: u_2, name: Tunde } }
    teapot: { status: 418, body: { error: short and stout } }
`

func contractFiles(routes string) map[string]string {
	return map[string]string{
		"routes/users.yaml":  routes,
		"schemas/users.yaml": userSchemas,
		"schemas/error.yaml": "Error:\n  type: object\n  required: [error]\n  properties: { error: { type: string } }\n",
	}
}

func TestLoadFS_RequestAndResponses(t *testing.T) {
	t.Parallel()

	p := loadClean(t, contractFiles(contractRoutes))
	list, _ := p.Route("users.list")
	if list.Request == nil || len(list.Request.Params) != 1 || !list.Request.Params[0].Required {
		t.Fatalf("params = %+v", list.Request)
	}
	query := map[string]bool{}
	for _, q := range list.Request.Query {
		query[q.Name] = q.Required
	}
	if diff := cmp.Diff(map[string]bool{"page": false, "q": true}, query); diff != "" {
		t.Errorf("query required (-want +got):\n%s", diff)
	}
	if h := list.Request.Headers; len(h) != 1 || h[0].Name != "Authorization" || !h[0].Required {
		t.Errorf("headers = %+v", h)
	}
	if string(list.Request.Query[1].Schema.Inline.JSON) != `{"type":"string"}` {
		t.Errorf("required: true wasn't taken out of the schema: %s", list.Request.Query[1].Schema.Inline.JSON)
	}
	statuses := make([]string, 0, len(list.Responses))
	for _, r := range list.Responses {
		statuses = append(statuses, r.Status)
	}
	if diff := cmp.Diff([]string{"200", "401"}, statuses); diff != "" || list.Responses[0].Schema.Name != "UserList" || list.Responses[1].Schema.Inline == nil {
		t.Errorf("responses = %+v", list.Responses)
	}
	create, _ := p.Route("users.create")
	if create.Request.Body == nil || create.Request.Body.Name != "User" {
		t.Errorf("request body = %+v", create.Request)
	}
}

func TestLoadFS_ContractProblems(t *testing.T) {
	t.Parallel()

	route := func(fields, states string) string {
		return "get:\n  route: GET /users/{id}\n" + fields + "  states:\n" + states
	}
	tests := []struct {
		name, routes string
		want         []string
	}{
		{
			"unknown schema name",
			route("  responses: { 200: Usr }\n", "    ok: {}\n"),
			[]string{`routes/users.yaml:3: responses.200: "Usr" isn't a schema (did you mean "User"?)`},
		},
		{
			"bad status",
			route("  responses: { 99: User }\n", "    ok: {}\n"),
			[]string{`routes/users.yaml:3: responses.99 isn't an HTTP status (100-599) or default`},
		},
		{
			"bad inline keyword",
			route("  responses: { 200: { type: object, propertis: {} } }\n", "    ok: {}\n"),
			[]string{`routes/users.yaml:3: unknown schema keyword "propertis" (did you mean "properties"?)`},
		},
		{
			"param that isn't in the path",
			route("  request: { params: { ids: { type: string } } }\n", "    ok: {}\n"),
			[]string{`routes/users.yaml:3: request.params.ids isn't a parameter of GET /users/{id} (did you mean "id"?)`},
		},
		{
			"unknown request field",
			route("  request: { qeury: {} }\n", "    ok: {}\n"),
			[]string{`routes/users.yaml:3: unknown field "qeury" (did you mean "query"?)`},
		},
		{
			"body doesn't match",
			route("  responses: { 200: User }\n", "    ok: { body: { id: x1, name: Ada, email: \"amaka@\" } }\n"),
			[]string{
				`routes/users.yaml:5: state "ok" doesn't match User: email: "amaka@" isn't a valid email`,
				`routes/users.yaml:5: state "ok" doesn't match User: id: "x1" doesn't match the pattern ^u_`,
			},
		},
		{
			"body file doesn't match",
			route("  responses: { 200: UserList }\n", "    ok: { body: list.json }\n"),
			[]string{`routes/users.yaml:5: state "ok" doesn't match UserList: users[0]: missing required field "name"`},
		},
		{
			"templates are skipped",
			route("  responses: { 200: User }\n", "    ok: { body: { id: \"{{ path.id }}\", name: \"{{ fake.person.name }}\" } }\n"),
			nil,
		},
		{
			"undocumented status",
			route("  responses: { 200: User }\n", "    ok: { body: { id: u_1, name: Ada } }\n    teapot: { status: 418 }\n"),
			[]string{`routes/users.yaml:6: state "teapot" returns 418, which users.get doesn't document (add it to responses, or a default)`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := contractFiles(tc.routes)
			files["routes/users/list.json"] = `{"users":[{"id":"u_1"}]}`
			_, probs := load(t, files)
			var got []string
			for _, p := range probs {
				if p.Severity == config.Error {
					got = append(got, p.String())
				}
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("problems (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWarnings_DocumentedStatusWithoutAState(t *testing.T) {
	t.Parallel()

	p, _ := load(t, contractFiles("get:\n  route: GET /users/{id}\n  responses: { 200: User, 404: Error }\n  states:\n    ok: { body: { id: u_1, name: Ada } }\n"))
	warnings := config.Warnings(p)
	got := make([]string, 0, len(warnings))
	for _, w := range warnings {
		got = append(got, w.String())
	}
	want := "routes/users.yaml:1: warning: users.get documents 404 but no state returns it"
	if !slices.Contains(got, want) {
		t.Errorf("warnings %q\nwant one to be %q", got, want)
	}
}

func TestLoadFS_GeneratedBodies(t *testing.T) {
	t.Parallel()

	p := loadClean(t, contractFiles("get:\n  route: GET /users/{id}\n  responses: { 200: User, 404: Error }\n  states:\n    found: { body: generate }\n    missing: { status: 404, body: generate }\n"))
	r, _ := p.Route("users.get")
	found, _ := r.States.Get("found")
	missing, _ := r.States.Get("missing")
	if string(found.Body.Data) != `{"id":"u_1","name":"x"}` || found.Body.ContentType != "application/json" {
		t.Errorf("found body = %s (%s)", found.Body.Data, found.Body.ContentType)
	}
	if string(missing.Body.Data) != `{"error":"string"}` {
		t.Errorf("missing body = %s", missing.Body.Data)
	}
}

func TestLoadFS_GenerateNeedsASchema(t *testing.T) {
	t.Parallel()

	_, probs := load(t, map[string]string{"routes/users.yaml": "get:\n  route: GET /users\n  states:\n    ok: { body: generate }\n"})
	want := `routes/users.yaml:4: state "ok" has body: generate, but users.get has no response schema for 200`
	if got := probs.String(); got != want {
		t.Errorf("problems:\n%s\nwant:\n%s", got, want)
	}
}

func TestLoadFS_GeneratedBodyThatDoesntMatch(t *testing.T) {
	t.Parallel()

	_, probs := load(t, map[string]string{
		"schemas/s.yaml":    "Slug: { type: string, pattern: \"^[a-z]+$\" }\n",
		"routes/users.yaml": "get:\n  route: GET /users\n  responses: { 200: Slug }\n  states:\n    ok: { body: generate }\n",
	})
	want := `routes/users.yaml:5: state "ok": the body generated from Slug doesn't match it: body: "1" doesn't match the pattern ^[a-z]+$ (add an example to the schema)`
	if got := probs.String(); got != want {
		t.Errorf("problems:\n%s\nwant:\n%s", got, want)
	}
}
