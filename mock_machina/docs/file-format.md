# The `.mockmachina` file format

Everything MockMachina serves comes from plain files in a `.mockmachina/` folder in your repository. You edit them by hand, review them in pull requests, and the running server picks up every save.

Every example on this page marked as routes or config is loaded by MockMachina's test suite, so it's guaranteed to work.

## Folder layout

```text
.mockmachina/
├── config.yaml              optional project settings
├── schemas/                 optional JSON Schemas, by name (see contracts.md)
│   └── users.yaml
├── data/                    optional starting data for CRUD routes
│   └── cart_items.json
└── routes/
    ├── users.yaml           every users route: list, get, create, delete…
    ├── users/               body files for those routes
    │   └── users.json
    └── orders.yaml
```

Each file in `routes/` holds the routes for one resource. Its name, here `users`, becomes the first half of every route id inside it: `users.list`, `users.get`.

MockMachina finds `.mockmachina/` in the folder you run it from or any folder above it, the way git finds `.git`, so you can run it from anywhere inside your app. Use `--dir` to point somewhere else.

Files whose names start with `.` are ignored, as are editor temp files. Route files must end in `.yaml`; a `.yml` file gets a reminder to rename it.

## A routes file

Each top-level key is a route name. A route answers one request and has one or more **states**: the different responses it can give. The first state is what your app gets unless it asks for another.

```yaml routes
owners: { backend: [ademola], frontend: [ada] }

list:
  route: GET /users
  summary: List users, newest first
  states:
    success:
      body: users.json
    empty:
      body: { users: [], nextPage: null }
    unauthorized:
      status: 401
      headers: { WWW-Authenticate: Bearer }
      body: { error: session expired }

get:
  route: GET /users/{id}
  states:
    found:
      body: { id: u_1, name: Ada }
    not_found:
      status: 404
      body: { error: user not found }

create:
  route: POST /users
  states:
    created:
      status: 201
      headers: { Location: /users/u_3 }
      body: { id: u_3 }
    invalid:
      status: 422
      body: { error: email is required }

delete:
  route: DELETE /users/{id}
  states:
    deleted:
      status: 204
    not_found:
      status: 404
```

### Fields at the top of the file

These apply to every route in the file, unless a route sets its own.

| Field | What it is |
| --- | --- |
| `owners` | Who to ask, as GitHub handles: `{ backend: [ademola], frontend: [ada] }` |
| `status` | Where the contracts stand: `draft` (the default), `agreed`, `implemented` or `deprecated` |
| `x-...` | Anything you like, kept as written |

Every other top-level key is a route. Route names use lowercase letters, digits and dashes.

### Route fields

