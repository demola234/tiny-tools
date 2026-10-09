# Changelog

All notable changes to MockMachina are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). Release tags are `mock_machina/vX.Y.Z`.

## [0.1.0] - 2026-10-09


The first release: a mock HTTP server whose contract lives in your repository.

### Added

- `mockmachina init` creates `.mockmachina/` with settings and an example route; `--port`, `--no-example`, `--force`.
- `mockmachina add METHOD PATH` adds a route, naming it from the method and path (`GET /users/{id}` becomes `users.get`); `--name`, `--summary`.
- `mockmachina start` serves `.mockmachina/routes/*.yaml`, one file per resource with named routes and states.
  - A state is picked by the `X-Mock-State` header, then `?__state=`, then the route's `active` state, then its first state.
  - Responses carry `X-Mock-State` and `X-Mock-Route`; `--no-mock-headers` turns them off.
  - Per-state `latency`, up to 1 minute. A client that leaves early is logged as 499.
  - CORS on by default, with origin echoing, preflight answers and exposed mock headers; `--no-cors` turns it off.
  - Hot reload: saves apply to the next request, and a broken save keeps the last good version serving.
  - 404s list the closest routes; unknown states get a 400 with the valid states and a "did you mean".
  - A busy port suggests a free one.
- `mockmachina state set ROUTE STATE` changes a route's default by editing only its `active` line; `state list [ROUTE]`, with `--json`.
- `mockmachina lint` reports every problem with file, line and a likely fix, plus warnings; `--strict` fails on warnings.
- Optional `config.yaml` for `host` and `ports.mock`. Flags win over it.
- Every command finds `.mockmachina/` from any subfolder, and suggests the closest command when one is mistyped.
- JSON Schemas for route files and `config.yaml`, for editor autocomplete.
- `mockmachina diff [BASE[..HEAD]]` reports contract changes between git versions, each marked breaking, warning, info or safe; `--format text|json|markdown|github`, `--fail-on breaking|warning`.
- Diff reports name each changed route's owners: an Owners column and a "whom to ask" line in `--format markdown`, and `owners` in `--format json`.
- A GitHub Action (`demola234/tiny-tools/mock_machina`) runs `lint` and `diff` on pull requests: annotations on changed lines, one comment kept up to date, and a failing check on breaking changes unless the pull request has the `breaking-change` label.
- `mockmachina start --proxy URL` (or `proxy:` in `config.yaml`) sends requests that match no route to the real backend, and routes can opt in with `serve: proxy`. A request that asks for a state is still mocked.
- Rules: a route's `rules` pick the state from the request (path parameters, query, headers, cookies, JSON body fields, or how many times it's been called), with `eq`, `ne`, `in`, `matches`, `exists` and `gt`/`gte`/`lt`/`lte`. The log shows which rule matched.
- `mode: sequential` (each state in turn) and `mode: random` (a seeded pick).
- Templates in bodies and headers: `{{ path.id }}`, `{{ body.email }}`, `{{ uuid }}`, `{{ now }}`, `{{ random.int 1 100 }}` and `{{ fake.* }}` (names, emails, phones, cities, companies, words, prices, dates, image URLs) in `en` and `en_NG`. A value that is exactly one template keeps its type. `template: false` turns them off for a state.
- Schemas in `schemas/*.yaml` (JSON Schema 2020-12), referred to by name. Routes can declare `responses` (by status or `default`) and `request` (`params`, `query`, `headers`, `body`).
- `lint` checks every state's body against its response schema, in plain language with file and line, and reports undocumented statuses; `--format json|github` and `--only RESOURCE`.
- Request validation: a request that doesn't match the route's `request` schemas gets a 400 listing each problem. `validateRequest: false` on a state, or `start --no-request-validation`, turns it off.
- `body: generate` builds a state's body from its response schema, the same every run.
- `mockmachina import FILE` reads OpenAPI 3.0/3.1, Swagger 2.0 and Postman Collection v2.1, with guessed schemas for Postman; importing again merges without losing hand edits. `--dry-run`.
- `mockmachina export` writes OpenAPI 3.1 (MockMachina fields as `x-mockmachina`, exact round trip) or a Postman collection with an environment.
- `mockmachina docs --serve` and `docs -o DIR`: Swagger UI over the contract, bundled for offline use.
- `diff` reports field-level changes in response and request schemas.
- MCP `get_route` shows each route's `request` and `responses`.
- `latency: { base, jitter }` for responses that vary in time.
- `fault: timeout | reset | truncated`, or `{ type, rate, after }`, so apps can be tested against dropped connections, cut-off bodies and requests that never answer.
- `set` stores variables when a state is served; rules (`var.x`) and templates (`{{ var.x }}`) on any route read them, for sign-in and other flows. `null` removes one.
- `route: CRUD /path` serves an in-memory collection (list with filters, get, create, replace, update, delete), starting from `data/<collection>.json`. States still apply, so error screens stay testable.
- `diff` reports changes to modes, rules and faults; the MCP `get_route` tool shows them.
- The Flutter example signs in, uses a CRUD cart and handles a dropped checkout, all tested against the mock.
- `seed` and `locale` in `config.yaml`, and `start --seed`. The same seed gives the same responses, and `start` prints the seed when a project uses randomness.
- `mockmachina diff --live URL` compares the contract with a running API: statuses, content types, declared headers and JSON body shape. It sends only safe methods unless `--include-writes`; also `--header`, `--param` and `--timeout`. Routes can give path parameter values with `examples:`.
- `mockmachina mcp` lets AI assistants (Claude Code, Claude Desktop, Cursor, VS Code Copilot, ChatGPT) work with the contract over the Model Context Protocol. MockMachina never calls an AI itself.
  - Tools: `list_routes`, `get_route`, `lint`, `diff` and `diff_live` read the contract; `add_route`, `add_state` and `set_state` change it. Every write is checked before it's saved, and nothing is written if it would break a file.
  - Prompts: `suggest_states`, `draft_from_sample`, `explain_diff`, `check_live`.
  - `--print-config claude-code|claude-desktop|cursor|vscode` prints the setup; `--read-only` leaves out the writing tools; `--live-header` gives `diff_live` auth without putting tokens in the chat.
  - `--http ADDR` serves over HTTP for assistants that connect by URL, with `--token` (or `MOCKMACHINA_MCP_TOKEN`) required off this machine, and protection against browser pages and DNS rebinding.
