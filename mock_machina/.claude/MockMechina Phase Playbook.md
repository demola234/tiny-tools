# MockMechina Phase Playbook

Oct 5, 2026 · @Ademola Kolawole

## How to use this playbook

This playbook plans MockMechina one phase at a time, in enough depth to start coding from it. Each phase is planned only after the previous one is reviewed, so later phases can learn from earlier ones. Phases 0 to 2, 4 and 5 are planned below; Phase 3 was planned in conversation and is built; Phases 5 to 9 are placeholders until their turn.

Every phase section answers the same questions: what it delivers and doesn't, which decisions must close first, the flows, what gets built and where, the tools and things we need, the build order, and how we know it's done.

| Phase | Version | Delivers | Status |
| --- | --- | --- | --- |
| 0 | – | Repo, data model, loader, frozen file format, CI | Planned below |
| 1 | v0.1 | init, add, start, switchable states, hot reload | Planned below |
| 2 | v0.2 | Schemas, lint, OpenAPI / Swagger / Postman | Built; release left |
| 3 | v0.3 | Diff, breaking changes, GitHub Action, verify, proxy | Built; manual checks left (`verify` became `diff --live`; recording left out) |
| 4 | v0.4 | MCP server, AI draft and suggest | Built; manual checks left |
| 5 | v0.5 | Latency, faults, fake data, rules, CRUD | Built; release left |
| 6 | v0.6 | WebSocket, SSE, gRPC, webhooks, Graphql | Later |
| 7 | v0.7 | TUI, Docker, releases, HTTPS | Built; release, Docker build and device checks left |
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
| Timing | `latency: 250ms` or `latency: { base, jitter }`, as Go duration strings (changed from `{ ms, jitter }` by Phase 0 Spec P0-01) | 1 (base), 5 (jitter) |
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

### Re-plan for the current format (2026-10-08)

The sections above were written for the old layout (one folder per route, `route.yaml`, `$ref: ../../schemas/x.yaml#/User`). Phase 2 is built on the format that shipped: one file per resource, named routes, and the Phase 5 behaviour fields. Where this re-plan differs from the sections above, this re-plan wins.

**Schemas and references**

```yaml
# .mockmachina/schemas/users.yaml
User:
  type: object
  required: [id, name, email]
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

```yaml
# .mockmachina/routes/users.yaml
list:
  route: GET /users
  request:
    query: { page: { type: integer, minimum: 1 } }
  responses:
    200: UserList
    401: Error
  states:
    success: { body: users.json }
    unauthorized: { status: 401, body: { error: session expired } }
