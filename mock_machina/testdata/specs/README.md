# Spec corpus

Specs that import, lint and export are tested against. Each records where it came from and its licence.

| File | Source | Licence |
| --- | --- | --- |
| `petstore-3.0.yaml` | [OAI/learn.openapis.org](https://github.com/OAI/learn.openapis.org/blob/main/examples/v3.0/petstore.yaml) | Apache-2.0 |
| `petstore-expanded-3.0.yaml` | [OAI/learn.openapis.org](https://github.com/OAI/learn.openapis.org/blob/main/examples/v3.0/petstore-expanded.yaml) | Apache-2.0 |
| `api-with-examples-3.0.yaml` | [OAI/learn.openapis.org](https://github.com/OAI/learn.openapis.org/blob/main/examples/v3.0/api-with-examples.yaml) | Apache-2.0 |
| `petstore-2.0.yaml` | [OAI/learn.openapis.org](https://github.com/OAI/learn.openapis.org/blob/main/examples/v2.0/yaml/petstore.yaml) | Apache-2.0 |
| `swagger-petstore-3.0.yaml` | [swagger-api/swagger-petstore](https://github.com/swagger-api/swagger-petstore/blob/master/src/main/resources/openapi.yaml) | Apache-2.0 |
| `shop.postman_collection.json` | Written for MockMachina: folders, saved examples, `:id` and `{{userId}}` path variables, a non-JSON example | Same as this repository |
| `oas/schema-3.1.json` | [OpenAPI 3.1 schema, 2022-10-07](https://spec.openapis.org/oas/3.1/schema/2022-10-07): exports are validated against it in tests | Apache-2.0 |
| `awkward-3.1.yaml` | Written for MockMachina: nullable unions, `allOf`, `oneOf`, recursion, shared parameters, request bodies and responses, several examples, `default`, 204, an operation without an `operationId` | Same as this repository |

Downloaded 2026-10-08. To refresh one, download it again from its source and rerun the tests.
