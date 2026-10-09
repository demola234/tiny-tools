package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

const userSchemas = `User:
  type: object
  required: [id, name]
  properties:
    id: { type: string, pattern: "^u_" }
    name: { type: string, minLength: 1 }
    email: { type: string, format: email }
UserList:
  type: object
  properties:
    users: { type: array, items: { $ref: User } }
    nextPage: { type: [integer, "null"] }
`

func TestLoadFS_Schemas(t *testing.T) {
	t.Parallel()

	p := loadClean(t, map[string]string{
		"schemas/users.yaml": userSchemas,
		"schemas/error.yaml": "Error:\n  type: object\n  properties: { error: { type: string } }\n",
	})
	names := make([]string, 0, len(p.Schemas))
	for _, s := range p.Schemas {
		names = append(names, s.Name+"@"+s.Src.File)
	}
	if diff := cmp.Diff([]string{"Error@schemas/error.yaml", "User@schemas/users.yaml", "UserList@schemas/users.yaml"}, names); diff != "" {
		t.Errorf("schemas (-want +got):\n%s", diff)
	}
	if len(p.Schemas) < 2 {
		t.FailNow()
	}
	if line := p.Schemas[1].Lines["/properties/email"]; line != 7 {
		t.Errorf("line of /properties/email = %d, want 7", line)
	}
}

func TestLoadFS_SchemaProblems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			"unknown reference",
			map[string]string{"schemas/users.yaml": "List:\n  type: array\n  items: { $ref: Usr }\nUser: { type: object }\n"},
			[]string{`schemas/users.yaml:3: $ref "Usr" isn't a schema (did you mean "User"?)`},
		},
		{
			"misspelled keyword",
			map[string]string{"schemas/users.yaml": "User:\n  type: object\n  properties:\n    name: { type: string, minLenght: 1 }\n"},
			[]string{`schemas/users.yaml:4: unknown schema keyword "minLenght" (did you mean "minLength"?)`},
		},
		{
			"property names aren't keywords",
			map[string]string{"schemas/users.yaml": "User:\n  type: object\n  properties:\n    minLenght: { type: string }\n    x-note: { type: string }\n  x-owner: ada\n"},
			nil,
		},
		{
			"invalid type",
			map[string]string{"schemas/users.yaml": "User:\n  type: object\n  properties:\n    age: { type: integr }\n"},
			[]string{`schemas/users.yaml:4: User isn't a valid schema at /properties/age/type: "integr" isn't a type (did you mean "integer"?)`},
		},
		{
			"invalid value",
			map[string]string{"schemas/a.yaml": "User:\n  type: string\n  minLength: -1\n"},
			[]string{`schemas/a.yaml:3: User isn't a valid schema at /minLength: -1 is less than the minimum, 0`},
		},
		{
			"defined twice",
			map[string]string{"schemas/a.yaml": "User: { type: object }\n", "schemas/b.yaml": "User: { type: string }\n"},
			[]string{`schemas/b.yaml:1: schema "User" is already defined in schemas/a.yaml:1`},
		},
		{
			"not a mapping of names",
			map[string]string{"schemas/a.yaml": "- type: object\n"},
			[]string{`schemas/a.yaml:1: a schemas file maps names to schemas, like User: { type: object }`},
		},
		{
			"bad name",
			map[string]string{"schemas/a.yaml": "user list: { type: object }\n"},
			[]string{`schemas/a.yaml:1: schema name "user list" must use letters, digits, dots, dashes and underscores`},
		},
		{
			"schema isn't a mapping",
			map[string]string{"schemas/a.yaml": "User: object\n"},
			[]string{`schemas/a.yaml:1: schema "User" must be a mapping, like { type: object }`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, probs := load(t, tc.files)
			var got []string
			for _, p := range probs {
				got = append(got, p.String())
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("problems (-want +got):\n%s", diff)
			}
		})
	}
}
