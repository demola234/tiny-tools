package openapi_test

import (
	"strings"
	"testing"

	"github.com/demola234/tiny-tools/mock_machina/internal/model"
	"github.com/demola234/tiny-tools/mock_machina/internal/openapi"
)

const twoSchemas = `openapi: 3.1.0
info: { title: Pets, version: "1" }
paths:
  /pets:
    post:
      requestBody:
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Pet" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Pets" }
        "400":
          description: bad
          content:
            application/json:
              schema: { type: object, properties: { pet: { $ref: "#/components/schemas/Pet" } } }
components:
  schemas:
    Pet: { type: object, description: 'mentions "$ref":"Pet" in text', properties: { name: { type: string } } }
    Pets: { type: array, items: { $ref: "#/components/schemas/Pet" } }
`

func TestRenameSchemas(t *testing.T) {
	t.Parallel()

	res, err := openapi.Import([]byte(twoSchemas))
	if err != nil {
		t.Fatal(err)
	}
	res.RenameSchemas(map[string]string{"Pet": "ShopPet"})

	names := map[string]string{}
	for _, s := range res.Schemas {
		names[s.Name] = string(s.JSON)
	}
	if _, ok := names["Pet"]; ok || names["ShopPet"] == "" {
		t.Fatalf("schemas after rename: %v", names)
	}
	if !strings.Contains(names["Pets"], `"$ref":"ShopPet"`) {
		t.Errorf("Pets still points at the old name: %s", names["Pets"])
	}
	if !strings.Contains(names["ShopPet"], `mentions \"$ref\":\"Pet\" in text`) {
		t.Errorf("text that looks like a ref was changed: %s", names["ShopPet"])
	}
	checkRenamedRoute(t, res.Routes[0])
}

func checkRenamedRoute(t *testing.T, r *model.Route) {
	t.Helper()
	if r.Request == nil || r.Request.Body == nil || r.Request.Body.Name != "ShopPet" {
		t.Errorf("request body ref = %+v", r.Request)
	}
	for _, resp := range r.Responses {
		switch resp.Status {
		case "400":
			if resp.Schema.Inline == nil || !strings.Contains(string(resp.Schema.Inline.JSON), `"$ref":"ShopPet"`) {
				t.Errorf("inline 400 schema still points at Pet: %+v", resp.Schema)
			}
		case "200":
			if resp.Schema.Name != "Pets" {
				t.Errorf("200 should keep Pets: %+v", resp.Schema)
			}
		}
	}
}