- A `generated: true` field on routes and states marks what an AI assistant wrote. `lint` warns until a person reviews it and deletes the line.
- New `lint` warnings: a list route with no empty-list state, a route with a path parameter and no 404, and a write route with no 4xx.
- In a terminal, `mockmachina start` opens a live screen: routes and states on the left, requests on the right, and the header shows the address, seed and request rate. Enter on a state makes it the default (the routes file changes, as with `state set`), `/` filters, tab selects a request to see why it got its state, `r` reloads and `l` lists problems. Light and dark palettes follow the terminal. `start --plain`, and any output that isn't a terminal, keeps the log lines; `mockmachina tui` opens the screen too.
- `start --plain` in a terminal opens with a welcome card: the version, project, address, commands to try with this project's routes, and the seed and proxy notes.
- `mockmachina` on its own, in a terminal, opens a command picker: type to filter, choose a command, fill in what it needs (routes and states are listed from the project), see the exact command, and run it.
- `start --https` serves HTTPS with a certificate from a local certificate authority that MockMachina creates once, valid for `localhost`, the Android emulator's `10.0.2.2`, your LAN addresses and your computer's name. `--tls-cert` and `--tls-key` use your own certificate.
- `mockmachina cert` shows where the local CA is and how to trust it on each device; `--pem` prints it and `--install` trusts it on this computer and in a booted iOS Simulator.
- Releases: archives for macOS, Linux and Windows on amd64 and arm64, with checksums and an SBOM, on tags `mock_machina/vX.Y.Z`. An install script for macOS and Linux that checks checksums, a Homebrew tap (`demola234/tap/mockmachina`) and a Scoop bucket.
- A multi-arch Docker image, `ghcr.io/demola234/mockmachina`: static, non-root, serving `/mock/.mockmachina` on port 4001 and following edits.
- Docs: quickstart, file format, mobile setup (now with HTTPS), installing, HTTPS. Flutter example app with widget tests that run against the mock, and a storefront example that uses every feature and is checked by the test suite.
