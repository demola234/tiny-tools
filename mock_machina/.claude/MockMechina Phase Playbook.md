# MockMechina Phase Playbook

Oct 5, 2026 · @Ademola Kolawole

## How to use this playbook

This playbook plans MockMechina one phase at a time, in enough depth to start coding from it. Each phase is planned only after the previous one is reviewed, so later phases can learn from earlier ones. Phases 0 to 2 are planned below; Phases 3 to 9 are placeholders until their turn.

Every phase section answers the same questions: what it delivers and doesn't, which decisions must close first, the flows, what gets built and where, the tools and things we need, the build order, and how we know it's done.

| Phase | Version | Delivers | Status |
| --- | --- | --- | --- |
| 0 | – | Repo, data model, loader, frozen file format, CI | Planned below |
| 1 | v0.1 | init, add, start, switchable states, hot reload | Planned below |
| 2 | v0.2 | Schemas, lint, OpenAPI / Swagger / Postman | Planned below |
| 3 | v0.3 | Diff, breaking changes, GitHub Action, verify, proxy | Next |
| 4 | v0.4 | MCP server, AI draft and suggest | Later |
| 5 | v0.5 | Latency, faults, fake data, rules, CRUD | Later |
| 6 | v0.6 | WebSocket, SSE, gRPC, webhooks | Later |
| 7 | v0.7 | TUI, Docker, releases, HTTPS | Later |
| 8 | v0.8 | Control API and live events | Later |
| 9 | v0.9 | Web dashboard | Later |

