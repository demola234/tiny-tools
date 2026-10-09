# Schemas and contracts

A route's states say what the mock returns. Schemas say what the API *promises*. With schemas in place:
- `lint` checks every state's body against them;
- the mock rejects requests that don't match;
- `diff` catches field-level breaking changes;
- states can have their bodies generated.

## Schemas

Put [JSON Schemas](https://json-schema.org/understanding-json-schema) (2020-12, the version OpenAPI 3.1 uses) in `.mockmachina/schemas/*.yaml`, each file mapping names to schemas:

```yaml
# .mockmachina/schemas/users.yaml
User:
  type: object
  required: [id, name]
  properties:
    id: { type: string, pattern: "^u_" }
    name: { type: string, minLength: 1 }
    email: { type: string, format: email }
UserList:
  type: object
  required: [users]
  properties:
    users: { type: array, items: { $ref: User } }
    nextPage: { type: [integer, "null"] }
```

- **Names are unique across all schema files.** Schemas refer to each other by name (`$ref: User`), and so do routes.
- **Formats are enforced:** `email`, `date-time`, `uuid` and the rest. A mock that returns `"email": "not-an-email"` would hide app bugs.
- **Mistakes are reported** with the file and line: an unknown `$ref`, a misspelled keyword (`minLenght`), an invalid type.

## Responses

```yaml
list:
  route: GET /users
  responses:
    200: UserList
    401: { type: object, properties: { error: { type: string } } }
    default: Error
  states:
    success: { body: users.json }
    unauthorized: { status: 401, body: { error: session expired } }
```

Each status maps to a schema name, or a schema written inline. `default` covers any status not listed.

`lint` then checks every state:

```text
routes/users.yaml:7: state "success" doesn't match UserList: users[1].email: "amaka@" isn't a valid email
routes/users.yaml:9: state "teapot" returns 418, which users.list doesn't document (add it to responses, or a default)
warning: users.list documents 404 but no state returns it
```

Values that are [templates](file-format.md#templates), like `"{{ path.id }}"`, aren't checked; they're only known when a request arrives.

## Requests

```yaml
create:
  route: POST /users/{org}
  request:
    params: { org: { type: string, pattern: "^o_" } }
    query: { dryRun: { type: boolean } }
    headers: { Authorization: { type: string, pattern: "^Bearer ", required: true } }
    body: NewUser
```

`params`, `query` and `headers` map names to schemas. Add `required: true` to a query or header schema; path parameters are always required. Query and header text is converted to numbers and booleans where the schema says so, so `?page=2` checks as an integer.

A request that doesn't match gets a `400`, and the state isn't served:

```json
{"error":"invalid_request","route":"users.create","problems":[{"at":"body.email","message":"\"amaka@\" isn't a valid email"}]}
```

The log line names the first problem. Extra fields are allowed unless the schema says `additionalProperties: false`.

| To turn it off | Write |
| --- | --- |
| For one state, such as one that tests the app's handling of a server-side validation error | `validateRequest: false` on the state |
| For the whole server | `mockmachina start --no-request-validation` |

## Generated bodies

`body: generate` builds the body from the state's response schema:

```yaml
get:
  route: GET /users/{id}
  responses: { 200: User }
  states:
    found: { body: generate }
```

| The schema says | The generated value |
| --- | --- |
| `example` or `examples` | That example |
| `const` / `enum` | The constant / the first value |
| `format: email`, `date-time`, `uuid`, `uri`… | A fixed valid value |
| `pattern` | The pattern's literal start, plus `1` |
| A number | `minimum`, or 1, within `maximum` |
| A list | `minItems` items, at least one |
| An object | Its required fields, in the order the schema lists them |

The body is the same every run, and it's checked against its schema. If the schema can't be satisfied this way, `lint` asks for an `example`.

## In diffs

With schemas, [`mockmachina diff`](diff.md) also reports field changes:

| Change | Severity |
| --- | --- |
| A response field removed, its type changed, or newly nullable | breaking |
| A response field no longer required | warning |
| A new required request field or parameter, or an optional one now required | breaking |
| A request field's type narrowed, or no longer nullable | breaking |
| A request field removed | info |
| A new response field, or a new optional request field | safe |