```

- **Names, not paths.** Schema names are unique across `schemas/*.yaml`. A route refers to one by name (`200: UserList`), and schemas refer to each other with `$ref: User`. An inline mapping is also allowed anywhere a name is. This is simpler than relative file paths, and OpenAPI's `#/components/schemas/User` maps to it one-to-one.
- **Dialect:** JSON Schema 2020-12, as in OpenAPI 3.1. Formats are enforced.
- **Route fields:** `request: { params, query, headers, body }` (each a schema, or a map of name to schema for params, query and headers) and `responses: { <status> | default: schema }`.
- **State fields:** `body: generate` builds the body from the response schema, and `validateRequest: false` lets a state accept anything.

**Build order**

| # | Task | Done when | Status |
| --- | --- | --- | --- |
| 1 | `internal/schema`: load `schemas/*.yaml`, resolve name refs, compile with jsonschema v6, errors with file:line and "did you mean" | Bad refs and unknown keywords fail lint with file:line | Done |
| 2 | Route `request` and `responses`; lint: each state's body matches its status's schema, in contract language, root cause only; status coverage both ways | Golden messages for type, required, format, enum, pattern | Done. A documented status without a state is a warning, not an error |
| 3 | Request validation in the server: params, query (text converted to numbers and booleans per schema), headers, JSON body; 400 `invalid_request` with problems; `validateRequest: false`; `start --no-request-validation`; the log names the first problem | One test per part; extra fields allowed; opt-outs work | Done. An empty body reports `body: is required` |
| 4 | `body: generate`, deterministic from the schema (example, enum, format, min/max, required only); every generated body validated | 500 generated bodies across the corpus all validate; same every run | Done. Generation runs at load; a body that doesn't fit its schema asks for an example |
| 5 | `mockmachina import openapi.yaml` (3.0 and 3.1): routes per resource, schemas, one state per response and example, `x-mockmachina-*` round trip, summary output | Petstore 3.0 and 3.1 and the mid-size spec import and lint clean | Done. Corpus: 5 downloaded specs and one written for MockMachina; all import, write and load clean |
| 6 | Swagger 2.0 import, by converting to the same internal form | Swagger Petstore imports and lints clean | Done, converted in memory |
| 7 | `mockmachina export --format openapi` (3.1): states as named examples, Phase 5 fields as `x-mockmachina-*`, sorted keys | Import → export → import is identical for the corpus | Done. Round trip exact for the corpus; MockMachina fields travel as `x-mockmachina`; exports validated against the OpenAPI 3.1 schema in tests |
| 8 | Re-import merge: keeps hand-edited states, `active`, `owners` and `status`; reports operations removed upstream; `--dry-run` | Merge golden | Done. Edits in place keep comments and formatting |
| 9 | Postman: import with schema guessing (inferred schemas flagged in lint) and export with per-state requests and an environment | Collection imports; export opens in Postman | Done; guessed schemas are marked and warned about |
| 10 | `diff` sees schemas: a removed response field, a changed field type or a new required request field is breaking; a new optional field is safe | Rule table tested | Done |
| 11 | `mockmachina docs`: Swagger UI over a live export (`--serve`) or static files (`-o`) | Page lists every route and state offline | Done; Swagger UI 5.33.1 bundled, `just swagger-ui VERSION` updates it |
| 12 | MCP `get_route` shows schemas; lint `--format json|github`, `--only`; exit tests; docs; corpus README | Exit tests green on all three OSes | Done: `TestExitPhase2_RoundTrip`, `_LintType`, `_RequestValidation`; `lint --format`, `--only RESOURCE`; docs in contracts.md and import-export.md |

**Decided (2026-10-08):** our own OpenAPI 3.0/3.1 and Swagger 2.0 reader on go.yaml.in/yaml/v3, not kin-openapi; Swagger UI bundled into the binary. YAML stays in `internal/config`; a new `internal/schema` compiles and validates with jsonschema v6.

**Exit criteria**

- `TestExitPhase2_RoundTrip`: import the mid-size spec, lint clean, export, import again, compare.
- `TestExitPhase2_LintType`: change a field in a body from a number to a string; lint fails with the file, the path in the body and the schema.
- `TestExitPhase2_RequestValidation`: a request missing a required field gets a 400 naming it.


## Phase 4: AI through MCP (v0.4)

### Goal and scope

People already have an AI assistant: Claude Code, Claude Desktop, Cursor, Copilot, ChatGPT. Phase 4 doesn't add another one. It turns MockMachina into an MCP server, so the assistant a person already uses can read the contract, suggest what's missing and write routes and states. The assistant does the thinking. MockMachina does the reading, the writing and the checking, with the same validation as `add` and `lint`. MockMachina never calls a model and never holds an API key.

```text
person ──asks──▶ their assistant ──MCP tools──▶ mockmachina mcp ──▶ .mockmachina/
                 (reasoning)                     (validation, atomic writes)
```

**In scope**

- `mockmachina mcp`: an MCP server over stdio, for the project it finds from `--dir` or the current folder.
- Read tools: `list_routes`, `get_route`, `lint`, `diff`, `diff_live`.
- Write tools: `add_route`, `add_state`, `set_state`. All writes go through `internal/config`, and are validated and written atomically.
- A `generated` field, so anything an assistant writes is marked until a person reviews it.
- Prompts: ready-made tasks the assistant can offer (suggest missing states, draft from a sample response, explain a diff, check the live API).
- `--read-only`, which leaves out the write tools.
- `--print-config CLIENT`, which prints the setup snippet for each assistant.
- Streamable HTTP (`--http`) for assistants that only connect to remote servers, such as ChatGPT. Built last.
- More `lint` rules that suggest without AI: deterministic gaps every user gets.

**Out of scope**

- MockMachina calling a model itself, or a bring-your-own-API-key mode. Revisit only if someone needs suggestions in CI, where there's no assistant.
- Delete and rename tools. An assistant that removes routes is a bigger risk than one that adds them. People do that in the files.
- MCP resources, sampling and elicitation. Tools and prompts work in every client; the rest is uneven.
- A hosted service.

**Done when** a person connects Claude Code to `mockmachina mcp` with one command, asks "add the error states users.get is missing", approves the writes, and `mockmachina lint` passes with the new states marked `generated: true`.

### Flows

**Set up once.**

```text
$ mockmachina mcp --print-config claude-code
claude mcp add mockmachina -- mockmachina mcp --dir /Users/ada/shop/.mockmachina

$ mockmachina mcp --print-config cursor
{ "mcpServers": { "mockmachina": { "command": "mockmachina", "args": ["mcp", "--dir", "/Users/ada/shop/.mockmachina"] } } }
```

`--print-config` always writes an absolute `--dir`, because some clients (Claude Desktop) start servers from `/`.

**Suggest missing states.**

```text
you:       what states is users.get missing?
assistant: [calls get_route users.get, lint]
           users.get only has "found" (200). Apps also need:
           - not_found (404), when the id doesn't exist
           - unauthorized (401), when the session expired
           - server_error (500)
           Add them?
you:       yes
assistant: [calls add_state ×3, you approve each]
           Added 3 states to routes/users.yaml, marked generated: true.
           Review them, then delete the generated lines.
```

**Draft from a sample response.** Paste a JSON response from the real API, and the assistant calls `add_route` with it as the first state.

**Explain a diff, check the live API.** The assistant calls `diff` or `diff_live` and writes the PR comment or the drift summary in plain language.

### Decisions

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| P4-1 | Which MCP library? | The official `github.com/modelcontextprotocol/go-sdk`, pinned (v1.8.0 at planning). Only `internal/mcp` imports it (depguard) | Maintained with the spec; typed tool inputs and outputs with generated JSON Schemas. It needs Go 1.25.0, and so do all its dependencies, so our floor holds |
| P4-2 | Does MockMachina call a model? | No. No API keys, no provider clients | The person's assistant is better and already set up. Nothing to keep up to date per provider, no keys to leak |
| P4-3 | Transport | stdio first; streamable HTTP second, on `127.0.0.1` by default. Any other address needs `--token` (or `MOCKMACHINA_MCP_TOKEN`), checked as a bearer token | stdio covers every local client. HTTP is only for clients that can't start a process |
| P4-4 | How do writes happen? | Only through `internal/config` (`AddRoute`, a new `AddState`, `SetActive`). `internal/mcp` never touches files | One set of rules for the CLI and the assistant. A write that would break the contract is refused before anything is written |
| P4-5 | How are AI writes marked? | Routes and states written through MCP get `generated: true`. A new route also gets `status: draft`. `lint` warns until a person deletes the line | Reviewers see what nobody has checked. It's a plain line in the file, so removing it is the review |
| P4-6 | Are writes on by default? | Yes. Clients ask the person before each tool call. `--read-only` leaves the write tools out. Tools carry MCP hints (`readOnlyHint`, `idempotentHint`) so clients can tell them apart | Writing states is the point of the feature. Teams that want suggestions only have a switch |
| P4-7 | What does a bad tool call return? | A tool result with `isError` and the same message the CLI prints, including "did you mean". Not a protocol error | Assistants read tool results and correct themselves. Protocol errors usually stop them |
| P4-8 | How does `diff_live` get auth headers? | From launch flags (`mockmachina mcp --live-header "Authorization: Bearer $TOKEN"`) in the client's config. The tool has no headers input. Safe methods only, with no way to include writes | Tokens stay out of the chat and the model's context. The assistant can't send writes to a real API |
| P4-9 | What goes on stdout? | Only the protocol. Logs go to stderr | One stray line breaks the stdio connection. A test checks this |
| P4-10 | Package layout | `internal/mcp` (server, tools, prompts) imports `config`, `diff`, `live`, `gitfs`, `model`. `cli` imports `mcp`. Archtest records both | Same shape as `server` and `live`: the CLI wires it, the package does the work |

### Tools

Every tool has a typed input and output. The SDK makes the JSON Schemas from the Go structs, and a golden file holds the whole tool list, so any change to names, descriptions or schemas shows up in review.

| Tool | Input | Output | Built on | Hints |
| --- | --- | --- | --- | --- |
| `list_routes` | `resource?` | `[{id, method, path, summary, status, active, states: [{name, status}]}]` | `config.Load` | read-only |
| `get_route` | `id` | The route with its states, bodies, headers, latency, file and line | `config.Load`, `FindRoute` | read-only |
| `lint` | – | `{errors, warnings, problems: [{severity, file, line, message}]}` | The loader's problems | read-only |
| `diff` | `base?`, `head?` | The `diff --format json` object | `gitfs`, `diff.Compare` | read-only |
| `diff_live` | `url`, `params?` | The same object, for the live API | `live.Check` | read-only |
| `add_route` | `method`, `path`, `name?`, `summary?`, `states: [{name, status?, body?, headers?, latency?}]` | `{id, file, line}` | `config.AddRoute`, `AddState` | write |
| `add_state` | `route`, `name`, `status?`, `body?`, `headers?`, `latency?` | `{route, state, file, line}` | `config.AddState` (new) | write |
| `set_state` | `route`, `state` | `{route, previous, current}` | `config.SetActive` | write, idempotent |

`body` is JSON, written into the route file as YAML flow style when it's short and as a block when it isn't. Large bodies (over 4 KB) go to `routes/<resource>/<route>.<state>.json`, the same way people do it by hand.

### Prompts

| Prompt | Arguments | Asks the assistant to |
| --- | --- | --- |
| `suggest_states` | `route?` | Read the route (or all routes) and lint, propose the missing states apps need (errors, empty, slow), explain each, and write them only after the person agrees |
| `draft_from_sample` | `method`, `path`, `sample` | Turn a real response into a route, with an error state alongside it |
| `explain_diff` | `base?` | Run `diff` and write a short PR comment: what breaks, who owns it, what to do |
| `check_live` | `url` | Run `diff_live` and explain the drift, separating "the backend is wrong" from "the contract is out of date" |

The prompt texts live in `internal/mcp/prompts/*.md`, embedded with `embed`, so they can be read and reviewed as text.

### Lint without AI

Some suggestions need no model. They go in `lint`, as warnings, so everyone gets them:

- A list route (`GET` on a collection) with no empty state.
- A route with a path parameter and no 404 state.
- A write route (`POST`, `PUT`, `PATCH`) with no 4xx state.
- `generated: true` still present (P4-5).

`lint` already warns about routes with no error state. These follow the same pattern and wording.

### Build order

| # | Task | Needs | Size | Done when | Status |
| --- | --- | --- | --- | --- | --- |
| 1 | Add the SDK; depguard limits it to `internal/mcp`; archtest entries | – | S | `go` line unchanged, govulncheck clean, binary size change recorded | Done: v1.8.0, binary 13.8 → 16.5 MB |
| 2 | `generated` field at route and state level: loader, JSON Schema, file-format doc, lint warning | – | S | Fixture and golden for the warning; schema test passes | Done |
| 3 | `config.AddState`: insert a state under `states:` keeping the rest of the file byte for byte | – | M | Goldens for flow and block bodies, large bodies to files, duplicate names refused with a suggestion | Done; `AddRoute` also takes states now, for `add_route` |
| 4 | `internal/mcp` server with `list_routes` and `get_route` | 1 | M | In-memory client tests; tool list golden | Done, with `lint` brought forward from 6 |
| 5 | Write tools with `generated` marking and errors as results | 2, 3, 4 | M | Each write tool: success golden, refusal with "did you mean", file unchanged on refusal | Done, plus `generated` shown by `list_routes` and `get_route` |
| 6 | `lint`, `diff`, `diff_live` tools; `--live-header` | 4 | S | Same output as the CLI's JSON formats; `diff_live` never sends a write | Done; `diff` and the CLI share `config.LoadAt` |
| 7 | Prompts | 4 | S | Prompt list golden; each prompt's text renders with its arguments | Done |
| 8 | `mockmachina mcp` command: `--dir`, `--read-only`, stdio | 5, 6, 7 | M | A real client starts the binary and lists tools; nothing but protocol on stdout | Done |
| 9 | `--print-config` for claude-code, claude-desktop, cursor, vscode | 8 | S | Golden per client, absolute `--dir` | Done; uses the binary's full path, since some assistants start servers without your shell's `PATH` |
| 10 | Lint rules without AI | 2 | S | Fixture and golden per rule | Done |
| 11 | Streamable HTTP: `--http`, loopback default, `--token` | 8 | M | Token required off loopback; ChatGPT connects through a tunnel (manual) | Done: `/mcp`, token as a bearer header or `/mcp/<token>`; DNS-rebinding protection off when a token is set, so tunnels work. ChatGPT connection still to check by hand |
| 12 | Exit scripts, `docs/ai.md`, README, CHANGELOG | 8–11 | S | Exit scripts green on all three OSes | Done: `TestExitPhase4_Stdio`, `TestExitPhase4_ReadOnly`, `docs/ai.md` |

Tasks 2, 3 and 10 don't need the SDK and can start first. Task 11 can slip to a later release without blocking v0.4.

### Tests, exit criteria and risks

**Tests**

| Kind | What it proves |
| --- | --- |
| In-memory client | Each tool, called through `mcp.NewInMemoryTransports`, returns the right output or error |
| Tool and prompt goldens | Names, descriptions and schemas change only on purpose |
| Write safety | A refused write leaves every file byte for byte the same; `--read-only` has no write tools |
| Config edits | `AddState` keeps formatting and other routes untouched (goldens) |
| Live safety | `diff_live` sends only `GET`, `HEAD`, `OPTIONS`, whatever the input says |
| stdout | Running `mcp` over a real pipe writes only JSON-RPC to stdout |
| HTTP | Off loopback without a token, nothing is served |

**Exit criteria as automated tests**

- `exit_phase4_stdio` (`TestExitPhase4_Stdio`): a Go test builds the binary, connects with the SDK's command transport, adds a state, and checks that `lint` passes and the state has `generated: true`.
- `exit_phase4_readonly` (`TestExitPhase4_ReadOnly`): with `--read-only`, the tool list has no write tools and the project is unchanged after every call.
- Manual once: Claude Code and one other client connect with `--print-config` output, and the "users.get is missing states" flow works end to end.

**Exit checklist**

- [x] All 12 build tasks Done
- [ ] Exit tests green on all three OSes (green on macOS; Linux and Windows run in CI)
- [ ] Manual check with two clients
- [ ] v0.4.0 released

**Risks**

| Risk | Effect | Mitigation |
| --- | --- | --- |
| The assistant writes plausible but wrong states | Apps test against a made-up API | Marked `generated: true` and `status: draft`; lint warns until reviewed; `diff` shows them in PRs |
| Tokens leak into the chat | Secrets in model logs | Headers only from launch flags (P4-8) |
| SDK changes its API | Rework | Pinned version; only `internal/mcp` imports it |
| A dependency raises the Go floor | CI fails on the `go` line | Checked at planning; task 1 checks again; pin like `x/text` if needed |
| Assistant and person edit the same file at once | One edit lost | Atomic writes, so a file is never half-written; the last write wins. Revisit if it happens in practice |
| A stray log line on stdout | Client disconnects | stdout reserved for the protocol, with a test (P4-9) |
| ChatGPT's remote MCP rules change | HTTP task slips | Built last, checked at build time, not on the v0.4 critical path |

**Open questions for you**

1. Should writes be on by default (as planned, with the client asking each time), or should `--read-only` be the default?
2. Should a leftover `generated: true` be a warning (fails only with `--strict`), or an error that blocks merging?
3. Does v0.4 need ChatGPT, or can the HTTP transport come in a later release?

## Phase 5: Realistic behaviour (v0.5)

### Goal and scope

Until now, a route answers with whichever state is active, the same way every time. Phase 5 makes the mock behave like a real API:
- **Rules:** the state depends on the request. `id u_404` gets a 404, and a missing token gets a 401.
- **Templates:** bodies echo what was asked for and fill in realistic fake data.
- **Timing and faults:** responses arrive with jitter and sometimes fail the way networks do.
- **Memory:** some routes remember what happened, either through variables or a full in-memory CRUD collection.

App teams can then test real flows (log in, add to cart, see the cart) against the mock, not just single screens.

The file format for all of this was frozen in Phase 0 (P0-01, P0-03, P0-04, P0-10, P0-11, P0-18; spec §4.3–4.4). This phase builds what the format already promises, adapted to the single-file-per-resource layout.

**In scope**

- `mode: active | rules | sequential | random`, `rules: [{ when, state }]`, with every selector and matcher in spec §4.4.
- Templates in bodies and headers: `{{ path.id }}`, `{{ query.page }}`, `{{ body.email }}`, `{{ fake.person.name }}`, `{{ now }}`, `{{ uuid }}`. `template: false` turns them off for a state.
- Fake data with no dependency, in locales `en` and `en_NG`, deterministic from `seed`.
- `seed` and `locale` in `config.yaml`, with `--seed` on `start`. The seed is logged, so a run can be replayed.
- `latency: { base, jitter }`.
- `fault: timeout | reset | truncated`, or `{ type, rate, after }`.
- `set:` on a state, which stores variables that rules and templates can read. `call`, the per-route request count.
- `route: CRUD /path` with `crud: { collection, idField }`: an in-memory collection seeded from `data/<collection>.json`.
- Lint for all of the above, the JSON Schema and docs.

**Out of scope**

- Callbacks and webhooks, which go with WebSocket and SSE (Phase 6).
- Body generation from schemas (`body: generate`), which needs Phase 2.
- A reset endpoint, which comes with the control API (Phase 8). Until then, restarting the server or editing a data file resets state.

**Done when** the Flutter example can sign in (a rule on the password), see a list whose names come from `fake.person.name`, add an item to a CRUD cart and see it in the next GET, and get a reset connection from a `fault: reset` state. All of this runs in widget tests with a fixed seed.

### Flows

**A state picked by the request.**

```yaml
get:
  route: GET /users/{id}
  mode: rules
  rules:
    - when: { path.id: u_404 }
      state: not_found
    - when: { header.authorization: { exists: false } }
      state: unauthorized
  states:
    found:
      body: { id: "{{ path.id }}", name: "{{ fake.person.name }}", joined: "{{ fake.date.past }}" }
    not_found: { status: 404, body: { error: user not found } }
    unauthorized: { status: 401 }
```

When no rule matches, the active state is served. An explicit `X-Mock-State` or `?__state=` still wins over everything, so app tests stay in control.

**A flow with memory.**

```yaml
login:
  route: POST /session
  mode: rules
  rules:
    - when: { body.password: { ne: secret } }
      state: wrong_password
  states:
    ok: { set: { signed_in: true }, body: { token: "{{ uuid }}" } }
    wrong_password: { status: 401 }
me:
  route: GET /me
  mode: rules
  rules:
    - when: { var.signed_in: { exists: false } }
      state: signed_out
  states:
    ok: { body: { name: "{{ fake.person.name }}" } }
    signed_out: { status: 401 }
```

**A full collection.**

```yaml
items:
  route: CRUD /cart/items
  crud: { collection: cart_items }
  states:
    ok: {}
    down: { status: 503, body: { error: unavailable } }
```

| Request | Does |
| --- | --- |
| `GET /cart/items` | The list |
| `GET /cart/items/{id}` | One item, or 404 |
| `POST /cart/items` | Adds the item, gives it an `id` if it has none, and returns it with 201 |
| `PUT` / `PATCH /cart/items/{id}` | Replaces it, or merges the fields given |
| `DELETE /cart/items/{id}` | Removes it and returns 204 |

The collection starts from `data/cart_items.json` (a JSON array) when there is one. A state with no fields means "behave as CRUD"; any other state, chosen by header, rule or `active`, answers as usual. So `X-Mock-State: down` still tests the error screen.

### Decisions

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| P5-1 | Template syntax | `{{ selector }}` using the rule selectors (`path.id`, `query.page`, `header.x`, `body.a.b`, `var.x`, `call`), plus generators (`fake.*`, `uuid`, `now`, `now.unix`, `random.int 1 100`). No logic or loops | One vocabulary for rules and templates. Small enough to parse by hand and give file:line errors for. Logic belongs in rules and states |
| P5-2 | Where templates run | Only inside JSON string values and header values, never on raw bytes. A string that is exactly one `{{ }}` that yields a number or bool becomes that type | Escaping can't be got wrong, so no broken JSON. `"{{ random.int 1 5 }}"` can still be a number |
| P5-3 | Fake data source | Built-in word lists (names, emails, phones, cities, companies, words, dates) for `en` and `en_NG`. No dependency | A few hundred lines of data. Deterministic per seed, with no large dependency and no license review |
| P5-4 | Determinism | `internal/seed`: PCG from `math/rand/v2`, seeded from `seed` (0 = random, logged). Each request derives its own stream from the seed, the route and that route's call number | Same seed, same requests, same answers, even when requests run concurrently |
| P5-5 | `sequential` and `random` modes | `sequential` serves the states in order and stays on the last. `random` picks one from the seeded stream | Matches how people test retries ("fail twice, then succeed") and chaos |
| P5-6 | Precedence | Explicit state (header, query) > mode (rules, sequential, random) > `active` | Tests must always be able to force a state |
| P5-7 | Faults over HTTP | `timeout`: hold the request without answering until the client gives up (the client's deadline, or 5 minutes at most). `reset`: hijack the connection and close it with SO_LINGER 0, so the client sees a reset. `truncated`: announce the full Content-Length, send half the body, then close | These are the three failures apps most often handle badly. `rate` is a 0–1 probability from the seeded stream; `after` delays the fault |
| P5-8 | Variables and call counts | Kept in memory per server, shared by all routes. Route reloads keep them; restarting clears them | Flows survive editing a file mid-test. Phase 8's control API adds a reset |
| P5-9 | CRUD storage | In memory, seeded from `data/<collection>.json`. Edits to that file reset the collection. Ids: `idField` (default `id`); a missing id becomes a random `uuid` string | Predictable, and needs no database. Editing the file is the reset button until Phase 8 |
| P5-10 | `proxy` in `config.yaml` | Stays the plain URL string Phase 3 shipped, not the spec's `{ target, enabled }` | It already shipped, and a URL alone is simpler. Recorded here as a deliberate change from the spec |

### Lint

New errors, each with file and line:
- a rule's `state` that doesn't exist, with "did you mean";
- `mode: rules` without `rules`, or `rules` with another mode (rules alone imply `mode: rules`);
- an unknown selector scope or matcher;
- an operator mapping with more than one operator;
- an invalid regex in `matches`;
- a `path.x` that isn't a parameter of the route;
- a template that doesn't parse, or names an unknown generator;
- an unknown fault type, or a `rate` outside (0, 1];
- `seed` or `locale` out of range;
- a CRUD route without `crud`;
- `crud.collection` naming a data file that isn't a JSON array.

New warning: a rule that can never match because an earlier rule with the same `when` catches it first.

### Build order

Vertical slices. Each one ships working on its own.

| # | Task | Needs | Size | Done when | Status |
| --- | --- | --- | --- | --- | --- |
| 1 | `internal/seed`; `seed`, `locale` in config; `--seed`; seed logged at start | – | S | Same seed gives the same stream; seed 0 logs the one it picked | Done |
| 2 | Rules: loader for `mode`, `rules`, `when` (selectors, matchers, regex compiled at load); lint | – | M | Every selector and matcher has a load test and a problem test | Done. Change: `rules` without `mode` implies `mode: rules` instead of being an error |
| 3 | Rules in the server: request facts (path, query, header, cookie, JSON body), precedence, log shows `(rule 2)` | 2 | M | Table test over every matcher; explicit state beats rules | Done: `internal/match` (fuzzed), call counts kept across reloads, log shows `(rule N)` |
| 4 | `sequential` and `random` modes; `call` | 1, 3 | S | Order and seeded picks are deterministic | Done |
| 5 | Templates: parser, `{{ }}` in JSON strings and headers, typed single expressions, request selectors, `uuid`, `now`, `random.int`; `template: false` | 1 | M | Golden bodies; broken templates fail lint with file:line | Done |
| 6 | Fake data: `fake.*` generators, `en` and `en_NG` | 1, 5 | M | Every generator is deterministic per seed; lint lists the generators | Done |
| 7 | `latency: { base, jitter }` | 1 | S | Seeded jitter stays within base ± jitter (synctest) | Done |
| 8 | Faults: `timeout`, `reset`, `truncated`, `rate`, `after` | 1 | M | A real client sees a reset, a cut body and a timeout; rate ≈ p over 10,000 seeded requests | Done |
| 9 | `set` and `var`; variables shared and kept across reloads | 3, 5 | S | Login → me flow test | Done |
| 10 | CRUD: `route: CRUD`, collection from `data/`, ids, all five operations, states still win | 5 | L | Table test for every operation; data file edit resets it | Done; `crud` is optional (collection from the path, id field `id`) |
| 11 | MCP tools and `get_route` show modes, rules and faults; `diff` reports rule and mode changes | 2–10 | S | Goldens updated | Done |
| 12 | Exit tests, Flutter example flow, docs (`docs/behaviour.md`), schema, changelog | all | M | Exit criteria pass on three OSes | Done: `TestExitPhase5_Flow` (golden transcript, seed 42), `TestExitPhase5_Faults`, Flutter sign-in, cart and checkout tests, docs in file-format.md |

Tasks 2–4 can start without 1 if `random` waits. Tasks 7 and 8 are independent of rules and templates.

### Tests, exit criteria and risks

**Tests**

| Kind | What it proves |
| --- | --- |
| Matcher table | Every matcher against strings, numbers, missing values and bad types |
| Precedence | Header > query > rules or mode > active, for every mode |
| Template goldens | Request echo, typed values, escaping of quotes and newlines, `template: false` |
| Determinism | Same seed and same requests give the same bodies, concurrently too (`-race`) |
| Fault behaviour | Real `net/http` clients see `ECONNRESET`, `unexpected EOF` and a deadline |
| CRUD table | Each operation on an empty, a seeded and a missing item; ids; PATCH merges; 404s |
| Fuzzing | The template parser and rule matcher never panic on any input |

**Exit criteria as automated tests**

- `TestExitPhase5_Flow`: sign in with a wrong password (401), then the right one, then `GET /me` (200 with a fake name); add to a CRUD cart and list it, with seed 42 and golden bodies.
- `TestExitPhase5_Faults`: a `reset` state gives the client a connection reset, and a `truncated` state gives an unexpected EOF.
- The Flutter example's widget tests cover sign-in and the cart.

**Risks**

| Risk | Effect | Mitigation |
| --- | --- | --- |
| Templates grow into a language | Hard to read and lint | No logic in templates (P5-1); behaviour lives in rules and states |
| Faults behave differently per OS | Flaky tests on Windows | Exit tests check the client-side error class, not the message |
| Randomness makes tests flaky | Teams stop trusting the mock | Seeded everywhere (P5-4); seed logged; `--seed` to replay |
| CRUD state confuses a team sharing one mock | "Who deleted my item?" | Documented as per-server; data file edit resets it; Phase 8 adds a reset endpoint |
| Rules slow every request | Latency | Matchers compiled at load; request body read once, at most 1 MiB, only when a rule or template uses `body.*` |

**Answered (2026-10-07):** template syntax uses rule selectors (P5-1); fake data comes from built-in `en` and `en_NG` lists (P5-3); CRUD data stays in memory (P5-9).

## Phase 7: Packaging and a terminal UI (v0.7)

### Goal and scope

People get MockMachina without a Go toolchain, run it where they already run things (a container, a phone that needs HTTPS), and control it from one screen. After this phase:
- a release is a git tag that builds everything;
- Docker users have an image;
- iOS and Android apps can talk to the mock over trusted HTTPS;
- the terminal shows routes, states and requests live, and switches states with a key.

**In scope**

- **Releases:** a GitHub Actions workflow on `mock_machina/v*` tags, built with GoReleaser:
  - binaries for macOS, Linux and Windows on amd64 and arm64;
  - checksums and an SBOM;
  - the version stamped into `--version`;
  - an install script for macOS and Linux.
- **Docker:** a multi-arch image on `ghcr.io/demola234/mockmachina`. It's static and non-root, serves `/mock/.mockmachina` on `0.0.0.0:4001`, and polls for changes, since file events don't cross bind mounts.
- **HTTPS:** `start --https` serves TLS with a certificate from a local CA that MockMachina creates once. The certificate is valid for `localhost`, `127.0.0.1`, the machine's LAN addresses and the emulator aliases. `mockmachina cert` prints or installs the CA, with steps for iOS, Android and desktop. `--tls-cert` and `--tls-key` use your own certificate instead.
- **Terminal UI:** a full-screen view of the running mock. Details below.
- Docs: install, Docker and HTTPS pages; the mobile setup page gains HTTPS.

**Out of scope**

- **Package-manager publishing beyond what's chosen below:** each channel needs its own repository or account.
- **Signing binaries for macOS and Windows:** needs paid certificates. Revisit when there are users.
- **Editing routes in the UI:** states are switched, not edited. Editing stays in files, the editor and MCP.

**Done when:**
- a tag on a fork produces binaries, checksums and an image;
- `docker run -v $PWD/.mockmachina:/mock/.mockmachina -p 4001:4001 ghcr.io/demola234/mockmachina` serves the example project;
- the Flutter example passes over HTTPS on the iOS Simulator after `mockmachina cert --install`;
- the UI switches a state, and the next request gets it.

### The terminal UI

```text
┌ mockmachina · shop · http://127.0.0.1:4001 · seed 42 ───────────────────────┐
│ Routes                         │ Requests                                    │
│ ▸ users.list  GET /users       │ 12:01:03  GET  /users        200  success   │
│     ● success  200             │ 12:01:05  GET  /users/u_404  404  rule 1    │
│     ○ empty    200             │ 12:01:09  POST /session      400  invalid   │
│     ○ unauthorized 401         │                                              │
│   users.get   GET /users/{id}  │                                              │
│   session.create POST /session │                                              │
├────────────────────────────────┴────────────────────────────────────────────┤
│ ↑↓ move · enter set default state · / filter · r reload · l lint · q quit   │
└──────────────────────────────────────────────────────────────────────────────┘
```

| Part | Does |
| --- | --- |
| Routes | Every route, with its states. The active state is marked. `enter` makes the selected state the default, through `state set`, so the file changes as it does from the CLI |
| Requests | The live log, newest last. Selecting a request shows its status, state, rule, latency and the first lines of the body |
| Header | Project, address, seed and proxy target |
| Problems | Reload errors and lint warnings appear in a bar; `l` lists them |

It's the same server as `start`, with the same flags. Only the screen differs.

### Decisions

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| P7-1 | UI library | Bubble Tea v2, pinned to 2.0.9, the last release on Go 1.25 (2.0.10 needs Go 1.26). Lip Gloss is already in through Fang | Charm's stack is what Fang uses. Pinned, as `x/text` was, so the Go floor holds |
| P7-2 | UI architecture | `internal/tui` gets the project, a request stream and a `SetActive` function. It doesn't know about HTTP. Its tests drive the model with messages and compare rendered frames to goldens | Testable without a terminal; the server stays unaware of the UI |
| P7-3 | Release tooling | A small tested tool in the module (`tools/release`, logic in `internal/release`): cross-compile, archive, checksums, Homebrew formula, Scoop manifest, release notes from the changelog. The workflow uploads to the `mock_machina/vX.Y.Z` release | GoReleaser's open-source edition can't use monorepo tag prefixes (a Pro feature) and would create plain `vX.Y.Z` tags that collide with other projects in the repository |
| P7-4 | Local CA | Created on first `--https`, stored in the user config folder (`os.UserConfigDir()/mockmachina/ca`), ECDSA P-256, 10 years for the CA and 397 days for leaf certificates (the browser limit). Leaf certificates are made per start for the current addresses | Same approach as mkcert. Nothing secret lives in the repository |
| P7-5 | HTTPS and HTTP together | `--https` serves TLS only, on the same port. Mobile config switches the scheme | Two ports confuse people more than they help |
| P7-6 | Docker defaults | The image sets the host to `0.0.0.0`, the directory to `/mock/.mockmachina` and polling at 500 ms. Runs as UID 65532 (distroless nonroot) | Works with `-v` and `-p` and nothing else |

**Answered (2026-10-08):** `mockmachina tui` as its own command (P7-UI); `--https` with a local CA plus `--tls-cert/--tls-key`; publish to GitHub Releases, ghcr.io, a Homebrew tap (`demola234/homebrew-tap`) and a Scoop bucket (`demola234/scoop-bucket`). The tap and bucket steps run only when their token secrets are set.

### Build order

| # | Task | Done when | Status |
| --- | --- | --- | --- |
| 1 | `internal/release`: cross-compile, archives, checksums, formula and manifest rendering, notes from the changelog; `tools/release`; `just release-dry` | A dry run builds 6 archives, checksums, formula and manifest locally | Done; own tool because GoReleaser OSS has no monorepo tag prefixes; archives are reproducible |
| 2 | Release workflow on `mock_machina/v*` tags, with SBOM; install script | Workflow lints with actionlint; install script tested against a local archive | Done; install script tested against an httptest server, refuses a checksum mismatch |
| 3 | Dockerfile (multi-stage, distroless static, non-root) and image publishing in the release workflow | `docker run` on the example serves `/users` | Done; built and run under Colima (arm64, 29.8 MB, UID 65532): serves the storefront example, `state set` in the container and edits on the host apply live, `lint` runs read-only. Multi-arch push is checked on the first release |
| 4 | `internal/certs`: local CA, leaf certificates, storage, tests | Leaf verifies against the CA for every requested name | Done |
| 5 | `start --https`, `--tls-cert/--tls-key`, `mockmachina cert` (print, `--install`) | An HTTPS client trusting the CA gets `/users` | Done; `runStart` split into `openSession` and `session.run` so the UI can reuse it |
| 6 | `internal/tui` model: routes pane, states, requests pane, keys | Frame goldens for each view; key handling tests | Done. Styled with Lip Gloss (light and dark palettes from the terminal background) and animated from one clock: breathing live dot, request-rate sparkline, new requests flash and fade, spinner then ✓ toast on a state switch, pop on the new active state, blinking filter cursor. Ticks every 50 ms while something moves, 200 ms otherwise. Layout goldens are ANSI-stripped; one colored golden locks the look. Request details show method, path, status, route:state and note (latency and body would need the server to report them) |
| 7 | Wire the UI into the server: request stream, state switching, reload and problems | Switching a state in the UI changes the next response (test) | Done: `mockmachina tui` shares start's flags and session; Enter calls `config.SetActive`, reloads and problems stream to the screen; end-to-end test switches a state through the keyboard and checks the next response. Later: `start` opens the screen in a terminal (`--plain` for log lines), a welcome card, and a command launcher for bare `mockmachina` |
| 8 | Docs: install, Docker, HTTPS; mobile setup over HTTPS; changelog | Pages exist and examples run | Done: docs/install.md (Homebrew, Scoop, script, Go, Docker and Compose), docs/https.md (local CA, every device, Flutter on Android, Docker, removal, troubleshooting), HTTPS in mobile setup, quickstart, README, changelog |