Scope and ordering come from the [implementation plan](https://claude.ai/code/artifact/780af24a-0683-4734-9dfd-ffda4546ab2a). When this playbook changes a decision, the plan is updated too.

## Phase 0: Foundations

### Goal and scope

Phase 0 produces no user-facing feature. It produces the two things every later phase builds on: a file format that won't need to change, and Go code that can load, validate and explain problems in that format. Getting these right is cheaper now than at any later point, because every phase after this reads the same files.

**In scope**

- Decisions that block everything else, written down as short decision records (ADRs).
- The Go repository: module, folder layout, package boundaries, build and lint tooling.
- The complete data model for every protocol and feature in the plan (HTTP, WebSocket, SSE, gRPC, rules, CRUD, callbacks), even though most of it won't be used until later phases.
- The project loader: reads `.mockmechina/`, keeps line numbers, validates, and reports every problem at once in plain language.
- A machine-readable schema of the route file format, so editors can autocomplete and check files as people type.
- The CLI skeleton with `--version` and a structural `lint` command.
- CI on macOS, Linux and Windows.

**Out of scope** (later phases)

- Serving any mock traffic (Phase 1).
- Validating response bodies against schemas (Phase 2). Phase 0 only checks that the files are well-formed and refer to things that exist.
- Templates, fake data, rules evaluation (Phase 5). Phase 0 parses and stores them but doesn't run them.
- Any terminal UI beyond plain output.

**Done when**

1. `mockmechina --version` runs on macOS, Linux and Windows from a CI-built binary.
2. An invalid `route.yaml` produces an error that names the file, the line, what's wrong, and the likely fix.
3. The file format document and its schema are published, and the six decisions below are recorded.

### Decisions to close first

Six decisions block the rest of Phase 0. Each becomes a one-page ADR in `docs/decisions/` (context, options, decision, consequences). Mark each one Agreed here before coding starts on the parts it affects.

| # | Decision | Options | Recommendation | Why | Status |
| --- | --- | --- | --- | --- | --- |
| 001 | How contracts are stored | Own route files with OpenAPI export; or OpenAPI files plus a states sidecar | Own route files, one folder per route | Easier to read and diff in PRs; holds states, rules and non-HTTP protocols natively; OpenAPI round-trips through `x-mockmechina-*` fields | Agreed 2026-10-06 |
| 002 | Template syntax and engine | Go `text/template`; Handlebars library; own small parser | Handlebars-like syntax with our own small parser | `text/template` can't read `{{fake 'person.fullName'}}`; the Go Handlebars library was archived in June 2025 | Agreed 2026-10-06 |
| 003 | YAML library | `gopkg.in/yaml.v3`; `go.yaml.in/yaml/v4` | `go.yaml.in/yaml/v4` (v3 at the new path if v4 has no stable tag yet) | The old path is frozen and gets security fixes only | Agreed 2026-10-06: v3 at new path (v3.0.5); v4 still rc-only |
| 004 | CLI stack | Plain Cobra; Cobra with Charm's Fang | Cobra + Fang v2, Fang used only in `main.go`; all Charm libraries on v2 | Styled help and errors for free; Fang is marked experimental, so keep it easy to remove | Agreed 2026-10-06 |
| 005 | Licence | MIT; Apache-2.0 | Apache-2.0 | Includes a patent grant; common for developer tools companies adopt | Agreed 2026-10-06 |
| 006 | Name and module path | Keep MockMechina; rename | Keep, if the checks pass | Check GitHub org, Go module path, Homebrew formula, ghcr.io and Docker Hub image names, and a domain before the first public commit | Agreed 2026-10-06: MockMachina, inside tiny-tools |

Decision 001 is the one that matters most: everything else in the playbook assumes own route files. If you'd rather store OpenAPI directly, say so now, because it changes the data model and loader sections below.

### Flows

**Developer flow at the end of Phase 0.** A contributor can clone the repo, run the checks, and see the loader explain a broken project:

```text
git clone github.com/<org>/mockmechina && cd mockmechina
make tools      # installs pinned golangci-lint
make test       # same checks CI runs

mockmechina --version
  mockmechina v0.0.0-dev (a1b2c3d)

mockmechina lint --dir testdata/projects/broken/.mockmechina
  routes/users.list/route.yaml:7: active state "emtpy" doesn't exist (did you mean "empty"?)
  routes/users.list/route.yaml:12: state "slow" body file "slow.json" not found
  routes/orders.get/route.yaml:3: unknown field "mehtod" (did you mean "method"?)
  3 problems in 2 files
  (exit code 1)
```

**Project load flow.** The same loader runs for `lint` now, and for `start`, hot reload, the control API and the dashboard later. It never stops at the first error, so people fix everything in one pass.

&#91;embedded content: project load flow · 6 steps\]

The result is either a usable project with warnings, or a list of errors that blocks use. `lint` prints them; from Phase 1, `start` refuses to start on errors and hot reload keeps serving the last good version.

### Repository layout and packages

Phase 0 creates the full folder structure but fills only the packages it needs. Later phases add files inside existing packages instead of reorganising.

```text
mockmechina/
├── cmd/mockmechina/main.go        entry point: build root command, run, map exit codes
├── internal/
│   ├── cli/                       one file per command
│   ├── model/                     plain data types, no file access
│   ├── config/                    load, validate, write .mockmechina/
│   ├── clock/                     Clock interface + real clock
│   └── seed/                      reproducible randomness
├── schema/route.schema.json       published schema of route.yaml
├── testdata/projects/             fixture projects shared by all tests
├── docs/
│   ├── decisions/                 ADR 001–006
│   └── file-format.md
├── .github/workflows/ci.yml
├── .golangci.yml  .gitattributes  Makefile
└── README.md  CONTRIBUTING.md  LICENSE
```

| Package | Phase 0 job | Can import | Never imports |
| --- | --- | --- | --- |
| `cmd/mockmechina` | Wire everything, call Fang, set exit code | `cli` | anything else directly |
| `internal/cli` | Root command, `--version`, `lint`, shared flags | `config`, `model` | runtime packages (they don't exist yet) |
| `internal/model` | Every type in the file format | standard library only | everything else |
| `internal/config` | Read, validate and write project files; the only package that touches `.mockmechina/` on disk | `model`, YAML library | `cli` |
| `internal/clock` | `Clock` interface so time can be faked in tests | standard library | – |
| `internal/seed` | `RandFor(seed, route, call, purpose)` so `--seed` gives the same results even with parallel requests | standard library | – |

These rules are enforced by a test that lists each package's imports and fails on a forbidden one, so the boundaries hold as the code grows.

Why `clock` and `seed` are in Phase 0 when nothing uses them yet: every later feature that waits or picks randomly (latency, failure rate, fake data) must go through them. Adding them first means no code ever calls `time.Now` or global random functions directly, which is much harder to fix later.

### Data model and file format

The format is frozen at the end of Phase 0. "Frozen" means later phases add behaviour for fields that already exist, not new fields. So the model covers every protocol and feature in the plan now, and the loader rejects anything it doesn't recognise, which also catches typos.

**Project folder**

```text
.mockmechina/
├── config.yaml                 project settings (optional; defaults apply)
├── schemas/*.yaml              shared JSON Schemas (used from Phase 2)
├── protos/**/*.proto           gRPC definitions (used from Phase 6)
├── data/*.json                 collection seed data (used from Phase 5)
└── routes/<route-id>/
    ├── route.yaml              one route
    └── *.json                  state bodies
```

**config.yaml**

```yaml
version: 1                 # format version; lets later releases migrate old projects
ports: { mock: 4001, grpc: 4002, control: 4000 }
host: 127.0.0.1
seed: 0                    # 0 = random each start; any other value = reproducible
locale: en_NG
proxy: { target: "", enabled: false }
ai: { enabled: false }
```

`version` is the one addition to the earlier plan. Without it, a future format change can't tell old files from new ones.

**route.yaml fields**

| Field | Type | Required | Used from | Notes |
| --- | --- | --- | --- | --- |
| `id` | string | yes | 0 | Must equal the folder name. Lowercase letters, digits, dots, dashes |
| `protocol` | `http` \| `ws` \| `sse` \| `grpc` | no | 0 | Defaults to `http` |
| `method` | HTTP method or `CRUD` | HTTP | 1 |  |
| `path` | string with `{param}` | HTTP, WS, SSE | 1 |  |
| `service`, `rpc` | string | gRPC | 6 | e.g. `shop.v1.OrderService`, `GetOrder` |
| `summary` | string | yes | 0 | One line, shown everywhere |
| `status` | `draft` \| `agreed` \| `implemented` \| `deprecated` | yes | 0 | New routes default to `draft` |
| `owners` | `{ backend, frontend }` | no | 0 | GitHub handles; used for review approvals in Phase 3 |
| `request`, `responses` | schema refs | no | 2 | Validated from Phase 2 |
| `active` | state name | yes | 1 | Must exist in `states` |
| `mode` | `active` \| `rules` \| `sequential` \| `random` | no | 5 | Defaults to `active` |
| `rules` | list of `{ when, state }` | no | 5 | Parsed and reference-checked in Phase 0 |
| `serve` | `auto` \| `mock` \| `proxy` | no | 3 | Defaults to `auto` |
| `crud` | `{ collection, idField }` | no | 5 |  |
| `states` | ordered map of name → state | yes | 0 | Order matters for sequential mode and fallback |
| `generated` | bool | no | 4 | Set by AI tools |
| `x-*` | anything | no | 0 | Kept as-is; lets imports round-trip unknown OpenAPI extensions |
| group | string | no | 2 | From an OpenAPI tag or Postman folder; groups routes in lists and the dashboard (added during Phase 2 planning) |

**State fields** (one shape, protocol-specific parts optional)

| Group | Fields | Used from |
| --- | --- | --- |
| HTTP response | `status`, `headers`, `body` (file, inline JSON, or `generate`) | 1 |
| Timing | `latency: { ms, jitter }` | 1 (ms), 5 (jitter) |
| Faults | `fault: timeout \| reset \| truncated` | 5 |
| Side effects | `set` (variables), `callbacks` | 5, 6 |
| WebSocket / SSE script | `onConnect`, `on`, `every`, `reject`, `fault` (drop, close, delay, duplicate) | 6 |
| gRPC | `code`, `message`, `metadata`, `trailers`, `stream` | 6 |
| Templates | `template: false` to turn templating off | 5 |
| Request checks | validateRequest: false (skip request validation for this state) | 2 |

**Naming rules**

- Route ids: `^[a-z0-9][a-z0-9.-]*$`, e.g. `users.list`, `orders.get`, `chat.room`.
- State names: `^[a-z][a-z0-9_]*$`, e.g. `success`, `server_error`.
- Body file names default to `<state>.json`.

The full format, with one complete example per protocol, goes in `docs/file-format.md`. The same rules are published as `schema/route.schema.json` so editors with YAML support (VS Code, JetBrains) autocomplete fields and flag mistakes as people type, by adding one comment line at the top of a route file:

```yaml
# yaml-language-server: $schema=https://<docs-site>/schema/route.schema.json
```

### Loader, validation and error messages

The loader is the part of Phase 0 people will feel every day: every mistake in a contract file shows up through it. Its errors follow one format and one standard.

**Error format**

```text
<file relative to .mockmechina, forward slashes>:<line>: <what's wrong> (<how to fix>)
routes/users.list/route.yaml:7: active state "emtpy" doesn't exist (did you mean "empty"? states: success, empty, unauthorized)
```

- Always the file and line. Never a Go stack trace or a raw YAML library message on its own.
- Say what's wrong in the project's words (route, state, body file), not the parser's.
- Offer the likely fix: a close match for typos (edit distance 2 or less), or the list of valid values.
- Report every problem in one run, sorted by file then line, then a count.

**Checks in Phase 0**

| Check | Example message | Severity |
| --- | --- | --- |
| YAML is well-formed | `route.yaml:4: bad indentation (YAML needs spaces, not tabs)` | Error |
| No unknown fields (except `x-*`) | `unknown field "mehtod" (did you mean "method"?)` | Error |
| Required fields present for the protocol | `HTTP route needs "path"` | Error |
| Enum values valid (`protocol`, `status`, `mode`, `serve`) | `status "aproved" isn't valid (draft, agreed, implemented, deprecated)` | Error |
| `id` matches folder name and naming rules | `id "users.list" doesn't match folder "users-list"` | Error |
| `active` and every rule's `state` exist | `rule 2 points to state "locked", which doesn't exist` | Error |
| Body files exist and stay inside the project | `body "../secrets.json" points outside the project` | Error |
| No two routes claim the same method and path (or gRPC method, or socket path) | `GET /users is defined in users.list and users.all` | Error |
| State names follow naming rules | `state "Server Error" should be lowercase with underscores, e.g. server_error` | Error |
| `config.yaml` version supported | `config version 2 needs a newer mockmechina (you have v0.1.0)` | Error |
| Route has no failure state | `users.list has no 4xx or 5xx state; apps can't test errors` | Warning |
| Route has no `owners` | `users.list has no owners; reviews can't be routed` | Warning |

Warnings print but don't change the exit code. `lint --strict` turns warnings into errors for teams that want that in CI.

**How the loader keeps line numbers.** YAML is decoded in two layers: first into the YAML library's node tree, which knows the line of every key, then into the model types. Each model value keeps a small `Source{File, Line}` record, so later checks (including Phase 2's schema checks) can point at the right line without re-reading the file.

**Writing files.** Only the `config` package writes, and always by writing a temporary file next to the target and renaming it over the original. A crash or power cut mid-write can never leave a half-written route file. Writes preserve comments and key order where the YAML library allows, so a CLI change to `active` doesn't reformat the whole file in the next PR diff.

### Tooling, dependencies and CI

Phase 0 keeps dependencies to the minimum the loader and CLI need. Every later dependency is added by the phase that uses it.

| Dependency | Why | Notes |
| --- | --- | --- |
| Go 1.25+ | Language and toolchain | 1.25 brings `testing/synctest`, used from Phase 1 for timing tests |
| `github.com/spf13/cobra` | Command tree and flags |  |
| `charm.land/fang/v2` | Styled help, errors, `--version`, completions | Experimental per its authors; isolated in `main.go` |
| `charm.land/log/v2` | Readable log and error output |  |
| `go.yaml.in/yaml/v4` | YAML with node positions for line numbers | ADR 003 |
| `golang.org/x/term` | Detect a real terminal vs CI or pipes |  |
| `github.com/google/go-cmp` | Readable diffs in tests | test only |
| `github.com/rogpeppe/go-internal/testscript` | Run the real CLI from scripted tests | test only |
| golangci-lint | Static checks, import rules | pinned version, installed by `make tools` |

**CI** (`.github/workflows/ci.yml`, on every push and pull request):

1. `go mod tidy` leaves no changes.
2. `go vet ./...` and `golangci-lint run`.
3. `go test -race -shuffle=on ./...` on Ubuntu, macOS and Windows.
4. Build the binary on each OS and run `mockmechina --version` (this is exit criterion 1).
5. Validate every fixture against `schema/route.schema.json`.

Golden test files are pinned to Unix line endings in `.gitattributes`, so Windows runs compare the same bytes.

**Things we need before or during Phase 0**

- [ ] GitHub organisation and the `mockmechina` repository
- [ ] Branch protection on `main`: CI must pass, one review
- [ ] Name checks for ADR 006: GitHub, Go module path, Homebrew, ghcr.io, Docker Hub, domain
- [ ] Licence chosen (ADR 005) and `LICENSE` committed
- [ ] Somewhere to publish `route.schema.json` with a stable URL (GitHub Pages on the repo is enough for now)
- [ ] Go 1.25+ and golangci-lint on each contributor's machine (`make tools`)
- [ ] A short `CONTRIBUTING.md`: how to run tests, how to write an ADR, commit message style

### Build order

Twelve tasks, in order. Each ends with something that can be checked. Sizes are relative (S about half a day, M one to two days, L three or more), to help split work, not deadlines.

| # | Task | Needs | Size | Done when | Status |
| --- | --- | --- | --- | --- | --- |
| 1 | Close ADRs 001–006 | – | S | Six ADR files merged, decisions marked Agreed above | To do |
| 2 | Repo scaffold: module, folders, Makefile, licence, CI with an empty test on 3 OSes | 1 | S | CI green on Ubuntu, macOS, Windows | To do |
| 3 | `main.go`, root command, `--version` via Fang, exit-code mapping | 2 | S | `mockmechina --version` prints version and commit in CI on all OSes | To do |
| 4 | Model types for every protocol, ordered states, durations, gRPC codes | 1 | M | Maximal example of each protocol decodes and re-encodes unchanged | To do |
| 5 | Error type, typo suggestions, collect-all reporting | 2 | S | Unit tests for message format and suggestions | To do |
| 6 | Loader: find routes, decode with line numbers, resolve bodies | 4, 5 | M | `shop` fixture loads; broken fixtures give exact messages | To do |
| 7 | Validation checks from the table above | 6 | M | One fixture and golden message per check | To do |
| 8 | Safe file writes (temp + rename), keeping comments and order | 6 | S | Round trip: load, write, load gives the same project; comments kept | To do |
| 9 | `route.schema.json` plus a test keeping it in step with the model | 4 | M | All fixtures validate; a field added to the model without the schema fails the test | To do |
| 10 | `lint` command (structural checks, `--strict`, exit codes) | 7 | S | testscript: broken project exits 1 with all problems listed | To do |
| 11 | `clock` and `seed` packages, import-rule test | 2 | S | `RandFor` gives the same values regardless of goroutine order | To do |
| 12 | `docs/file-format.md` with one full example per protocol; README and CONTRIBUTING | 9 | M | Reviewed by one backend and one frontend developer | To do |

Tasks 4, 5 and 11 can run in parallel once task 2 is done. Task 12's review is the real freeze point: after it, format changes need a new ADR and a migration.

### Tests, exit criteria and risks

**Tests**

| Kind | What it proves | Where |
| --- | --- | --- |
| Golden error messages | Each check in the validation table gives the exact expected message, file and line | `testdata/projects/broken-*`, `testdata/golden/` |
| Decode and round trip | State order kept; durations and gRPC codes parse; load → write → load is identical, comments kept | `internal/config`, `internal/model` |
| Schema sync | Every fixture validates against `route.schema.json`; the model and schema can't drift | `schema/` test |
| Import rules | No package imports what the layout table forbids | one test over `go list` output |
| Reproducible randomness | `RandFor` gives identical values across 100 parallel runs | `internal/seed` |
| CLI scripts | `--version`; `lint` on good and broken projects with the right exit codes | `testdata/script/*.txtar` |

The two exit criteria become scripts that CI runs on all three OSes: `exit_phase0_version.txtar` and `exit_phase0_invalid_route.txtar`.

**Exit checklist**

- [ ] ADRs 001–006 agreed and merged
- [ ] `mockmechina --version` passes in CI on macOS, Linux and Windows
- [ ] Broken fixtures produce file:line messages with fixes, all problems in one run
- [ ] `docs/file-format.md` and `route.schema.json` published and reviewed by one backend and one frontend developer
- [ ] Implementation plan's Phase 0 items ticked; this playbook updated with anything that changed

**Risks**

| Risk | Effect | Mitigation |
| --- | --- | --- |
| Format frozen with a gap | A later phase needs a breaking file change | Model every phase's fields now; review against the per-protocol examples before task 12 signs off; `version` field allows migrations |
| Windows path and line-ending bugs | Wrong messages or failing tests on Windows | CI on Windows from task 2; forward-slash paths in all messages |
| YAML library writes reformat files | Noisy PR diffs after CLI edits | Edit via the node tree to keep comments and order; round-trip test |
| Fang changes or stalls | CLI help and errors break on upgrade | Used in one place only; falling back to plain Cobra is a one-line change |

**Open questions for you**

1. Do you agree with ADR 001 (own route files rather than OpenAPI as the stored format)?
2. MIT or Apache-2.0?
3. Is there already a GitHub organisation for this, or should the repo start under your personal account?
4. Who can review the file format from the backend side before it's frozen?

Once Phase 0 is reviewed, Phase 1 (the core mock server) is planned next in the same depth.

## Phase 1: Core mock server (v0.1)

### Goal and scope

Phase 1 is the first release anyone outside the project would use. A Flutter developer points their app at `localhost:4001`, gets responses from files in `.mockmechina/`, and switches between success, empty and error screens without touching app code or restarting anything.

**In scope**

- Commands: `init`, `add`, `start` (plain log output), `state set`, `state list`.
- The HTTP mock server: route matching, picking a state per request, writing the response from files.
- Hot reload: edits to `.mockmechina/` apply to the next request.
- Fixed latency per state (`latency.ms`), so a "slow" state works from the first release.
- CORS on by default, for Flutter web and browser apps.
- A small Flutter example app, plus setup docs for emulators and simulators.

**Out of scope** (later phases)

- Checking bodies against schemas, and request validation (Phase 2).
- Jitter, failure rates, timeouts and resets, templates, fake data, rules (Phase 5). States in v0.1 return fixed files.
- WebSocket, SSE, gRPC (Phase 6). Routes with those protocols load and lint, but `start` skips them with a notice.
- The live terminal view (Phase 7). `start` prints one line per request.

**Assumptions** (from Phase 0's open questions, which aren't answered yet)

- Contracts are stored as our own route files (ADR 001 as recommended).
- Licence Apache-2.0. Neither affects Phase 1's design; ADR 001 would.

**Done when** a real Flutter app renders its success, empty and 401 screens by changing only the `X-Mock-State` header or running `mockmechina state set`, with no app restart.

### Flows

**First run.** From nothing to an app hitting the mock in about two minutes:

```text
cd my-flutter-app
mockmechina init
  ? Port for the mock server (4001)
  ? Add an example route? (Y/n)
  created .mockmechina/config.yaml
  created .mockmechina/routes/health.get/  (GET /health, state: ok)

mockmechina add GET /users
  created .mockmechina/routes/users.list/route.yaml  (state: success)
  created .mockmechina/routes/users.list/success.json
  next: edit success.json, add more states in route.yaml

mockmechina start
  loaded 2 routes from .mockmechina/
  serving on http://127.0.0.1:4001  (Ctrl+C to stop)
  10:42:01 GET /users 200 success active 3ms
```

**Switching states.** Two ways, for two situations:

1. **Per request**, for automated tests or trying one call: send `X-Mock-State: empty`, or add `?__state=empty` to the URL. Only that request is affected.
2. **As the default**, while clicking through the app by hand: `mockmechina state set users.list empty`. This writes `active: empty` to the route file, hot reload picks it up, and every following request gets the empty state until it's changed back. Because it's a file change, the team sees it in `git status` and can commit a shared default.

**Path of one request.**

&#91;embedded content: path of one request in v0.1 · 6 steps, 2 error exits\]

The whole path reads one snapshot of the project, taken when the request arrives. A file edit in the middle of a request never mixes two versions.

### Commands

Every command works without prompts (flags for everything) so it runs in scripts and CI. Prompts appear only when a value is missing and the command runs in a real terminal.

| Command | What it does | Key flags | Writes | Exit codes |
| --- | --- | --- | --- | --- |
| `init` | Creates `.mockmechina/` with `config.yaml` and an optional example route | `--port`, `--no-example`, `--yes`, `--force` (overwrite) | `config.yaml`, `routes/health.get/` | 0 created; 1 already exists without `--force` |
| `add METHOD PATH` | Creates a route folder with a `success` state | `--id`, `--status` (default `draft`), `--summary` | `routes/<id>/route.yaml`, `success.json` | 0; 1 id or method+path taken |
| `start` | Loads, serves, reloads on change, stops on Ctrl+C | `--port`, `--host` (default 127.0.0.1), `--no-cors`, `--no-mock-headers`, `--poll`, `--log-format text\|json` | nothing | 0 clean stop; 1 project has errors or port in use |
| `state set ROUTE STATE` | Makes STATE the default for ROUTE | – | `active:` in that `route.yaml` only | 0; 1 unknown route or state (with suggestions) |
| `state list ROUTE` | Shows states, marks the active one | `--json` | nothing | 0; 1 unknown route |
| `lint` | From Phase 0, unchanged |  | nothing |  |

**Route ids from `add`.** The id comes from the last fixed path segment plus an action, so ids read naturally in logs and PRs:

| Request | Id |
| --- | --- |
| `GET /users` | `users.list` |
| `GET /users/{id}` | `users.get` |
| `POST /users` | `users.create` |
| `PUT` or `PATCH /users/{id}` | `users.update` |
| `DELETE /users/{id}` | `users.delete` |
| `GET /users/{id}/orders` | `orders.list` (then `users-orders.list` if taken) |
| `GET /health` | `health.get` |

If the id is still taken, `add` asks for one in a terminal, or fails with a suggestion and `--id` in scripts.

**What `start` prints**

```text
loaded 4 routes from .mockmechina/  (skipped 1 gRPC route: served from v0.6)
serving on http://127.0.0.1:4001  (Ctrl+C to stop)
10:42:01 GET  /users        200 success       active   3ms
10:42:03 GET  /users        200 empty         header   2ms
10:42:05 GET  /usrs         404 –             –        1ms  closest: GET /users
10:42:09 reloaded: routes/users.list/route.yaml (active: success → unauthorized)
10:42:15 reload failed, still serving the previous version:
         routes/users.list/route.yaml:9: state "slow" body file "slow.json" not found
```

The fourth column says why that state was chosen (`header`, `query`, `active`), which answers the most common question during debugging: "why am I getting this response?"

### Runtime design

New package `internal/runtime`. It turns a loaded project into an HTTP handler; `start` wraps it in a server.

**Routing.** Go's built-in router (`net/http.ServeMux`) already understands `GET /users/{id}`, so no router library is needed. Two of its limits shape the design:

- Routes can't be removed from a router once added, so every reload builds a brand-new router and swaps it in whole.
- Adding two conflicting patterns crashes the program. The builder catches that and turns it into a normal error naming both route files, so a bad edit can't take the server down.

**Picking the state.** First match wins:

1. `X-Mock-State` header
2. `__state` query parameter. It's removed from the query before anything else sees it, so it never leaks into later phases' templates or proxied requests.
3. The route's `active` state
4. The first state listed (only reachable if `active` is missing, which lint already flags)

State names are case-sensitive. An unknown name returns `400`:

```json
{ "error": "unknown_state", "route": "users.list", "state": "emtpy", "valid": ["success", "empty", "unauthorized"], "hint": "did you mean \"empty\"?" }
```

**Writing the response**

| State has | Result |
| --- | --- |
| `body: file.json` | File contents, `Content-Type` from the extension (`.json` → `application/json`) unless `headers` sets one |
| Inline JSON or YAML `body` | Encoded as JSON |
| No body, or status 204/304 | Empty body, no `Content-Type` |
| `headers` | Added as given; they win over defaults |

Body files are read when the project loads, not per request, so a missing file is a load error and serving is just copying bytes from memory.

**Headers MockMechina adds:** `X-Mock-State: <state>` and `X-Mock-Route: <route id>`, so app developers can see in their network inspector which state they got. `--no-mock-headers` removes them for clients that reject unknown headers.

**Unmatched requests:** `404` with up to three closest routes, ranked by same method first, then by how few characters differ in the path:

```json
{ "error": "no_route", "method": "GET", "path": "/usrs", "closest": ["GET /users", "GET /users/{id}"] }
```

**CORS** (on by default):

- Reflect the request's `Origin` instead of `*`, with `Vary: Origin`, so apps that send cookies or auth headers work.
- Answer every browser preflight (`OPTIONS`) with `204`, allowing the requested method and headers. Preflights are not logged as requests.
- Expose `X-Mock-State` and `X-Mock-Route` so browser code can read them.

**Latency.** If a state sets `latency.ms`, wait that long before writing. The wait ends early if the client disconnects, so abandoned requests don't pile up. All waiting goes through the `clock` package from Phase 0, which makes it testable without real delays.

**Logging and events.** After each request, the runtime publishes one event (time, method, path, route, state, why it was chosen, status, duration). In Phase 1 the only listener is the log line; the terminal view (Phase 7) and control API (Phase 8) subscribe to the same events later. Publishing never waits on a slow listener.

**Binding and shutdown.** Listens on `127.0.0.1` by default, so nothing outside the machine can reach the mock unless `--host 0.0.0.0` is given (needed for physical phones and Docker). On Ctrl+C, stops accepting new requests and gives in-flight ones up to 5 seconds to finish.

**Speed target.** Overhead under 2 ms per request on a laptop, excluding configured latency. The mock should never be the slow part of an app.

### Hot reload

Any change under `.mockmechina/` applies to the next request, with no restart. The rule that keeps this safe: **a broken edit never replaces a working project.**

**How it works**

1. A file watcher (fsnotify) watches `.mockmechina/` and every folder inside it. The watcher isn't recursive on Linux, so new route folders are added to the watch list as they appear.
2. Events are grouped: the reload waits until 100 ms pass with no new events. Editors save in several steps (write a temp file, rename, change permissions), and one save shouldn't cause three reloads.
3. The whole project is loaded again with the Phase 0 loader, including body files.
4. If loading succeeds, a new router is built and swapped in at once. Requests already running finish on the old version; new ones use the new one.
5. If loading fails, the errors are logged and the previous version keeps serving. The next successful save recovers automatically.
6. A one-line summary of what changed is logged (routes added, removed, active state changed).

**Edge cases**

| Case | Behaviour |
| --- | --- |
| Editor temp and swap files (`.swp`, `~`, `.#file`, `4913`) | Ignored |
| Hidden files and folders, `.git` | Ignored |
| A route folder deleted | Route disappears on reload; requests to it get the 404 with suggestions |
| `.mockmechina/` itself deleted or renamed | Keep serving, log a warning, resume watching if it comes back |
| `config.yaml` port changed | Logged as "restart needed for port changes"; everything else in config applies live |
| Many files change at once (git checkout, branch switch) | Grouping turns it into one reload |
| Docker Desktop or network folders where file events don't arrive | `--poll 1s` checks modification times instead; documented for Docker users |
| `state set` while `start` runs | It's a file write, so it reaches the server through the same reload path; no special channel needed |

**Why `state set` writes a file** instead of telling the running server directly: it keeps one way to change anything (the files), works whether or not the server is running, and leaves a visible change in `git status`. The control API in Phase 8 follows the same rule.

### Flutter example app and mobile setup

The exit test needs a real app, and new users need something to copy. Both are served by `examples/flutter_shop/`, kept deliberately small.

**What the example app has**

- One Users screen with four visual states: loading spinner, list, empty message, and a "session expired" screen for 401. A retry button for 500.
- The API base URL from a build flag: `flutter run --dart-define=API_BASE_URL=http://10.0.2.2:4001`.
- A debug-only badge showing the `X-Mock-State` response header, so it's obvious which state the app received.
- A widget test that calls the mock with `X-Mock-State` set to each state and checks the right screen appears. This doubles as the template users copy for their own tests.
- Its own `.mockmechina/` folder with `users.list` and four states, so `mockmechina start` inside the example just works.

**Reaching the mock from each device**

| Where the app runs | Base URL | Extra setup |
| --- | --- | --- |
| iOS Simulator | `http://localhost:4001` | none |
| Android Emulator | `http://10.0.2.2:4001` (the emulator's name for the host machine) | Allow plain HTTP in the debug build (network security config); Android blocks it by default |
| Android phone over USB | `http://localhost:4001` | `adb reverse tcp:4001 tcp:4001` forwards the phone's port to the computer |
| iPhone or Android on Wi-Fi | `http://<computer's LAN IP>:4001` | `mockmechina start --host 0.0.0.0`; iOS debug builds need local networking allowed in Info.plist |
| Flutter web, Chrome | `http://localhost:4001` | none; CORS is on by default |
| Desktop (macOS, Windows, Linux) | `http://localhost:4001` | macOS apps need the network client entitlement |

This table becomes `docs/mobile-setup.md`, with copy-paste snippets for the Android network security config and the iOS Info.plist key. Phase 7's HTTPS support removes the plain-HTTP exceptions for teams that don't want them.

**Things we need for Phase 1**

- [ ] Flutter SDK, Xcode with an iOS Simulator, Android Studio with an emulator (for the manual exit check and the recording)
- [ ] One physical Android phone and one iPhone to verify the Wi-Fi and USB rows above
- [ ] Dependency: `github.com/fsnotify/fsnotify`, and `charm.land/huh/v2` for prompts
- [ ] A 30-second screen recording of the exit flow for the README
- [ ] `docs/quickstart.md`, `docs/states.md`, `docs/mobile-setup.md`

### Build order

The order gets a request served end to end first (tasks 1–3), then fills in behaviour, then the commands that edit files, then the example and docs.

| # | Task | Needs | Size | Done when | Status |
| --- | --- | --- | --- | --- | --- |
| 1 | `start` loads the project once and serves it (no reload yet); clean shutdown | Phase 0 | S | `curl localhost:4001/health` returns the example state | To do |
| 2 | Router builder: fresh router per project, conflict errors instead of crashes | 1 | M | Two routes with the same pattern give a file-named error, server stays up | To do |
| 3 | Response writing: body files read at load, content types, headers, 204 | 2 | S | Every state in the `shop` fixture returns its exact status, headers and body | To do |
| 4 | State selection: header, `__state`, active, first; 400 for unknown names | 3 | S | Precedence table test passes | To do |
| 5 | `X-Mock-*` headers, 404 with closest routes | 3 | S | Golden tests for both | To do |
| 6 | CORS middleware and `--no-cors` | 3 | S | Browser preflight and real request pass from a test origin | To do |
| 7 | Events and log lines, `--log-format json` | 4 | S | Log line shows state and why it was chosen | To do |
| 8 | Fixed latency through the clock, cancelled on disconnect | 4 | S | Timing test passes without real waiting | To do |
| 9 | Hot reload: watcher, grouping, swap, keep last good version, `--poll` | 2 | M | Edit, delete and broken-file tests pass under the race detector | To do |
| 10 | `init`, `add` (id rules), `state set`, `state list`, with prompts and flags | Phase 0 writer | M | testscript covers each command, including typos and collisions | To do |
| 11 | Flutter example app with its widget test and `.mockmechina/` | 1–9 | M | Widget test passes against a running mock | To do |
| 12 | Docs: quickstart, states, mobile setup; README recording | 11 | S | Someone new follows the quickstart without help | To do |

Task 10 can be built alongside tasks 2–9, since it only touches files through the Phase 0 writer. Release v0.1 when all twelve are Done and the exit tests pass.

### Tests, exit criteria and risks

**Tests**

| Kind | What it proves |
| --- | --- |
| State precedence table | Header beats query beats active beats first; unknown name gives 400 with suggestions |
| HTTP integration (real client against a test server) | Each `shop` fixture state returns its exact status, headers and body; `X-Mock-*` headers present or removed by flag |
| 404 and 400 golden files | Error bodies stay stable for apps that parse them |
| CORS | Preflight returns 204 with the right headers; real request reflects the origin |
| Latency (Go's fake-time test mode) | `slow` waits exactly its `latency.ms`; a disconnect ends the wait |
| Reload | Change `active`, add a route, delete a route, save a broken file: each behaves as the hot reload table says |
| Concurrency | 100 parallel requests during repeated reloads, run with the race detector |
| CLI scripts | `init`, `add` (incl. id collisions), `state set` with a typo, `state list --json`, `start` refusing a broken project |
| Flutter widget test | The example app shows the right screen for each state |

**Exit criteria as automated tests**

- `exit_phase1_states.txtar`: starts the server in the background, requests `/users` with no header, then with `X-Mock-State: empty` and `unauthorized`, checks bodies and statuses; runs `state set users.list empty`, waits for the reload log line, and checks the default response changed.
- The Flutter widget test in CI (Linux runner with Flutter installed), run against `mockmechina start`.
- Manual once, recorded for the README: the real app on an Android emulator and an iOS simulator, switching all three screens without a restart.

**Exit checklist**

- [ ] All twelve build tasks Done
- [ ] Exit scripts green on macOS, Linux and Windows
- [ ] Manual check done on both emulators, recording added to the README
- [ ] v0.1.0 tagged (release automation comes in Phase 7; a manual GitHub release with binaries built by CI is enough here)
- [ ] Plan doc's Phase 1 items ticked

**Risks**

| Risk | Effect | Mitigation |
| --- | --- | --- |
| File events missed in Docker or network folders | Edits don't apply; looks broken | `--poll` fallback, documented with Docker |
| Port 4001 already used | `start` fails confusingly | Clear message naming the port and `--port`; suggest the next free port |
| Plain HTTP blocked on Android and iOS | App can't reach the mock; first impression ruined | Mobile setup doc with copy-paste snippets; called out in the quickstart |
| State switching via files is too slow for clicking through screens | Frustration | Reload target under 200 ms after save; the terminal view in Phase 7 adds one-key switching |

**Open questions for you**

1. Is v0.1 released publicly, or shared privately with a few Flutter developers first? A private round would catch setup problems before strangers hit them.
2. Should the example app live in the main repo (`examples/`) or a separate repo? The main repo keeps it tested in CI; a separate repo is easier to clone on its own.
3. Do you have test devices for both Android and iOS, or should the manual check rely on emulators only?

When you're happy with Phase 1, Phase 2 (the contract layer) is next.

## Phase 2: Contract layer (v0.2)

### Goal and scope

In v0.1 a route is a set of canned responses. Phase 2 makes it a spec: every response has a shape (a schema), MockMechina checks that each state matches that shape, and the whole contract moves in and out of the formats backend teams already use. After this phase, backend can read, review and export the contract without learning anything new.

**In scope**

- Shared schemas in `.mockmechina/schemas/`, referenced from routes for request params, request body, and each response status.
- Full `lint`: every state body checked against its schema, every documented status covered by a state.
- Request validation in the mock: a wrong request gets a `400` that names the bad field.
- Route metadata (`summary`, `status`, `owners`) shown in `state list`, `lint` and errors.
- Bodies generated from schemas when a state has no example, so imported specs work straight away.
- Import from OpenAPI 3.0 / 3.1, Swagger 2.0 and Postman Collection v2.1.
- Export to OpenAPI 3.1 and Postman (with an environment file).
- `mockmechina docs`: Swagger UI over the exported spec.

**Out of scope** (later phases)

- Comparing versions and finding breaking changes (Phase 3).
- Fake-data templates such as `{{fake 'person.fullName'}}` (Phase 5). Phase 2's generated bodies come from the schema alone.
- AsyncAPI and proto import (Phase 6).

**Done when** a real backend OpenAPI spec imports, lints clean, exports back without losing any operation, and a state with a wrong field type fails `lint` with a file and line.

### Flows

Teams start from one of two sides, and Phase 2 has to serve both.

**Backend-first: the API already has a spec.**

```text
mockmechina import openapi.yaml
  read OpenAPI 3.0.3 · 24 operations, 41 responses, 17 schemas
  added   24 routes (status: draft)
  added   41 states (33 from examples, 8 generated from schemas)
  wrote   17 schemas to schemas/
  next: mockmechina lint, then mockmechina start

mockmechina lint          # 0 errors, 3 warnings (routes without owners)
mockmechina start         # frontend builds against it the same day
```

**Frontend-first: the API doesn't exist yet.**

```text
mockmechina add POST /login
write schemas/auth.yaml (LoginRequest, Session)
reference them in routes/auth.login/route.yaml
mockmechina lint          # catches a body that doesn't match its schema
mockmechina export --format openapi -o openapi.yaml   # hand to backend for codegen
mockmechina export --format postman -o postman/        # collection + environment
```

**Formats in and out.** Whatever comes in, the contract in `.mockmechina/` is what lint checks, the mock serves and every export is made from.

&#91;embedded content: formats in and out · 3 imports, 3 exports, one lint\]

A lint error blocks `export` (with `--force` to override), so a broken contract never reaches the backend team's tools.

### Schemas

**Where they live.** One YAML file per domain in `.mockmechina/schemas/` (`user.yaml`, `order.yaml`, `error.yaml`), each holding several named schemas. Routes point at them with a relative reference:

```yaml
# schemas/user.yaml
User:
  type: object
  required: [id, name, email]
  properties:
    id:    { type: string, pattern: "^u_" }
    name:  { type: string, minLength: 1 }
    email: { type: string, format: email }
    phone: { type: [string, "null"] }
UserList:
  type: object
  required: [users]
  properties:
    users:    { type: array, items: { $ref: "#/User" } }
    nextPage: { type: [integer, "null"] }
```

```yaml
# routes/users.list/route.yaml (excerpt)
request:
  query:
    page: { type: integer, minimum: 1 }
responses:
  200: { $ref: "../../schemas/user.yaml#/UserList" }
  401: { $ref: "../../schemas/error.yaml#/Error" }
```

Small one-off schemas can sit inline in the route; anything used twice belongs in `schemas/`.

**Dialect.** JSON Schema 2020-12, the version OpenAPI 3.1 uses, so exported specs need no translation. OpenAPI 3.0 imports use an older variant (`nullable: true` instead of `type: [x, "null"]`); the importer converts it on the way in. Formats like `email`, `date-time` and `uuid` are enforced, not just documented, because a mock that returns `"email": "not-an-email"` would hide app bugs.

**Library.** `santhosh-tekuri/jsonschema` v6: supports every draft from 4 to 2020-12, resolves `$ref` across files, and reports the exact location of each failure. Numbers are read without rounding, which matters for money in kobo.

**Errors in contract language.** Raw validator output reads like `jsonschema: '/users/1/email' does not validate with .../format`. MockMechina rewrites it:

```text
routes/users.list/success.json: users[1].email: "amaka@" isn't a valid email (schema User.email in schemas/user.yaml:8)
routes/users.list/success.json: users[0]: missing required field "name" (schema User in schemas/user.yaml:3)
```

Every message names the body file, the path inside the body, what's wrong, and the schema file and line that says so. When one problem causes a cascade (a missing object makes five nested checks fail), only the root cause is shown.

### Lint

Phase 0's structural checks stay. Phase 2 adds the checks that make the contract trustworthy. `lint` runs them all, and `start` runs them at startup and on every reload (warnings logged, errors keep the last good version).

**New checks**

| Check | Example message | Severity |
| --- | --- | --- |
| Every state's body matches the schema for its status | `success.json: users[1].email: "amaka@" isn't a valid email` | Error |
| Every documented status has at least one state | `users.list documents 401 but has no state returning 401` | Error |
| Every state's status is documented | `state "teapot" returns 418, which users.list doesn't document` | Error |
| Schema references resolve | `$ref ../../schemas/user.yaml#/Usr not found (did you mean User?)` | Error |
| Schemas themselves are valid JSON Schema | `schemas/user.yaml:6: "minLenght" isn't a schema keyword (did you mean minLength?)` | Error |
| Request example (if given) matches the request schema | `request example: page: -1 is below minimum 1` | Error |
| Generated bodies validate (sanity check on the generator) | `couldn't generate a valid body for schemas/order.yaml#/Order: recursive with no exit` | Error |
| Route has no failure state | `users.list has no 4xx or 5xx state` | Warning |
| Route has no owners | `users.list has no owners` | Warning |
| Schema guessed from Postman examples, not yet reviewed | `schemas/imported.yaml#/Cart was inferred; review optional and nullable fields` | Warning |
| Schema defined but never referenced | `schemas/user.yaml#/LegacyUser isn't used by any route` | Info |

**Output formats**

| Flag | For | Shape |
| --- | --- | --- |
| (default) | People in a terminal | Grouped by file, sorted by line, count at the end, colour when it's a terminal |
| `--format json` | Tools and editors | `[{severity, file, line, column, route, message, hint}]` |
| `--format github` | GitHub Actions | `::error file=...,line=...::message` so problems appear inline on the PR diff |

`--strict` turns warnings into errors. `--only route-id` lints one route, useful while editing. Exit code is 1 when there are errors (or warnings under `--strict`), 0 otherwise.

**Speed target.** Under one second for 200 routes, so lint can run on every save from an editor task without being noticed. Schemas are compiled once per run and cached.

### Request validation and generated bodies

**Request validation.** The mock now checks what the app sends, which catches frontend contract mistakes on the first request instead of in QA. It runs after the route matches and before a state is picked.

| Part of the request | Checked against | Example failure |
| --- | --- | --- |
| Path params | `request.params` schema | `id: "abc" doesn't match pattern ^u_` |
| Query params | `request.query` schema, after converting text to numbers and booleans where the schema says so | `page: "zero" isn't an integer` |
| Headers | `request.headers` schema (names case-insensitive) | `Authorization is required` |
| Body | `request.body` schema, when the content type is JSON | `body.email is required` |

A failure returns `400` and the state isn't served:

```json
{ "error": "invalid_request", "route": "auth.login", "problems": [ { "at": "body.email", "message": "is required" }, { "at": "body.password", "message": "must be at least 8 characters" } ] }
```

Rules that keep it from getting in the way:

- Only what the route documents is checked. A route with no `request` section accepts anything, so v0.1 projects keep working.
- Extra fields are allowed unless the schema says `additionalProperties: false`. Real apps send extra data.
- `X-Mock-State` and `__state` are never validated.
- A state can opt out with `validateRequest: false` (useful for a `validation_error` state that apps trigger on purpose with bad input).
- `start --no-request-validation` turns it off everywhere.
- The log line shows `invalid_request` with the first problem, so the developer sees it without opening the response.

**Generated bodies.** Imports often have schemas without examples. Rather than leaving empty states, MockMechina builds a body from the schema. In Phase 2 this is plain and predictable; Phase 5 makes it realistic with fake data.

| Schema says | Generated value |
| --- | --- |
| `example` or `examples` | That example, always first choice |
| `enum` / `const` | First enum value / the constant |
| `format: email`, `date-time`, `uuid`, `uri` | `user@example.com`, a fixed timestamp, a fixed UUID, `https://example.com` |
| string with `pattern` | A value matching the pattern if simple; otherwise flagged for an example |
| number / integer | `minimum` if set, else 1; respects `maximum` |
| array | `minItems` items, at least 1 |
| object | All required properties; optional ones left out |
| `null` allowed | Not null (the non-null shape is more useful to build screens against) |

Output is the same every run, so generated states can be committed and diffed. Generation stops at 6 levels deep for recursive schemas. Every generated body is validated against its schema; if that fails, lint reports it (the generator has a bug or the schema is contradictory). States created this way are marked `body: generate` in the route file, and `mockmechina state freeze ROUTE STATE` writes the generated body to a file so it can be edited by hand.

### Import and export

The rule for every format: **nothing the user wrote is ever lost.** Imports never overwrite hand-edited states, and exports carry MockMechina-only fields along so a round trip is exact.

**OpenAPI 3.0 and 3.1** (library: `getkin/kin-openapi`)

| OpenAPI | Becomes | Notes |
| --- | --- | --- |
| `operationId` | Route id | Slugged (`listUsers` → `users.list` when it fits the id rules, else kept as `listusers`) |
| No `operationId` | Route id from path and method | Same rules as `mockmechina add` |
| Path and method | `path`, `method` | `{param}` syntax is already the same |
| `summary`, `description` | `summary` | First line of description if no summary |
| `tags[0]` | `group` | Used by the dashboard later |
| Parameters, `requestBody` | `request` |  |
| Each response code | A `responses` entry and one state | `default` becomes a `server_error` state with 500 |
| One `example` | State body file |  |
| Several `examples` | One state per example, named after the example key | `empty`, `full_list` |
| No example | `body: generate` |  |
| `components/schemas` | Files in `schemas/`, one per tag or one `openapi.yaml` file | References rewritten to relative file refs |
| `x-mockmechina-*` | The MockMechina fields they hold (`active`, `owners`, `status`) | Written by our own export |
| Other `x-` fields | Kept on the route as-is | Exported again unchanged |

Everything imported starts as `status: draft`. Unsupported pieces (callbacks before Phase 6, links, security schemes beyond documenting them) are kept as `x-` data and listed in the import summary, never silently dropped.

**Swagger 2.0.** Converted to OpenAPI 3 in memory with the library's converter, then imported the same way. The summary says it was converted.

**Postman Collection v2.1** (own reader; the format is simple JSON)

| Postman | Becomes |
| --- | --- |
| Request | Route |
| Folder | `group` |
| Saved example response | State, named after the example, with its status and body |
| `{{baseUrl}}/users/{{userId}}` | `/users/{userId}`; `:id` style segments too |
| Example bodies | A schema guessed from all examples for that status: fields present in every example are required, mixed types become "one of". Marked as inferred and warned about in lint |

**Re-import (the second import of the same spec).** Matched by route id:

- New operations: added.
- Changed schemas or new response codes: updated, and the summary lists them.
- States you created or edited by hand, `active`, `owners`, `status`: kept.
- Operations gone from the spec: not deleted. Listed as "removed upstream" so a person decides.
- `--dry-run` prints the summary without writing.

**Export**

| Format | Command | What's in it |
| --- | --- | --- |
| OpenAPI 3.1 | `export --format openapi -o openapi.yaml` | Every route, schemas under `components`, each state as a named example on its response, MockMechina fields as `x-mockmechina-*`. Keys sorted so diffs are small. Validated before writing |
| Postman | `export --format postman -o postman/` | A collection with one request per route and each state as a saved example, a folder of ready requests per state with `X-Mock-State` set, and an environment file with `baseUrl = http://localhost:4001` |

**Round-trip guarantee.** For every spec in the test corpus: import, export, import again, and the two imported contracts are identical. This test is what keeps the "nothing lost" rule true as formats grow.

### Docs site, spec corpus and things we need

**`mockmechina docs`.** Two modes:

- `docs --serve`: serves Swagger UI at `http://localhost:4000/__mockmechina/docs` over a live export of the contract, refreshed on every reload. Each state shows up as a named example under its response, so backend sees exactly what frontend is building against.
- `docs -o site/`: writes the same pages as static files for GitHub Pages or any web host.

Swagger UI's files are bundled into the binary (it's Apache-2.0 licensed; its licence ships alongside). No internet needed. This also starts the use of port 4000, which the control API takes over in Phase 8.

**Spec corpus.** A set of real specs in `testdata/specs/` that every import, lint and export change is tested against. Each entry records its source and licence.

| Spec | Why it's in the corpus |
| --- | --- |
| OpenAPI Petstore (3.0 and 3.1 versions) | The standard small example; both dialects |
| Swagger 2.0 Petstore | Conversion path |
| One mid-size public API spec (30–100 operations, permissive licence) | Real-world size, `allOf`, `oneOf`, shared components |
| One Postman collection with saved examples | Postman import and schema guessing |
| Our own `shop` contract, exported | Round trip of MockMechina-only fields |
| A deliberately awkward spec we write (recursive schemas, discriminators, odd names) | Edge cases |

**Things we need for Phase 2**

- [ ] Dependencies: `github.com/santhosh-tekuri/jsonschema/v6`, `github.com/getkin/kin-openapi`
- [ ] A pinned Swagger UI release and a small script to update it
- [ ] The corpus files, with sources and licences noted in `testdata/specs/README.md`
- [ ] A real backend OpenAPI spec from a friendly team for the exit check (or the mid-size public one)
- [ ] A backend developer to read an exported spec and say whether they'd use it as-is
- [ ] Docs: `docs/contracts.md` (schemas and lint), `docs/import-export.md`

### Build order

Schemas and lint come first because import, export and request validation all depend on them. OpenAPI is built before Postman and Swagger because both reuse its pieces.

| # | Task | Needs | Size | Done when | Status |
| --- | --- | --- | --- | --- | --- |
| 1 | Schema registry: load `schemas/`, resolve refs across files, cache compiled schemas | Phase 1 | M | Refs between files resolve; bad refs give file:line errors | To do |
| 2 | Validate state bodies; rewrite errors into contract language, root cause only | 1 | M | Golden messages for type, required, format, enum failures | To do |
| 3 | New lint checks, output formats (text, json, github), `--strict`, `--only` | 2 | M | One fixture and golden output per check and format | To do |
| 4 | Lint on `start` and every reload | 3 | S | Broken body on save keeps the last good version and logs the error | To do |
| 5 | Request validation step, opt-outs, log line | 1 | M | 400 body golden; extra fields allowed; opt-outs work | To do |
| 6 | Body generator from schemas, `body: generate`, `state freeze` | 1 | M | 500 generated bodies per corpus schema all validate; output identical across runs | To do |
| 7 | OpenAPI import (3.0 and 3.1), including 3.0 `nullable` conversion | 6 | L | Petstore and the mid-size spec import and lint clean | To do |
| 8 | OpenAPI export 3.1 with `x-mockmechina-*`, validated before writing | 7 | M | Round trip identical for the whole corpus | To do |
| 9 | Re-import merge with `--dry-run` | 7 | M | Hand-edited states survive a re-import; summary golden | To do |
| 10 | Swagger 2.0 import via conversion | 7 | S | Swagger Petstore imports and lints clean | To do |
| 11 | Postman import with schema guessing; Postman export with per-state requests and environment | 7 | L | Collection imports; export opens in Postman with working state requests | To do |
| 12 | `docs --serve` and `docs -o` with bundled Swagger UI | 8 | S | Docs page shows every route and state offline | To do |
| 13 | Corpus harness in CI; docs pages | 8–12 | S | Corpus round trip runs on every PR | To do |

Tasks 5 and 6 can run in parallel with 3 and 4. Tasks 10 and 11 can run in parallel once 7 is done.

### Tests, exit criteria and risks

**Tests**

| Kind | What it proves |
| --- | --- |
| Lint goldens | Each check gives the exact message, file and line, in all three output formats |
| Error rewriting | Validator output becomes contract language; cascades collapse to the root cause |
| Request validation | Each part (params, query, headers, body) fails correctly; extra fields pass; opt-outs work |
| Generator properties | 500 generated bodies per corpus schema all validate; same input gives same output |
| Import mapping | Each row of the OpenAPI and Postman mapping tables has a fixture and expected route files |
| Round trip | Import → export → import is identical for every corpus spec, including `x-` fields |
| Re-import | Hand-edited states, `active`, `owners` and `status` survive; removed operations are reported, not deleted |
| Fuzzing (nightly) | Importers and the generator never crash on malformed or strange input |

**Exit criteria as automated tests**

- `exit_phase2_roundtrip.txtar`: import the mid-size spec, lint (no errors), export, re-import, compare.
- `exit_phase2_lint_type.txtar`: change a field in a state body from a number to a string; lint fails with file, path in the body, and line of the schema.
- Manual once: import a real team's spec, start the mock, and have the Flutter example (or that team's app) call two of its routes.

**Exit checklist**

- [ ] All 13 build tasks Done
- [ ] Exit scripts green on all three OSes
- [ ] A backend developer has read an exported spec and confirmed it's usable as-is
- [ ] v0.2.0 released; plan doc's Phase 2 items ticked

**Risks**

| Risk | Effect | Mitigation |
| --- | --- | --- |
| OpenAPI edge cases (`allOf` chains, discriminators, recursion) | Imports fail or lose detail | Unknown constructs kept as raw schema and exported unchanged; awkward-spec fixture; depth limit in the generator |
| Request validation rejects legitimate traffic | Teams turn it off and lose the benefit | Only documented parts checked, extra fields allowed, clear 400 body, per-state and global opt-outs |
| Postman schema guessing is wrong | Contract encodes bad assumptions | Always `draft`, always flagged in lint until a person edits the schema |
| Format errors too technical | People ignore lint | Rewriting into contract language, reviewed with someone who hasn't seen JSON Schema |

**Open questions for you**

1. When an imported spec has no `operationId`s, should ids follow our `users.list` style (readable, may differ from backend names) or keep a mechanical form close to the spec?
2. Should `export` refuse on lint errors by default (as planned), or only warn?
3. Is there a real team's spec available for the exit check, and can it be added to the corpus, or does it have to stay private?

Phase 3 (collaboration: diff, breaking changes, verify, proxy) is next.
