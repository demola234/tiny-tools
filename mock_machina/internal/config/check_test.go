package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

const okRoute = "get:\n  route: GET /r\n  states:\n    ok: {}\n"

func withField(field string) string {
	return "get:\n  route: GET /r\n  " + field + "\n  states:\n    ok: {}\n"
}

func withState(state string) string {
	return "get:\n  route: GET /r\n  states:\n    ok:\n      " + state + "\n"
}

func TestLoadFS_Problems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			"invalid YAML",
			map[string]string{"routes/r.yaml": "get: [r\n"},
			[]string{"routes/r.yaml:1: invalid YAML: did not find expected ',' or ']'"},
		},
		{
			"YAML error without a line",
			map[string]string{"routes/r.yaml": "get: \x01\n"},
			[]string{"routes/r.yaml: invalid YAML: control characters are not allowed"},
		},
		{
			"tab indentation",
			map[string]string{"routes/r.yaml": "get:\n\troute: GET /r\n"},
			[]string{"routes/r.yaml:2: invalid YAML: indent with spaces, not tabs"},
		},
		{
			"empty file",
			map[string]string{"routes/r.yaml": "\n"},
			[]string{"routes/r.yaml: file is empty"},
		},
		{
			"only file fields",
			map[string]string{"routes/r.yaml": "owners: { backend: [a] }\n"},
			[]string{"routes/r.yaml: no routes in this file"},
		},
		{
			"not a mapping",
			map[string]string{"routes/r.yaml": "- get\n"},
			[]string{"routes/r.yaml:1: expected routes by name, like list: { route: GET /users, states: { ok: {} } }"},
		},
		{
			".yml extension",
			map[string]string{"routes/users.yml": okRoute},
			[]string{"routes/users.yml: use the .yaml extension, so mockmachina reads this file"},
		},
		{
			"bad file name",
			map[string]string{"routes/User Data.yaml": okRoute},
			[]string{`routes/User Data.yaml: file name "User Data" must use lowercase letters, digits and dashes`},
		},
		{
			"bad route name",
			map[string]string{"routes/r.yaml": "List Users:\n  route: GET /r\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:1: route name "List Users" must use lowercase letters, digits and dashes`},
		},
		{
			"route name defined twice",
			map[string]string{"routes/r.yaml": okRoute + "get:\n  route: GET /s\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:5: "get" is defined twice (first on line 1)`},
		},
		{
			"route not a mapping",
			map[string]string{"routes/r.yaml": "get: GET /r\n"},
			[]string{`routes/r.yaml:1: route "get" must be a mapping of fields like route and states`},
		},
		{
			"unknown route field with suggestion",
			map[string]string{"routes/r.yaml": withField("rotue: GET /x")},
			[]string{`routes/r.yaml:3: unknown field "rotue" (did you mean "route"?)`},
		},
		{
			"unknown route field without suggestion",
			map[string]string{"routes/r.yaml": withField("extra: red")},
			[]string{`routes/r.yaml:3: unknown field "extra" (expected one of: route, examples, summary, status, owners, request, responses, active, mode, rules, serve, crud, generated, states)`},
		},
		{
			"field defined twice",
			map[string]string{"routes/r.yaml": withField("route: GET /s")},
			[]string{`routes/r.yaml:3: "route" is defined twice (first on line 2)`},
		},
		{
			"missing route and states",
			map[string]string{"routes/r.yaml": "get:\n  summary: R\n"},
			[]string{
				`routes/r.yaml:1: route "get" needs "route", like route: GET /users`,
				`routes/r.yaml:1: route "get" needs "states"`,
			},
		},
		{
			"example for a parameter the path doesn't have",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{id}\n  examples: { ids: u_1 }\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:3: examples has "ids", which isn't a parameter of GET /users/{id} (did you mean "id"?)`},
		},
		{
			"examples on a path without parameters",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users\n  examples: { id: u_1 }\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:3: examples has "id", which isn't a parameter of GET /users (it has none)`},
		},
		{
			"examples not a mapping",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{id}\n  examples: [u_1]\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:3: examples must map path parameters to values, like { id: u_1 }`},
		},
		{
			"example value not a single value",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{id}\n  examples: { id: [1, 2] }\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:3: examples.id must be a single value, like u_1`},
		},
		{
			"route without a path",
			map[string]string{"routes/r.yaml": "get:\n  route: GET\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: route "GET" must be a method and a path, like GET /users`},
		},
		{
			"lowercase method",
			map[string]string{"routes/r.yaml": "get:\n  route: get /r\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: method "get" must be uppercase: GET`},
		},
		{
			"unknown method",
			map[string]string{"routes/r.yaml": "get:\n  route: FETCH /r\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: method "FETCH" isn't an HTTP method (expected one of: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)`},
		},
		{
			"path without slash",
			map[string]string{"routes/r.yaml": "get:\n  route: GET users\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "users" must start with "/"`},
		},
		{
			"unclosed brace",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{id\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/users/{id" has an unclosed "{"`},
		},
		{
			"closing brace without opening",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/id}\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/users/id}" has a "}" without a "{"`},
		},
		{
			"parameter inside a segment",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{id}.json\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/users/{id}.json": a parameter must be a whole segment, like /users/{id}`},
		},
		{
			"empty parameter",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{}\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/users/{}" has a parameter with no name`},
		},
		{
			"bad parameter name",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users/{user-id}\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/users/{user-id}": parameter "user-id" must use letters, digits and underscores`},
		},
		{
			"repeated parameter",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /orgs/{id}/users/{id}\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/orgs/{id}/users/{id}" uses parameter "id" twice`},
		},
		{
			"query in path",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /users?page=1\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: path "/users?page=1" can't contain "?" or "#"; query parameters aren't part of the path`},
		},
		{
			"states is a list",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  states: [ok]\n"},
			[]string{`routes/r.yaml:3: "states" must map state names to their responses`},
		},
		{
			"no states",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  states: {}\n"},
			[]string{`routes/r.yaml:3: "states" needs at least one state`},
		},
		{
			"state is not a mapping",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  states:\n    ok: hello\n"},
			[]string{`routes/r.yaml:4: state "ok" must be a mapping of fields like status and body`},
		},
		{
			"invalid state name",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  states:\n    ok: {}\n    Server Error: {}\n"},
			[]string{`routes/r.yaml:5: state "Server Error" must use lowercase letters, digits and underscores, starting with a letter`},
		},
		{
			"state name is a YAML keyword",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  states:\n    ok: {}\n    no: {}\n"},
			[]string{`routes/r.yaml:5: state "no" is a YAML keyword; pick another name`},
		},
		{
			"active state with typo",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  active: empt\n  states:\n    success: {}\n    empty: {}\n"},
			[]string{`routes/r.yaml:3: active state "empt" doesn't exist (did you mean "empty"?)`},
		},
		{
			"active state unknown",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  active: nope\n  states:\n    success: {}\n    empty: {}\n"},
			[]string{`routes/r.yaml:3: active state "nope" doesn't exist (states: success, empty)`},
		},
		{
			"unknown state field",
			map[string]string{"routes/r.yaml": withState("bdoy: {}")},
			[]string{`routes/r.yaml:5: unknown field "bdoy" (did you mean "body"?)`},
		},
		{
			"HTTP status not a number",
			map[string]string{"routes/r.yaml": withState("status: ok")},
			[]string{`routes/r.yaml:5: status must be a number, like 200`},
		},
		{
			"HTTP status out of range",
			map[string]string{"routes/r.yaml": withState("status: 999")},
			[]string{`routes/r.yaml:5: status 999 isn't an HTTP status code (100-599)`},
		},
		{
			"serve misspelled",
			map[string]string{"routes/r.yaml": withField("serve: proxi")},
			[]string{`routes/r.yaml:3: serve "proxi" isn't valid (did you mean "proxy"?)`},
		},
		{
			"bad template in a body",
			map[string]string{"routes/r.yaml": withState(`body: { id: "{{ nope }}" }`)},
			[]string{`routes/r.yaml:5: body: unknown template "nope" (did you mean "now"?)`},
		},
		{
			"bad template in a header",
			map[string]string{"routes/r.yaml": withState(`headers: { X-Id: "{{ path.id" }`)},
			[]string{`routes/r.yaml:5: header X-Id: template "{{ path.id" has no closing }}`},
		},
		{
			"bad template in a text body file",
			map[string]string{"routes/r.yaml": withState("body: hello.txt"), "routes/r/hello.txt": "hi {{ nme }}"},
			[]string{`routes/r.yaml:5: body hello.txt: unknown template "nme" (did you mean "now"?)`},
		},
		{
			"template isn't true or false",
			map[string]string{"routes/r.yaml": withState("template: no")},
			[]string{`routes/r.yaml:5: template must be true or false`},
		},
		{
			"route generated isn't true or false",
			map[string]string{"routes/r.yaml": withField("generated: yes")},
			[]string{`routes/r.yaml:3: generated must be true or false`},
		},
		{
			"state generated isn't true or false",
			map[string]string{"routes/r.yaml": withState("generated: [true]")},
			[]string{`routes/r.yaml:5: generated must be true or false`},
		},
		{
			"jitter over the limit",
			map[string]string{"routes/r.yaml": withState("latency: { base: 50s, jitter: 20s }")},
			[]string{`routes/r.yaml:5: latency 50s ± 20s can reach 1m10s, over the 1m limit`},
		},
		{
			"jitter not a duration",
			map[string]string{"routes/r.yaml": withState("latency: { base: 1s, jitter: lots }")},
			[]string{`routes/r.yaml:5: latency jitter "lots" isn't a duration, like 250ms or 2s`},
		},
		{
			"latency mapping without base",
			map[string]string{"routes/r.yaml": withState("latency: { jitter: 1s }")},
			[]string{`routes/r.yaml:5: latency needs a base, like { base: 2s, jitter: 500ms }`},
		},
		{
			"latency mapping with an unknown field",
			map[string]string{"routes/r.yaml": withState("latency: { base: 1s, jiter: 1s }")},
			[]string{`routes/r.yaml:5: unknown field "jiter" (did you mean "jitter"?)`},
		},
		{
			"crud on another method",
			map[string]string{"routes/r.yaml": withField("crud: { collection: things }")},
			[]string{`routes/r.yaml:3: crud only applies to CRUD routes, like route: CRUD /things`},
		},
		{
			"crud path ends with a parameter",
			map[string]string{"routes/r.yaml": "get:\n  route: CRUD /things/{id}\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:2: a CRUD route's path names the collection, like CRUD /things; it adds /{id} itself`},
		},
		{
			"crud data isn't a list",
			map[string]string{"routes/r.yaml": "get:\n  route: CRUD /things\n  states:\n    ok: {}\n", "data/things.json": `{"id":1}`},
			[]string{`data/things.json: must be a JSON list of objects, like [{"id": "t_1"}]`},
		},
		{
			"crud item without an id",
			map[string]string{"routes/r.yaml": "get:\n  route: CRUD /things\n  states:\n    ok: {}\n", "data/things.json": `[{"id":"a"},{"name":"b"}]`},
			[]string{`data/things.json: item 2 has no "id"`},
		},
		{
			"crud bad collection name",
			map[string]string{"routes/r.yaml": "get:\n  route: CRUD /things\n  crud: { collection: ../x }\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:3: crud collection "../x" must use lowercase letters, digits, dashes and underscores`},
		},
		{
			"set not a mapping",
			map[string]string{"routes/r.yaml": withState("set: signed_in")},
			[]string{`routes/r.yaml:5: set must map variable names to values, like { signed_in: true }`},
		},
		{
			"set with a bad name",
			map[string]string{"routes/r.yaml": withState("set: { Signed-In: true }")},
			[]string{`routes/r.yaml:5: variable "Signed-In" must use lowercase letters, digits and underscores, starting with a letter`},
		},
		{
			"set with a list",
			map[string]string{"routes/r.yaml": withState("set: { tags: [a] }")},
			[]string{`routes/r.yaml:5: set.tags must be a single value, like true, 3 or "{{ body.email }}"`},
		},
		{
			"set with a bad template",
			map[string]string{"routes/r.yaml": withState(`set: { user: "{{ bdy.email }}" }`)},
			[]string{`routes/r.yaml:5: set.user: unknown template "bdy.email" (did you mean "body.email"?)`},
		},
		{
			"unknown fault",
			map[string]string{"routes/r.yaml": withState("fault: rest")},
			[]string{`routes/r.yaml:5: fault "rest" isn't valid (did you mean "reset"?)`},
		},
		{
			"fault rate out of range",
			map[string]string{"routes/r.yaml": withState("fault: { type: reset, rate: 1.5 }")},
			[]string{`routes/r.yaml:5: fault rate must be more than 0 and at most 1, like 0.2`},
		},
		{
			"fault rate zero",
			map[string]string{"routes/r.yaml": withState("fault: { type: reset, rate: 0 }")},
			[]string{`routes/r.yaml:5: fault rate must be more than 0 and at most 1, like 0.2`},
		},
		{
			"fault without a type",
			map[string]string{"routes/r.yaml": withState("fault: { rate: 0.5 }")},
			[]string{`routes/r.yaml:5: fault needs a type: timeout, reset or truncated`},
		},
		{
			"fault after not a duration",
			map[string]string{"routes/r.yaml": withState("fault: { type: reset, after: soon }")},
			[]string{`routes/r.yaml:5: fault after "soon" isn't a duration, like 250ms or 2s`},
		},
		{
			"latency without a unit",
			map[string]string{"routes/r.yaml": withState("latency: 800")},
			[]string{`routes/r.yaml:5: latency "800" has no unit; write 800ms`},
		},
		{
			"negative latency",
			map[string]string{"routes/r.yaml": withState("latency: -1s")},
			[]string{`routes/r.yaml:5: latency "-1s" can't be negative`},
		},
		{
			"latency not a duration",
			map[string]string{"routes/r.yaml": withState("latency: fast")},
			[]string{`routes/r.yaml:5: latency "fast" isn't a duration, like 250ms or 2s`},
		},
		{
			"latency over the limit",
			map[string]string{"routes/r.yaml": withState("latency: 2m")},
			[]string{`routes/r.yaml:5: latency "2m" is over the 1m limit`},
		},
		{
			"headers not a mapping",
			map[string]string{"routes/r.yaml": withState("headers: [a]")},
			[]string{`routes/r.yaml:5: headers must map names to values, like { Cache-Control: no-store }`},
		},
		{
			"body file missing",
			map[string]string{"routes/r.yaml": withState("body: slow.json")},
			[]string{`routes/r.yaml:5: body file "slow.json" not found in routes/r`},
		},
		{
			"body outside project",
			map[string]string{"routes/r.yaml": withState("body: ../../../secrets.json")},
			[]string{`routes/r.yaml:5: body "../../../secrets.json" points outside the project`},
		},
		{
			"inline body that isn't JSON",
			map[string]string{"routes/r.yaml": withState("body: { n: .inf }")},
			[]string{`routes/r.yaml:5: body can't be written as JSON: infinity and NaN aren't JSON numbers`},
		},
		{
			"infinity inside a list",
			map[string]string{"routes/r.yaml": withState("body: { a: [1, .inf] }")},
			[]string{`routes/r.yaml:5: body can't be written as JSON: infinity and NaN aren't JSON numbers`},
		},
		{
			"undecodable inline value",
			map[string]string{"routes/r.yaml": withState("body: { n: !!int abc }")},
			[]string{"routes/r.yaml:5: body can't be written as JSON: yaml: cannot decode !!str `abc` as a !!int"},
		},
		{
			"route status typo",
			map[string]string{"routes/r.yaml": withField("status: agrred")},
			[]string{`routes/r.yaml:3: status "agrred" isn't valid (did you mean "agreed"?)`},
		},
		{
			"route status unknown",
			map[string]string{"routes/r.yaml": withField("status: approved")},
			[]string{`routes/r.yaml:3: status "approved" isn't valid (expected one of: draft, agreed, implemented, deprecated)`},
		},
		{
			"HTTP status at route level",
			map[string]string{"routes/r.yaml": withField("status: 200")},
			[]string{`routes/r.yaml:3: status "200" isn't valid here: HTTP status codes go inside a state (route status is one of: draft, agreed, implemented, deprecated)`},
		},
		{
			"file status unknown",
			map[string]string{"routes/r.yaml": "status: approved\n" + okRoute},
			[]string{`routes/r.yaml:1: status "approved" isn't valid (expected one of: draft, agreed, implemented, deprecated)`},
		},
		{
			"summary not text",
			map[string]string{"routes/r.yaml": withField("summary: [a, b]")},
			[]string{`routes/r.yaml:3: summary must be one line of text`},
		},
		{
			"summary over several lines",
			map[string]string{"routes/r.yaml": "get:\n  route: GET /r\n  summary: |\n    one\n    two\n  states:\n    ok: {}\n"},
			[]string{`routes/r.yaml:3: summary must be one line of text`},
		},
		{
			"owners not a mapping",
			map[string]string{"routes/r.yaml": withField("owners: [ademola]")},
			[]string{`routes/r.yaml:3: owners must map backend and frontend to lists of GitHub handles`},
		},
		{
			"owners field typo",
			map[string]string{"routes/r.yaml": "owners:\n  backnd: [ademola]\n" + okRoute},
			[]string{`routes/r.yaml:2: unknown field "backnd" (did you mean "backend"?)`},
		},
		{
			"owners handle not a list",
			map[string]string{"routes/r.yaml": "owners:\n  backend: ademola\n" + okRoute},
			[]string{`routes/r.yaml:2: owners.backend must be a list of GitHub handles, like [ademola]`},
		},
		{
			"extension where none is allowed",
			map[string]string{"routes/r.yaml": "owners:\n  x-note: hi\n" + okRoute},
			[]string{`routes/r.yaml:2: unknown field "x-note" (expected one of: backend, frontend)`},
		},
		{
			"duplicate method and path across files",
			map[string]string{
				"routes/a.yaml": "get:\n  route: GET /users/{id}\n  states:\n    ok: {}\n",
				"routes/b.yaml": "one:\n  route: GET /users/{userId}\n  states:\n    ok: {}\n",
			},
			[]string{`routes/b.yaml:1: GET /users/{} is already defined by route "a.get"`},
		},
		{
			"every problem reported, sorted by file and line",
			map[string]string{
				"routes/b.yaml": "get:\n  route: get /b\n  active: nope\n  states:\n    ok: {}\n",
				"routes/a.yaml": "get:\n  route: GET a\n  states:\n    ok:\n      status: 9\n",
			},
			[]string{
				`routes/a.yaml:2: path "a" must start with "/"`,
				`routes/a.yaml:5: status 9 isn't an HTTP status code (100-599)`,
				`routes/b.yaml:2: method "get" must be uppercase: GET`,
				`routes/b.yaml:3: active state "nope" doesn't exist (states: ok)`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, tc.files)
			got := make([]string, len(probs))
			for i, p := range probs {
				got[i] = p.String()
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("problems (-want +got):\n%s", diff)
			}
		})
	}
}
