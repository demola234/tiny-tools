package server_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/server"
)

func inline(js string) model.SchemaRef {
	return model.SchemaRef{Inline: &model.Schema{Name: "inline", JSON: []byte(js), Src: model.Source{File: "routes/r.yaml", Line: len(js)}}}
}

func validatedProject() *model.Project {
	return &model.Project{
		Schemas: []*model.Schema{{Name: "Login", JSON: []byte(`{"type":"object","required":["email","password"],"properties":{"email":{"type":"string","format":"email"},"password":{"type":"string","minLength":8}}}`)}},
		Routes: []*model.Route{
			{
				ID: "orgs.members", Method: model.MethodGet, Path: "/orgs/{org}/members", Active: "ok",
				Request: &model.Request{
					Params:  []model.Param{{Name: "org", Required: true, Schema: inline(`{"type":"string","pattern":"^o_"}`)}},
					Query:   []model.Param{{Name: "page", Schema: inline(`{"type":"integer","minimum":1}`)}, {Name: "active", Schema: inline(`{"type":"boolean"}`)}},
					Headers: []model.Param{{Name: "Authorization", Required: true, Schema: inline(`{"type":"string","pattern":"^Bearer "}`)}},
				},
				States: model.States{{Name: "ok", State: &model.State{Body: jsonBody(`{"members":[]}`)}}},
			},
			{
				ID: "session.create", Method: model.MethodPost, Path: "/session", Active: "ok",
				Request: &model.Request{Body: &model.SchemaRef{Name: "Login"}},
				States: model.States{
					{Name: "ok", State: &model.State{Status: 201}},
					{Name: "invalid", State: &model.State{Status: 422, SkipRequestValidation: true}},
				},
			},
		},
	}
}

func TestRequestValidation(t *testing.T) {
	t.Parallel()

	var got []server.Request
	h, err := server.New(validatedProject(), server.Options{Report: func(r server.Request) { got = append(got, r) }})
	if err != nil {
		t.Fatal(err)
	}
	auth := map[string]string{"Authorization": "Bearer t"}
	tests := []struct {
		name, method, target, body string
		header                     map[string]string
		code                       int
		want                       string
	}{
		{"valid", "GET", "/orgs/o_1/members?page=2&active=true", "", auth, 200, `{"members":[]}`},
		{"extra query allowed", "GET", "/orgs/o_1/members?other=x&__state=ok", "", auth, 200, `{"members":[]}`},
		{
			"bad path param", "GET", "/orgs/x/members", "", auth, 400,
			`{"error":"invalid_request","route":"orgs.members","problems":[{"at":"params.org","message":"\"x\" doesn't match the pattern ^o_"}]}`,
		},
		{
			"query text that isn't a number", "GET", "/orgs/o_1/members?page=zero", "", auth, 400,
			`{"error":"invalid_request","route":"orgs.members","problems":[{"at":"query.page","message":"should be an integer, but is a string"}]}`,
		},
		{
			"query below minimum", "GET", "/orgs/o_1/members?page=0", "", auth, 400,
			`{"error":"invalid_request","route":"orgs.members","problems":[{"at":"query.page","message":"0 is less than the minimum, 1"}]}`,
		},
		{
			"missing header", "GET", "/orgs/o_1/members", "", nil, 400,
			`{"error":"invalid_request","route":"orgs.members","problems":[{"at":"headers.Authorization","message":"is required"}]}`,
		},
		{
			"body", "POST", "/session", `{"email":"ada@example.com"}`, nil, 400,
			`{"error":"invalid_request","route":"session.create","problems":[{"at":"body","message":"missing required field \"password\""}]}`,
		},
		{
			"several problems", "POST", "/session", `{"email":"nope","password":"short"}`, nil, 400,
			`{"error":"invalid_request","route":"session.create","problems":[{"at":"body.email","message":"\"nope\" isn't a valid email"},{"at":"body.password","message":"has 5 characters; the schema wants at least 8"}]}`,
		},
		{
			"body that isn't JSON", "POST", "/session", `email=x`, nil, 400,
			`{"error":"invalid_request","route":"session.create","problems":[{"at":"body","message":"isn't valid JSON"}]}`,
		},
		{
			"no body", "POST", "/session", "", nil, 400,
			`{"error":"invalid_request","route":"session.create","problems":[{"at":"body","message":"is required"}]}`,
		},
		{"valid body", "POST", "/session", `{"email":"ada@example.com","password":"long enough"}`, nil, 201, ""},
		{"state that skips validation", "POST", "/session", `{}`, map[string]string{"X-Mock-State": "invalid"}, 422, ""},
	}
	for _, tc := range tests {
		req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, strings.NewReader(tc.body))
		for k, v := range tc.header {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.code || strings.TrimSpace(rec.Body.String()) != tc.want {
			t.Errorf("%s: %d %s\nwant %d %s", tc.name, rec.Code, rec.Body, tc.code, tc.want)
		}
	}
	bad := got[2]
	if bad.Problem != server.ProblemInvalidRequest || bad.Detail != `params.org: "x" doesn't match the pattern ^o_` {
		t.Errorf("reported %+v", bad)
	}
}

func TestRequestValidation_CanBeTurnedOff(t *testing.T) {
	t.Parallel()

	h, err := server.New(validatedProject(), server.Options{NoRequestValidation: true})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, "GET", "/orgs/x/members", nil); rec.Code != 200 {
		t.Errorf("= %d, want 200 with validation off", rec.Code)
	}
}