| Field | Required | What it is |
| --- | --- | --- |
| `route` | yes | The method and path: `GET /users/{id}`. Methods are `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD` and `OPTIONS`, in uppercase. Parameters are whole segments in braces. No `?` or `#`; query parameters aren't part of the path |
| `states` | yes | The responses, by name, in the order you want them listed |
| `examples` | no | Values for the path's parameters, like `{ id: u_1 }`. Used by `mockmachina diff --live` to call a real backend |
| `active` | no | The state served by default. Defaults to the first state |
| `crud` | no | For `CRUD` routes: `{ collection, idField }`; see [CRUD routes](#crud-routes) |
| `rules` | no | A list of `{ when, state }`. The first rule that matches the request picks the state; see [rules](#rules) |
| `mode` | no | `rules` when the route has rules, otherwise `active` |
| `serve` | no | `proxy` sends the route's requests to the real backend set by `proxy` in `config.yaml` or `--proxy`, unless a request asks for a state with `X-Mock-State` or `?__state=`. Defaults to `mock` |
| `generated` | no | `true` on routes an AI assistant wrote through `mockmachina mcp`. `lint` warns until someone reviews the route and deletes the line |
| `summary` | no | One line saying what the route returns |
| `request` | no | Schemas for what the app must send: `params`, `query`, `headers` and `body`. See [contracts](contracts.md) |
| `responses` | no | The schema of each response, by status or `default`. `lint` checks every state's body against it. See [contracts](contracts.md) |
| `status` | no | Overrides the file's `status` for this route |
| `owners` | no | Overrides the file's `owners` for this route |
| `x-...` | no | Anything you like. Kept as written; exported to OpenAPI later |

A route's `status` describes the contract, not the HTTP response. HTTP status codes go inside a state.

### State fields

| Field | Default | What it is |
| --- | --- | --- |
| `status` | `200` | HTTP status code, 100–599 |
| `headers` | none | Response headers, like `{ Cache-Control: no-store }` |
| `body` | none | A file in the routes folder, inline YAML that's served as JSON, or `generate` to build it from the response schema |
| `validateRequest` | `true` | `false` serves this state even when the request doesn't match the route's `request` schemas |
| `latency` | none | How long to wait before responding, like `800ms` or `2s`, up to `1m`. For testing loading states. Units are required: `800` alone is an error. `{ base: 2s, jitter: 500ms }` varies it by up to the jitter either way |
| `fault` | none | Fail like a network does: `timeout`, `reset` or `truncated`. See [faults](#latency-and-faults) |
| `set` | none | Variables to store when this state is served, like `{ signed_in: true }`. See [variables](#variables) |
| `template` | `true` | `false` serves `{{ }}` in the body and headers as written |
| `generated` | none | `true` on states an AI assistant wrote. `lint` warns until someone reviews the state and deletes the line |
| `x-...` | | Anything you like, kept as written |

Every field is optional, so an empty state is a `200` with no body:

```yaml routes
get:
  route: GET /health
  states:
    up:
    down:
      status: 503
```

## Bodies

A body is either **a file** or **inline**:

- **A file** (`body: users.json`) is read from the folder named after the routes file: `routes/users/users.json` for routes in `users.yaml`. It's served as written. Its `Content-Type` comes from the extension: `.json`, `.txt`, `.html` and `.xml` are recognised, and anything else is `application/octet-stream`. A `Content-Type` in `headers` overrides it.
- **Inline YAML** (a mapping or a list) is served as JSON, with keys in the order you wrote them.
- **No body.** Leave `body` out, as in `deleted` above.

Body files can be shared between resources: `body: ../../shared/user.json` reads `.mockmachina/shared/user.json`. They can't come from outside `.mockmachina/`.

## Choosing a state

For each request, MockMachina serves the first of these that applies:

1. **The `X-Mock-State` header:** `curl -H 'X-Mock-State: empty' localhost:4001/users`
2. **The `__state` query parameter:** `localhost:4001/users?__state=empty`, which is handy in a browser
3. **The first of the route's `rules` that matches** (see [rules](#rules))
4. **The route's `active` state**, or its first state

### Rules

Rules pick a state from the request itself, so `GET /users/u_404` can return the 404 while other ids return the user:

```yaml
get:
  route: GET /users/{id}
  rules:
    - when: { path.id: u_404 }
      state: not_found
    - when: { header.authorization: { exists: false } }
      state: unauthorized
  states:
    found: { body: { id: u_1, name: Ada } }
    not_found: { status: 404 }
    unauthorized: { status: 401 }
```

Rules are checked in order, and the first one whose conditions **all** match picks the state. When none match, the `active` state is served. A header or `?__state=` still wins, so tests can always force a state.

| Selector | Reads |
| --- | --- |
| `path.<param>` | A path parameter: `path.id` for `/users/{id}` |
| `query.<name>` | A query parameter (its first value) |
| `header.<name>` | A request header, in any case |
| `cookie.<name>` | A cookie |
| `body.<a.b.0>` | A field in the JSON body; numbers index lists |
| `call` | How many times this route has been called since the server started, counting this request |
| `var.<name>` | A [variable](#variables) set by a state |

| Matcher | Matches when the value |
| --- | --- |
| `u_404` (a plain value) | Equals it. Numbers compare as numbers, so `query.page: 2` matches `?page=2` |
| `{ ne: x }` | Doesn't equal it, or is missing |
| `{ in: [a, b] }` | Is one of these |
| `{ matches: "^\\+234" }` | Matches the regex |
| `{ exists: false }` | Is missing (or present, with `true`) |
| `{ gt: 3 }`, `gte`, `lt`, `lte` | Is a number above or below this |

Each condition has one matcher. For "either of", write two rules that point at the same state. `call` makes retries testable: `when: { call: { lte: 2 } }` fails the first two calls and lets the third through.

Every response carries `X-Mock-State` and `X-Mock-Route` headers, so your app's network inspector shows what it got.

To change the default for everyone, without touching your app:

```text
mockmachina state set users.list empty
```

That writes `active: empty` into the `list` route, changing one line, or adding it above `states:` if the route didn't have one. A running server picks it up straight away.

### Modes

`mode` sets how a route picks its state when the request doesn't ask for one:

| Mode | Serves |
| --- | --- |
| `active` | The `active` state (the default) |
| `rules` | The first matching rule's state, otherwise `active` (the default when a route has `rules`) |
| `sequential` | Each state in turn, staying on the last: test "fails twice, then works" |
| `random` | A random state, picked from the [seed](#configyaml) |

## Templates

Bodies and header values can fill in parts of the request and generated data with `{{ }}`:

```yaml
get:
  route: GET /users/{id}
  states:
    found:
      headers: { X-Request: "user-{{ path.id }}" }
      body:
        id: "{{ path.id }}"
        name: "{{ fake.person.name }}"
        email: "{{ fake.email }}"
        joined: "{{ fake.date.past }}"
        score: "{{ random.int 1 100 }}"
```

With `--seed 42`, the first `GET /users/u_7` returns something like `{"id":"u_7","name":"Amelia Parker","email":"ben.hughes@example.com","joined":"2025-02-06T21:29:41Z","score":75}`. Each call gets new values, and `joined` moves with the clock.

| Template | Gives |
| --- | --- |
| `path.<param>`, `query.<name>`, `header.<name>`, `cookie.<name>` | That part of the request, or `""` |
| `body.<a.b.0>` | A field of the JSON request body, with its type |
| `call` | How many times this route has been called |
| `uuid` | A random UUID |
| `now`, `now.unix` | The current time, as RFC 3339 in UTC, or as seconds |
| `random.int A B` | A whole number from A to B |
| `fake.person.name`, `fake.person.first_name`, `fake.person.last_name` | A name |
| `fake.email`, `fake.username`, `fake.phone` | Contact details. Emails are always `@example.com`, `.net` or `.org` |
| `fake.city`, `fake.country`, `fake.company` | Places and businesses |
| `fake.word`, `fake.sentence` | Text |
| `fake.price` | A number from 1.00 to 500.00 |
| `fake.date.past`, `fake.date.future` | A time within two years |
| `fake.image.url` | A placeholder image URL |

A value that is exactly one template keeps its type: `"{{ random.int 1 100 }}"` is a number, and `"{{ body.qty }}"` is whatever the request sent. With text around it, it's a string. Templates work inside JSON string values, header values and `text/*` body files. Keys are left as they are, and the result is always valid JSON.

`fake.*` follows `locale` in [config.yaml](#configyaml): `en`, or `en_NG` for Nigerian names, cities and `+234` numbers. Everything random comes from the seed, so `mockmachina start --seed 42` gives the same responses every time. When a project uses randomness, `start` prints its seed. Set `template: false` on a state to serve `{{ }}` as written.

## Latency and faults

```yaml routes
create:
  route: POST /checkout
  states:
    placed:
      latency: { base: 800ms, jitter: 300ms }
      status: 201
    dropped:
      fault: reset
    sometimes:
      status: 201
      fault: { type: truncated, rate: 0.2, after: 1s }
```

| Fault | What the app sees |
| --- | --- |
| `timeout` | No answer at all, until the app gives up (or after 5 minutes) |
| `reset` | The connection is reset: a "connection reset" or "connection closed" error |
| `truncated` | The status, headers and half of the body arrive, then the connection closes: an unexpected end of data |

`rate` fails only that share of requests (`0.2` is one in five), and `after` waits first. Jitter and rates come from the [seed](#configyaml), so a run replays exactly with the same `--seed`.

## Variables

`set` stores variables when a state is served. Any route's rules (`var.<name>`) and templates (`{{ var.<name> }}`) can read them:

```yaml routes
create:
  route: POST /session
  rules:
    - when: { body.password: { ne: secret } }
      state: wrong_password
  states:
    signed_in:
      status: 201
      set: { signed_in: true, email: "{{ body.email }}" }
      body: { token: "{{ uuid }}" }
    wrong_password: { status: 401 }
delete:
  route: DELETE /session
  states:
    signed_out: { status: 204, set: { signed_in: null } }
me:
  route: GET /me
  rules:
    - when: { var.signed_in: { exists: false } }
      state: signed_out
  states:
    ok: { body: { email: "{{ var.email }}" } }
    signed_out: { status: 401 }
```

`null` removes a variable. Variables last while the server runs, through reloads, and a restart clears them.

## CRUD routes

`route: CRUD /path` answers a whole collection from memory:

```yaml routes
items:
  route: CRUD /cart/items
  crud: { collection: cart_items, idField: id }
  states:
    ok: {}
    down: { status: 503, body: { error: cart unavailable } }
```

| Request | Answer |
| --- | --- |
| `GET /cart/items` | `200` and the list. `?field=value` filters it |
| `GET /cart/items/{id}` | `200` and the item, or `404` |
| `POST /cart/items` | `201`, the item and a `Location` header. A missing id is filled in with a UUID; an existing one is a `409` |
| `PUT /cart/items/{id}` | `200` and the replaced item, or `404` |
| `PATCH /cart/items/{id}` | `200` and the item with the given fields changed, or `404` |
| `DELETE /cart/items/{id}` | `204`, or `404` |

The collection starts from `data/<collection>.json`, a JSON list of objects, if there is one; otherwise it starts empty. `crud` is optional: the collection defaults to the path's last segment (`items` here) and the id field to `id`. A state with no `status`, `body`, `headers` or `fault` means "behave as CRUD". Any other state, picked by a header, a rule or `active`, answers as usual, so `X-Mock-State: down` still tests the error screen. Changes last while the server runs; editing the data file or restarting resets them.

## Naming rules

| Name | Rule | Examples |
| --- | --- | --- |
| Routes file | Lowercase letters, digits and dashes, ending in `.yaml` | `users.yaml`, `order-items.yaml` |
| Route name | Lowercase letters, digits and dashes | `list`, `get`, `cancel-order` |
| Route id | The file name and the route name | `users.list`, `order-items.get` |
| State name | Lowercase letters, digits and underscores; starts with a letter | `success`, `server_error` |
| Path parameter | Letters, digits and underscores; each used once per path | `{id}`, `{user_id}` |

State names can't be YAML keywords (`yes`, `no`, `on`, `off`, `true`, `false`, `null`, `y`, `n`), because some YAML tools would read `active: no` as a boolean.

## config.yaml

Optional. Every field has a default, and command-line flags win over it.

```yaml config
version: 1
host: 127.0.0.1
ports:
  mock: 4001
```

| Field | Default | What it is |
| --- | --- | --- |
| `version` | `1` | File format version, so later releases can migrate older projects |
| `host` | `127.0.0.1` | Address to listen on. Use `0.0.0.0` so a phone on your Wi-Fi can connect |
| `ports.mock` | `4001` | Port the mock API is served on |
| `seed` | `0` | Seed for everything random: fake data, random states, latency jitter and fault rates. The same seed gives the same responses. `0` picks one at start and logs it; `--seed` overrides it |
| `locale` | `en` | Language and region for fake data: `en` or `en_NG` |
| `proxy` | none | The real backend, like `http://localhost:8080`. Requests that match no route, and routes with `serve: proxy`, are sent there instead of getting a 404. `--proxy` overrides it |

Changing `host` or `ports` while the server runs prints a reminder to restart; everything else applies live.

## Editor support

Add this line at the top of a routes file to get autocomplete and checking as you type in VS Code (with the YAML extension) or JetBrains IDEs:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/schema/route.schema.json
```

For `config.yaml`, use `config.schema.json` in the same place.

## When something's wrong

`mockmachina lint` checks every file without starting a server, and `mockmachina start` refuses to start until errors are fixed. Each problem names the file and line, says what's wrong, and suggests a fix:

```text
routes/users.yaml:6: active state "emty" doesn't exist (did you mean "empty"?)
routes/orders.yaml:3: unknown field "rotue" (did you mean "route"?)
routes/orders.yaml:2: warning: orders.get has no owners; reviews can't be routed
3 problems in 2 files
```

Errors stop the server from starting. Warnings are suggestions; `lint --strict` treats them as errors, which is useful in CI.

If you save a broken file while the server runs, it keeps serving the last good version and prints the problems. The next good save recovers on its own.
