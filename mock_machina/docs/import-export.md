# Import and export

The contract in `.mockmachina/` can come from, and go to, the formats backend teams already use.

## Import

```text
$ mockmachina import openapi.yaml
read OpenAPI 3.0.4 "Swagger Petstore - OpenAPI 3.0": 19 routes, 64 states (0 from examples, 14 generated from schemas), 6 schemas
wrote routes/pet.yaml
…
wrote schemas/api.yaml
note: all routes start with /api/v3, the path of the first server
next: mockmachina lint, then mockmachina start
```

| Format | Detected by |
| --- | --- |
| OpenAPI 3.0 and 3.1, YAML or JSON | `openapi: 3.x` |
| Swagger 2.0 | `swagger: "2.0"`, converted to OpenAPI 3 first |
| Postman Collection v2.1 | Its `info.schema` |

**What it becomes:**

| OpenAPI | MockMachina |
| --- | --- |
| Path and method | A route, named the way `mockmachina add` names it (`GET /users/{id}` is `users.get`), with the first server's path in front |
| `summary`, or the first line of `description` | `summary` |
| Path, query and header parameters, `requestBody` | `request` |
| Each response | `responses`, and a state named for its status (`success`, `not_found`, `server_error`…) |
| Several named examples | One state each, named after the example |
| No example | `body: generate` |
| `components/schemas` | `schemas/api.yaml` |
| OpenAPI 3.0 `nullable` and boolean `exclusiveMinimum` | The 2020-12 forms |

Anything that isn't imported is listed as a note, never dropped silently: cookie parameters, non-JSON bodies, callbacks, webhooks.

**From Postman:**
- `{{baseUrl}}/users/:id` becomes `/users/{id}`.
- Requests with the same method and URL become one route.
- Each saved example becomes a state.
- Each status gets a schema guessed from its examples. Fields present in every example are required; one that's sometimes `null` is nullable. Guessed schemas are marked `x-mockmachina-inferred: true`, and `lint` warns until you've checked them and deleted the mark.

**Importing again** merges, matching routes by id:
- New operations are added.
- `request` and `responses` are updated in place, and new example states are added.
- Your own states, `active`, `owners`, `status`, `summary`, comments and formatting are kept.
- Routes that are gone from the spec are kept and listed.
- `schemas/api.yaml` is rewritten.

Add `--dry-run` to see the changes without writing them.

## Export

```sh
mockmachina export -o openapi.yaml                 # OpenAPI 3.1
mockmachina export --format postman -o postman/    # a collection and an environment
```

- **OpenAPI 3.1:** every route is an operation whose `operationId` is the route id, and every state is a named example on its response. The MockMachina-only fields (`active`, `owners`, `status`, `rules`, `mode`, latency, faults, `set`…) travel as `x-mockmachina` extensions. Exporting and importing again gives back the same contract.
- **Postman:** a folder per resource, with a ready request per state that sets `X-Mock-State`, and the state saved as its example response. The environment sets `baseUrl` to `http://localhost:4001`.

`export` refuses when `lint` finds errors, so a broken contract doesn't reach other tools; `--force` overrides it. `--title` and `--version` set `info`.

## Docs

```sh
mockmachina docs --serve       # Swagger UI at http://127.0.0.1:4000, following your edits
mockmachina docs -o site/      # the same, as static files for any web host
```

Swagger UI is built into `mockmachina`, so the page works offline. Each state shows as a named example under its response.
