---
tags: [mockmachina, phase-0, spec, go, tdd]
date: 2026-10-06
phase: 0
status: decisions closed; building
---

# Phase 0 Spec

The complete build specification for Phase 0. It covers every package, file, exported identifier, test, fixture, config file and CI step, in build order. It turns [[MockMechina Phase Playbook]] Phase 0 into something you can code from without making design decisions along the way.

- The **why** is in the playbook and [[MockMachina System Design]].
- The **how we write Go** is in `docs/engineering.md`.
- The **decisions** are in `docs/decisions/` (ADRs 001–007).
- [[Phase 0 Build Guide]] stays as the gentle walkthrough. Where they differ, this spec wins.

All 18 decisions in §1 were agreed on 2026-10-06, each as recommended.

---

## 1. Decisions to close first

Writing the spec at this level turned up gaps the playbook left open. Every one of them affects the frozen file format or a public behaviour, so they're cheap to decide now and expensive later. Mark each **Agreed** or write your alternative.

| # | Question | Recommendation | Why | Status |
| --- | --- | --- | --- | --- |
| P0-01 | How are durations written in route files? | Go duration strings everywhere: `latency: 250ms`, `interval: 5s`. Long form: `latency: { base: 250ms, jitter: 50ms }`. **Replaces the playbook's `latency: { ms, jitter }`** | One rule for every timed field (latency, intervals, delays, fault timing); units are visible; `time.ParseDuration` parses it | Agreed 2026-10-06 |
| P0-02 | Is `owners.backend` one handle or a list? | A list: `owners: { backend: [ademola], frontend: [ade-fe] }` | Teams share endpoints; changing scalar to list later is a format break | Agreed 2026-10-06 |
| P0-03 | What is the grammar for rule conditions (`when`)? | A mapping of selector → value; all must match. A plain value means equals; an operator mapping means `{op: value}`. Details in §4.4 | It must be frozen now even though rules only run in Phase 5 | Agreed 2026-10-06 |
| P0-04 | What shape is `fault`? | One field for all protocols: a string shorthand (`fault: reset`) or `{ type, rate, after }`. The allowed types depend on the protocol | Covers HTTP (`timeout`, `reset`, `truncated`) and streams (`drop`, `close`, `delay`, `duplicate`); `rate` gives Phase 5's failure rates without a new field | Agreed 2026-10-06 |
| P0-05 | What does a scalar `body` mean? | A scalar is always a file path relative to the route folder, except the reserved word `generate`. A mapping or sequence is inline JSON. A literal text body needs a file. If `body` is omitted, `<state>.json` is used when it exists; otherwise there is no body, with a **warning** unless the status is 204 or 304 | No guessing about whether `ok` is a file or text; typos in implicit files surface as warnings | Agreed 2026-10-06 |
| P0-06 | Are YAML anchors and merge keys allowed? | Aliases (`*name`) yes; merge keys (`<<:`) rejected with a hint | Aliases resolve cleanly in the node tree; merge keys complicate line numbers and unknown-field checks | Agreed 2026-10-06 |
| P0-07 | Can state names be YAML special words? | No: reserve `true`, `false`, `null`, `yes`, `no`, `on`, `off`, `y`, `n` | `active: no` would be read as a boolean by some YAML tools | Agreed 2026-10-06 |
| P0-08 | What exit code do usage errors get? | 2 for bad flags, arguments or unknown commands; 1 for problems found or failure to run; 0 for success | Unix convention; scripts can tell "you called it wrong" from "your project is wrong" | Agreed 2026-10-06 |
| P0-09 | Where does `.mockmachina` get found? | Search upward from the working directory, like git. `--dir` overrides | Works from any subfolder of an app or monorepo | Agreed 2026-10-06 |
| P0-10 | What type is `seed`? | Unsigned 64-bit; `0` means pick a random seed at start and log it | Matches the PCG generator; negative seeds have no meaning | Agreed 2026-10-06 |
| P0-11 | What is the default `locale`? | `en`. The playbook's `en_NG` remains the example in docs | A neutral default; Nigerian teams set `en_NG` | Agreed 2026-10-06 |
| P0-12 | How are problems identified in output? | Every problem has a stable code (`unknown-field`). Warnings print with a `warning:` prefix; errors print as in the playbook | Codes feed `--format json` (Phase 2), docs and tests; the prefix tells warnings apart in plain output | Agreed 2026-10-06 |
| P0-13 | What does the loader return? | `(*model.Project, config.Problems, error)`. `error` only when loading couldn't run at all (folder unreadable). The project is returned even with error-level problems, so `lint` and the dashboard can show everything | Separates "your files are wrong" from "MockMachina couldn't read them" | Agreed 2026-10-06 |
| P0-14 | Does `config.yaml` get a published schema too? | Yes: `schema/config.schema.json` next to `route.schema.json` | Same editor help for the other file people edit by hand | Agreed 2026-10-06 |
| P0-15 | How do version and commit get into the binary? | `internal/buildinfo` reads Go's embedded build info; `-ldflags` may override the version for releases | Go already embeds the commit and dirty flag; `go install …@v0.1.0` then reports the right version with no ldflags | Agreed 2026-10-06 |
| P0-16 | Where do shared test fixtures live? | Module-root `testdata/` plus an `internal/testkit` package for golden files, fixture paths and in-memory projects | Fixtures are shared by `config`, `cli` scripts and the schema tests | Agreed 2026-10-06 |
| P0-17 | Which route fields can carry `x-` extensions? | Route level and state level. They're kept verbatim and written back unchanged | Enough for OpenAPI operation and response extensions in Phase 2 | Agreed 2026-10-06 |
| P0-18 | Where can rules read from? | Scopes `path`, `query`, `header`, `cookie`, `body`, `var`, plus `call` (the nth call to this route) | `cookie` is common for session-based apps and costs nothing to add now | Agreed 2026-10-06 |

---

## 2. Package map for Phase 0

| Package | Job | May import | Task |
| --- | --- | --- | --- |
| `cmd/mockmachina` | Signals, build info, `os.Exit` | `cli`, `buildinfo` | 3 |
| `internal/cli` | Command tree, flags, output, exit codes | `config`, `model`, `buildinfo`, cobra, fang | 3, 11 |
| `internal/buildinfo` | Version, commit and dirty flag | stdlib | 3 |
| `internal/config` | Load, check and write `.mockmachina/`. The only package that knows file names, YAML or disk | `model`, yaml | 6–10 |
| `internal/model` | Plain data types for the file format | stdlib | 5 |
| `internal/clock` | `Clock` interface and real clock | stdlib | 12 |
| `internal/seed` | Deterministic randomness | stdlib | 12 |
| `internal/testkit` | Test helpers: golden files, fixture paths, in-memory projects | stdlib, go-cmp | 4 |
| `internal/archtest` | The import-rule test (no production code) | stdlib (tests only) | 13 |

Test files may additionally import `testkit`, go-cmp, testscript and jsonschema. **`testkit` may never be imported by a non-test file**; the arch test checks this.

---

## 3. Repository files

### 3.1 Tree at the end of Phase 0

```text
mock_machina/
├── .editorconfig
├── .gitattributes
├── .golangci.yml
├── CHANGELOG.md                 (created empty with "Unreleased")
├── CONTRIBUTING.md
├── LICENSE                      Apache-2.0
├── README.md
├── go.mod / go.sum
├── justfile
├── cmd/mockmachina/main.go
├── internal/
│   ├── archtest/{doc.go, imports_test.go}
│   ├── buildinfo/{doc.go, buildinfo.go, buildinfo_test.go}
│   ├── cli/{doc.go, run.go, root.go, lint.go, find.go, *_test.go, script_test.go}
│   ├── clock/{doc.go, clock.go, clock_test.go}
│   ├── config/{doc.go, paths.go, problem.go, suggest.go, load.go, decode_*.go, fields.go,
│   │           checks.go, check_*.go, write.go, marshal.go, *_test.go}
│   ├── model/{doc.go, project.go, route.go, state.go, rule.go, enums.go, names.go,
│   │          grpc.go, duration.go, source.go, *_test.go}
│   ├── seed/{doc.go, seed.go, seed_test.go}
│   └── testkit/{doc.go, golden.go, fixture.go, mapfs.go, testkit_test.go}
├── schema/{route.schema.json, config.schema.json}
├── testdata/
│   ├── projects/{minimal, shop, broken, broken-<code>…}/.mockmachina/…
│   ├── golden/…
│   └── script/*.txtar
└── docs/{engineering.md, file-format.md, decisions/…}
```

CI lives one level up, at `tiny-tools/.github/workflows/mock_machina.yml`.

### 3.2 `go.mod` at the end of Phase 0

```text
module github.com/demola234/tiny-tools/mock_machina

go 1.25.0

require (
	charm.land/fang/v2 v2.0.1
	github.com/spf13/cobra v1.10.2
	go.yaml.in/yaml/v3 v3.0.5
	// test only:
	github.com/google/go-cmp v0.7.0
	github.com/rogpeppe/go-internal v1.16.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
)
```

(`go mod tidy` arranges the blocks itself; the comment above is for reading only.) There is deliberately no `tool` block: tool dependencies join the module graph and can raise our `go` line (govulncheck v1.8.0 requires 1.26). Tools run with pinned `go run pkg@version` from the justfile. `golang.org/x/term` and `charm.land/log/v2` come in Phase 1, when something uses them.

### 3.3 `justfile`

*As built in task 2: tools run through pinned `go run`, `lint` includes actionlint, and `ci` ends with `--help` until task 3 switches it to `--version`. The file in the repo is authoritative; the block below is the original plan.*

```make
set windows-shell := ["pwsh", "-NoProfile", "-Command"]

golangci_version := "v2.14.0"

# list recipes
default:
    @just --list

# install golangci-lint at the pinned version
tools:
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{golangci_version}}

# format code
fmt:
    golangci-lint fmt

# run every test with the race detector, in random order
test *args:
    go test -race -shuffle=on -count=1 {{args}} ./...

# re-run one package's tests on every save, e.g. `just tdd config`
tdd pkg:
    go tool gotestsum --watch --format testname -- ./internal/{{pkg}}/...

# rewrite golden files after an intended change (review the diff!)
update-golden:
    go test ./... -update

# static checks
lint:
    go mod tidy -diff
    go vet ./...
    golangci-lint run

# vulnerability scan
vuln:
    go tool govulncheck ./...

# coverage per package
cover:
    go test -coverprofile=cover.out ./...
    go tool cover -func=cover.out

# build the binary into ./bin
build:
    go build -o bin/ ./cmd/mockmachina

# everything CI runs
ci: lint test vuln build
    ./bin/mockmachina --version
```

`go install …@version` for golangci-lint is the fallback install; Homebrew or the upstream install script also work, as long as `golangci-lint --version` reports v2.14.0.

### 3.4 `.golangci.yml`

```yaml
version: "2"

run:
  go: "1.25"

linters:
  default: standard        # errcheck, govet, ineffassign, staticcheck, unused
  enable:
    - bodyclose
    - contextcheck
    - depguard
    - errname
    - errorlint
    - exhaustive
    - forbidigo
    - gochecknoinits
    - gocognit
    - gocritic
    - gosec
    - intrange
    - misspell
    - modernize
    - nestif
    - nilerr
    - noctx
    - nolintlint
    - perfsprint
    - prealloc
    - revive
    - testpackage
    - thelper
    - tparallel
    - unconvert
    - unparam
    - usestdlibvars
  settings:
    depguard:
      rules:
        no-old-libs:
          deny:
            - pkg: math/rand$
              desc: use internal/seed (math/rand/v2 underneath)
            - pkg: gopkg.in/yaml.v3
              desc: use go.yaml.in/yaml/v3 (ADR 003)
            - pkg: github.com/pkg/errors
              desc: use the standard errors package
            - pkg: github.com/stretchr/testify
              desc: use the standard library and go-cmp (engineering.md §10)
        yaml-only-in-config:
          files: ["$all", "!**/internal/config/**"]
          deny:
            - pkg: go.yaml.in/yaml
              desc: only internal/config reads or writes YAML
        fang-only-in-run:
          files: ["$all", "!**/internal/cli/run.go"]
          deny:
            - pkg: charm.land/fang
              desc: Fang is called only in cli.Run (ADR 004)
    exhaustive:
      default-signifies-exhaustive: true
    gocognit:
      min-complexity: 15   # engineering.md §15
    nestif:
      min-complexity: 4
    forbidigo:
      analyze-types: true
      forbid:
        - pattern: ^(fmt\.Print.*|print|println)$
          msg: write through the command's writers (engineering.md §8)
        - pattern: ^time\.(Now|Since|Until|Sleep|After|AfterFunc|NewTimer|NewTicker|Tick)$
          msg: use internal/clock (engineering.md §7)
        - pattern: ^os\.Exit$
          msg: only main may exit; return an error instead
        - pattern: ^os\.(Chdir|Getwd)$
          msg: take the directory as a parameter
    gosec:
      excludes:
        - G304   # file paths come from fs.FS (validated) or tests
    misspell:
      locale: US
    nolintlint:
      require-explanation: true
      require-specific: true
    revive:
      rules:
        - name: exported
        - name: package-comments
        - name: var-naming
        - name: receiver-naming
        - name: error-strings
        - name: error-naming
        - name: unused-receiver
        - name: context-as-argument
        - name: early-return
        - name: indent-error-flow
  exclusions:
    presets: [std-error-handling, common-false-positives]
    rules:
      - path: cmd/mockmachina/main\.go
        linters: [forbidigo]
        text: "os\\.(Exit|Getwd)"
      - path: internal/clock/
        linters: [forbidigo]
        text: "time\\."
      - path: internal/cli/find\.go
        linters: [forbidigo]
        text: "os\\.Getwd"
      - path: internal/cli/script_test\.go
        linters: [forbidigo]
        text: "os\\.Exit"

formatters:
  enable: [gofumpt, goimports]
  settings:
    goimports:
      local-prefixes: [github.com/demola234/tiny-tools/mock_machina]
```

**Note on `os.Getwd`:** the working directory is read once in `cli/find.go` (the upward search, P0-09) and passed down from there. Everything below `cli` receives directories as parameters.

**Note on `time.` in tests:** synctest tests use `time.Now` and `time.Sleep` *inside* the bubble on purpose, so `_test.go` files under `clock` are excluded by the same path rule. If another package's test needs it, add a path exclusion with a reason rather than a `//nolint`.

### 3.5 `.gitattributes`, `.editorconfig`

```text
# .gitattributes
* text=auto
*.go text eol=lf
testdata/** text eol=lf
testdata/**/crlf-* text eol=crlf
*.json text eol=lf
```

The `crlf-*` rule exists for the one test fixture that checks we keep CRLF files intact (§10, `TestSetActive_KeepsCRLF`).

```ini
# .editorconfig
root = true
[*]
end_of_line = lf
insert_final_newline = true
charset = utf-8
trim_trailing_whitespace = true
[*.go]
indent_style = tab
[*.{yaml,yml,json,md}]
indent_style = space
indent_size = 2
[*.md]
trim_trailing_whitespace = false
```

### 3.6 CI: `tiny-tools/.github/workflows/mock_machina.yml`

*As built in task 2: the lint job runs on stable Go (govulncheck needs 1.26; analysis still targets 1.25 through vet's stdversion check and golangci's `run.go`), runs govulncheck through `go run`, and adds an actionlint step. Dependabot is configured from the start. The test jobs end with `--help` until task 3. The workflow in the repo is authoritative.*

```yaml
name: mock_machina
on:
  push:
    branches: [main]
    paths: ["mock_machina/**", ".github/workflows/mock_machina.yml"]
  pull_request:
    paths: ["mock_machina/**", ".github/workflows/mock_machina.yml"]
permissions:
  contents: read
concurrency:
  group: mock_machina-${{ github.ref }}
  cancel-in-progress: true
defaults:
  run:
    working-directory: mock_machina

jobs:
  lint:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: mock_machina/go.mod
          cache-dependency-path: mock_machina/go.sum
      - run: go mod tidy -diff
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.14.0
          working-directory: mock_machina
      - run: go tool govulncheck ./...

  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go: ["go.mod", "stable"]
    runs-on: ${{ matrix.os }}
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: ${{ matrix.go == 'go.mod' && 'mock_machina/go.mod' || '' }}
          go-version: ${{ matrix.go == 'stable' && 'stable' || '' }}
          cache-dependency-path: mock_machina/go.sum
      - run: go test -race -shuffle=on -count=1 -cover ./...
      - run: go build -o bin/ ./cmd/mockmachina
      - run: ./bin/mockmachina --version
        shell: bash
```

**Security practice:** once the workflow works, pin every action to a full commit SHA with the tag in a comment (`actions/checkout@<sha> # v7.0.1`), and add `.github/dependabot.yml` for `github-actions` and `gomod` (weekly, path `/mock_machina` for gomod). Tags can be moved by whoever controls the action; SHAs can't.

**Windows and `-race`:** the race detector needs cgo, and GitHub's Windows images include a C compiler, so it should work. If it doesn't, drop `-race` on Windows only, with a comment linking the failing run. Don't drop it everywhere.

**Proving CI can fail (task 2):** push a commit containing `t.Fatal("ci smoke")`, confirm all six test jobs fail, then revert it.

### 3.7 `README.md`, `CONTRIBUTING.md`, `LICENSE`, `CHANGELOG.md`

- **README:** what MockMachina is (two sentences), status ("pre-release, Phase 0"), how to build from source, and links to `docs/`. No usage docs yet; they come with v0.1.
- **CONTRIBUTING:** prerequisites (Go 1.25+, just, golangci-lint v2.14.0), `just ci`, the TDD loop (link ADR 007), how to write an ADR, commit format, and how to update goldens safely.
- **LICENSE:** the standard Apache-2.0 text, unmodified. **NOTICE:** "MockMachina / Copyright 2026 Ademola Kolawole", as Apache-2.0 intends.
- **CHANGELOG:** a Keep a Changelog header and an `## [Unreleased]` section.
- The tiny-tools root `README.MD` gets its Mock-Machina line updated to the new name and a link to `mock_machina/README.md`.

---

## 4. The file format (what gets frozen)

This is the draft of `docs/file-format.md`. It assumes the §1 recommendations. Task 14 turns it into the published doc.

### 4.1 Project folder

```text
.mockmachina/
├── config.yaml          optional
├── schemas/*.yaml       Phase 2
├── protos/**/*.proto    Phase 6
├── data/*.json          Phase 5 (CRUD seed data)
├── .run/                runtime files, gitignored, ignored by the loader (from Phase 1)
└── routes/<id>/
    ├── route.yaml
    └── <state>.json …   bodies
```

The loader ignores any entry whose name starts with `.`, and (in `routes/`) anything that isn't a folder. Phase 0 doesn't read `schemas/`, `protos/` or `data/`, but it does check that every file the routes reference there exists.

### 4.2 `config.yaml`

```yaml
version: 1
ports: { mock: 4001, grpc: 4002, control: 4000 }
host: 127.0.0.1
seed: 0
locale: en
proxy: { target: "", enabled: false }
ai: { enabled: false }
```

| Field | Type | Default | Checks |
| --- | --- | --- | --- |
| `version` | int | 1 | Must be ≤ the supported version (`config-version`) |
| `ports.mock` / `grpc` / `control` | int | 4001 / 4002 / 4000 | 1–65535, all different (`invalid-port`) |
| `host` | string | `127.0.0.1` | An IP address or `localhost` (`invalid-host`) |
| `seed` | uint64 | 0 | Not negative (`invalid-seed`) |
| `locale` | string | `en` | Matches `^[a-z]{2}(_[A-Z]{2})?$` (`invalid-locale`) |
| `proxy.target` | string | `""` | Empty, or an absolute `http`/`https` URL (`invalid-url`) |
| `proxy.enabled` | bool | false | `enabled: true` needs a target (`missing-field`) |
| `ai.enabled` | bool | false | |

### 4.3 `route.yaml`, complete HTTP example

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/schema/route.schema.json
id: users.list
method: GET
path: /users
summary: List users, newest first
status: agreed
group: users
owners:
  backend: [ademola]
  frontend: [ada]

request:
  query:
    page: { type: integer, minimum: 1 }
  headers:
    Authorization: { type: string, pattern: "^Bearer " }
responses:
  200: { $ref: "../../schemas/user.yaml#/UserList" }
  401: { $ref: "../../schemas/error.yaml#/Error" }

active: success
mode: rules
rules:
  - when: { header.authorization: { exists: false } }
    state: unauthorized
  - when: { query.page: { gt: 3 } }
    state: empty

states:
  success:
    status: 200                 # default 200
    body: success.json          # default <state>.json when omitted
    headers: { Cache-Control: no-store }
    latency: 120ms
  empty:
    body: { users: [], nextPage: null }
  slow:
    latency: { base: 2s, jitter: 500ms }
  unauthorized:
    status: 401
    body: unauthorized.json
  flaky:
    status: 200
    fault: { type: reset, rate: 0.2 }
  generated:
    body: generate              # Phase 2: built from the 200 schema
x-team-notes: keep in sync with the web app
```

### 4.4 Field reference

**Route fields.** Order in the file is free; the canonical writer uses this order.

| Field | Type | Required | Applies to | Notes |
| --- | --- | --- | --- | --- |
| `id` | string | yes | all | `^[a-z0-9][a-z0-9.-]*$`; equals the folder name |
| `protocol` | `http`·`ws`·`sse`·`grpc` | no | all | Default `http` |
| `method` | `GET`·`POST`·`PUT`·`PATCH`·`DELETE`·`HEAD`·`OPTIONS`·`CRUD` | http | http | Uppercase only |
| `path` | string | http, ws, sse | http, ws, sse | Starts with `/`; `{name}` params; names unique; no `?` or `#` |
| `service`, `rpc` | string | grpc | grpc | `service` is fully qualified (`shop.v1.OrderService`) |
| `summary` | string | yes | all | One line, ≤ 120 characters |
| `status` | `draft`·`agreed`·`implemented`·`deprecated` | yes | all | |
| `group` | string | no | all | |
| `owners` | `{backend: [..], frontend: [..]}` | no | all | GitHub handles without `@` |
| `request` | `{params, query, headers: {name: schema}, body: schema, example}` | no | http | Stored raw until Phase 2 |
| `responses` | `{<status or "default">: schema}` | no | http | Keys 100–599 or `default` |
| `active` | state name | yes | all | Must exist in `states` |
| `mode` | `active`·`rules`·`sequential`·`random` | no | all | Default `active` |
| `rules` | list of `{when, state}` | if `mode: rules` | all | Each `state` must exist |
| `serve` | `auto`·`mock`·`proxy` | no | http | Default `auto` |
| `crud` | `{collection, idField}` | if `method: CRUD` | http | `idField` default `id`; `collection` names `data/<collection>.json` if present |
| `states` | ordered map name → state | yes, at least one | all | Names `^[a-z][a-z0-9_]*$`, not YAML words (P0-07) |
| `generated` | bool | no | all | Set by AI tools (Phase 4) |
| `x-*` | anything | no | all | Kept verbatim |

**State fields.** Fields outside a state's protocol are an error (`field-not-for-protocol`), so `code:` on an HTTP state is caught.

| Group | Field | Type | Protocols |
| --- | --- | --- | --- |
| Response | `status` | int 100–599, default 200 | http |
| | `headers` | map string → string | http |
| | `body` | file path, `generate`, or inline mapping/sequence | http, grpc |
| | `template` | bool, default true | all |
| | `validateRequest` | bool, default true | http |
| Timing | `latency` | duration, or `{base, jitter}` | all |
| Faults | `fault` | type string, or `{type, rate, after}`. `rate` in (0, 1], default 1; `after` is a duration | all (types per protocol, below) |
| Side effects | `set` | map var name → value | all |
| | `callbacks` | list of `{url, method, headers, body, delay}` | all |
| Streams | `onConnect` | list of frames | ws, sse |
| | `on` | list of `{when, send: [frames]}` | ws |
| | `every` | list of `{interval, send: frame, times}` | ws, sse |
| | `reject` | `{status, body}` | ws, sse |
| gRPC | `code` | name (`NOT_FOUND`) or number 0–16, default `OK` | grpc |
| | `message` | string | grpc |
| | `metadata`, `trailers` | map string → string | grpc |
| | `stream` | list of frames | grpc |
| Extensions | `x-*` | anything | all |

**Frame:** `{body, event, id, delay}`. `event` and `id` apply to SSE only.

**Fault types:** `timeout`, `reset` and `truncated` for http; `drop`, `close`, `delay` and `duplicate` for ws and sse; `timeout` and `reset` for grpc. (`fault-not-for-protocol` otherwise.)

**Rule conditions (`when`, P0-03).** A mapping of selector to matcher, where all entries must match:

| Selector | Reads |
| --- | --- |
| `path.<param>` | Path parameter |
| `query.<name>` | First value of a query parameter |
| `header.<name>` | Request header, case-insensitive |
| `cookie.<name>` | Cookie value |
| `body.<a.b.0.c>` | JSON body, dotted path, numbers index arrays |
| `var.<name>` | A variable set by an earlier `set` |
| `call` | 1-based count of calls to this route since start or reset |
| `message.<a.b>` | Incoming WebSocket message (only inside `on`) |

| Matcher | Meaning |
| --- | --- |
| a plain value | Equals; numbers compare numerically, strings exactly |
| `{eq: v}` / `{ne: v}` | Equals / not equals |
| `{in: [..]}` | One of |
| `{matches: "regex"}` | RE2 regex; compiled at load (`invalid-regex`) |
| `{exists: true\|false}` | Present / absent |
| `{gt: n}`, `{gte: n}`, `{lt: n}`, `{lte: n}` | Numeric comparison |

An operator mapping holds exactly one operator. For OR, write two rules pointing at the same state.

### 4.5 Short examples for the other protocols

```yaml
# routes/chat.room/route.yaml
id: chat.room
protocol: ws
path: /chat/{room}
summary: Chat room socket
status: draft
active: normal
states:
  normal:
    onConnect: [ { body: { type: welcome } } ]
    on:
      - when: { message.type: ping }
        send: [ { body: { type: pong } } ]
    every: [ { interval: 5s, send: { body: tick.json }, times: 10 } ]
  rejected:
    reject: { status: 401 }
  flaky:
    fault: { type: drop, rate: 0.1 }
```

```yaml
# routes/prices.stream/route.yaml
id: prices.stream
protocol: sse
path: /prices
summary: Live price feed
status: draft
active: live
states:
  live:
    every: [ { interval: 1s, send: { event: price, body: price.json } } ]
```

```yaml
# routes/orders.get-rpc/route.yaml
id: orders.get-rpc
protocol: grpc
service: shop.v1.OrderService
rpc: GetOrder
summary: Fetch one order
status: draft
active: found
states:
  found:
    body: found.json
    metadata: { x-request-id: abc }
  missing:
    code: NOT_FOUND
    message: order not found
```

```yaml
# routes/cart/route.yaml
id: cart
method: CRUD
path: /cart/items
summary: Cart items, full CRUD
status: draft
crud: { collection: cart_items, idField: id }
active: ok
states:
  ok: {}
  down:
    status: 503
    body: { error: unavailable }
```

---

## 5. `internal/model` (task 5)

Plain types with no I/O. The yaml tags are the single source of field names: the loader, the canonical writer and the schema test all read them through reflection. Nothing here imports anything outside the standard library.

### 5.1 Files and exported API

```go
// source.go
// Source locates a value for error messages.
type Source struct {
	File string // slash path relative to the .mockmachina folder
	Line int    // 1-based; 0 when unknown
}

// project.go
type Project struct {
	Config Config
	Routes []*Route // sorted by ID; Route relies on it
}
// Route finds a route by id with a binary search over Routes. O(log r).
func (p *Project) Route(id string) (*Route, bool)

const ConfigVersion = 1 // newest config.yaml version this build understands

type Config struct {
	Version int    `yaml:"version"`
	Ports   Ports  `yaml:"ports"`
	Host    string `yaml:"host"`
	Seed    uint64 `yaml:"seed"`
	Locale  string `yaml:"locale"`
	Proxy   Proxy  `yaml:"proxy"`
	AI      AI     `yaml:"ai"`
	Src     Source `yaml:"-"`
}
type Ports struct{ Mock, GRPC, Control int } // tags: mock, grpc, control
type Proxy struct{ Target string; Enabled bool }
type AI struct{ Enabled bool }
func DefaultConfig() Config

// route.go
type Route struct {
	ID         string    `yaml:"id"`
	Protocol   Protocol  `yaml:"protocol,omitempty"`
	Method     Method    `yaml:"method,omitempty"`
	Path       string    `yaml:"path,omitempty"`
	Service    string    `yaml:"service,omitempty"`
	RPC        string    `yaml:"rpc,omitempty"`
	Summary    string    `yaml:"summary"`
	Status     Status    `yaml:"status"`
	Group      string    `yaml:"group,omitempty"`
	Owners     Owners    `yaml:"owners,omitempty"`
	Request    *Request  `yaml:"request,omitempty"`
	Responses  Responses `yaml:"responses,omitempty"`
	Active     string    `yaml:"active"`
	Mode       Mode      `yaml:"mode,omitempty"`
	Rules      []Rule    `yaml:"rules,omitempty"`
	Serve      Serve     `yaml:"serve,omitempty"`
	CRUD       *CRUD     `yaml:"crud,omitempty"`
	States     States    `yaml:"states"`
	Generated  bool      `yaml:"generated,omitempty"`
	Extensions Extensions `yaml:"-"`
	Dir        string     `yaml:"-"` // "routes/users.list"
	Src        Source     `yaml:"-"`
}
// Keys returns the conflict keys this route claims: "GET /users/{}",
// "WS /chat/{}", "GRPC shop.v1.OrderService/GetOrder". CRUD claims six.
func (r *Route) Keys() []string

type Owners struct{ Backend, Frontend []string }
func (o Owners) IsZero() bool
type CRUD struct{ Collection, IDField string }

type Request struct {
	Params, Query, Headers map[string]Schema
	Body    *Schema
	Example any
	Src     Source
}
// Schema is a JSON Schema kept as raw data; Phase 2 compiles it.
type Schema struct {
	Raw map[string]any
	Src Source
}
type ResponseCode int // 100–599, or DefaultResponse
const DefaultResponse ResponseCode = 0
type Response struct{ Code ResponseCode; Schema Schema }
type Responses []Response // file order

// Extensions holds x-* keys verbatim, in file order.
type Extensions []Extension
type Extension struct{ Key string; Value any }

// state.go
type States []NamedState // file order matters (sequential mode, fallback)
type NamedState struct{ Name string; State *State }
// Get scans linearly: states per route are few, so this beats a map
// and keeps file order without a second structure (engineering.md §16).
func (s States) Get(name string) (*State, bool)
func (s States) Names() []string

type State struct {
	Status          int               `yaml:"status,omitempty"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	Body            Body              `yaml:"body,omitempty"`
	Template        *bool             `yaml:"template,omitempty"`
	ValidateRequest *bool             `yaml:"validateRequest,omitempty"`
	Latency         Latency           `yaml:"latency,omitempty"`
	Fault           *Fault            `yaml:"fault,omitempty"`
	Set             map[string]any    `yaml:"set,omitempty"`
	Callbacks       []Callback        `yaml:"callbacks,omitempty"`
	OnConnect       []Frame           `yaml:"onConnect,omitempty"`
	On              []Reply           `yaml:"on,omitempty"`
	Every           []Repeat          `yaml:"every,omitempty"`
	Reject          *Reject           `yaml:"reject,omitempty"`
	Code            GRPCCode          `yaml:"code,omitempty"`
	Message         string            `yaml:"message,omitempty"`
	Metadata        map[string]string `yaml:"metadata,omitempty"`
	Trailers        map[string]string `yaml:"trailers,omitempty"`
	Stream          []Frame           `yaml:"stream,omitempty"`
	Extensions      Extensions        `yaml:"-"`
	Src             Source            `yaml:"-"`
}
// EffectiveStatus returns Status, or 200 when unset.
func (s *State) EffectiveStatus() int

type BodyKind uint8
const (BodyNone BodyKind = iota; BodyFile; BodyInline; BodyGenerate)
type Body struct {
	Kind     BodyKind
	File     string // slash path relative to the route folder
	Inline   any    // JSON-compatible: map[string]any, []any, string, float64, bool, nil
	Data     []byte // file contents, filled by the loader
	Implicit bool   // File came from the <state>.json default, not from the YAML
	Src      Source
}

type Latency struct{ Base, Jitter time.Duration }
func (l Latency) IsZero() bool

type Fault struct {
	Type  FaultType
	Rate  float64       // (0, 1]; 1 when omitted
	After time.Duration // 0 = immediately
	Src   Source
}
type Callback struct {
	URL     string
	Method  Method // default POST
	Headers map[string]string
	Body    Body
	Delay   time.Duration
	Src     Source
}
type Frame struct {
	Body  Body
	Event string        // SSE
	ID    string        // SSE
	Delay time.Duration
	Src   Source
}
type Reply struct{ When []Condition; Send []Frame; Src Source }
type Repeat struct{ Interval time.Duration; Send Frame; Times int; Src Source }
type Reject struct{ Status int; Body Body; Src Source }

// rule.go
type Rule struct{ When []Condition; State string; Src Source }
type Condition struct {
	Selector Selector
	Op       Op
	Value    any
	Src      Source
}
type Scope string // path, query, header, cookie, body, var, call, message
type Selector struct{ Scope Scope; Name string } // Name empty for call
func ParseSelector(s string) (Selector, error)
func (s Selector) String() string
type Op string // eq, ne, in, matches, exists, gt, gte, lt, lte
func (o Op) Valid() bool
func Ops() []Op

// enums.go: each enum has constants, Valid(), and a list function for hints.
type Protocol string // ProtocolHTTP "http", ProtocolWS "ws", ProtocolSSE "sse", ProtocolGRPC "grpc"
func (p Protocol) Valid() bool      // "" is not valid; callers treat "" as unset
func (p Protocol) Effective() Protocol // "" → ProtocolHTTP
func Protocols() []Protocol
type Method string    // GET POST PUT PATCH DELETE HEAD OPTIONS CRUD
type Status string    // draft agreed implemented deprecated
type Mode string      // active rules sequential random; Effective: "" → active
type Serve string     // auto mock proxy;                  Effective: "" → auto
type FaultType string // timeout reset truncated drop close delay duplicate
func (f FaultType) AppliesTo(p Protocol) bool
// (Valid and list functions for each, as for Protocol.)

// grpc.go
type GRPCCode int // 0 OK … 16 UNAUTHENTICATED
func ParseGRPCCode(s string) (GRPCCode, error) // "NOT_FOUND" or "5"; names are uppercase only
func (c GRPCCode) String() string
func (c GRPCCode) Valid() bool

// duration.go
// ParseDuration is time.ParseDuration that rejects negative values and bare numbers.
func ParseDuration(s string) (time.Duration, error)

// names.go
func ValidRouteID(s string) bool
func ValidStateName(s string) bool // regex plus the reserved YAML words
func ValidVarName(s string) bool   // ^[a-zA-Z_][a-zA-Z0-9_]*$
```

`ParseSelector`, `ParseGRPCCode` and `ParseDuration` return wrapped sentinel errors (`ErrUnknownScope`, `ErrUnknownGRPCCode`, `ErrNegativeDuration`, `ErrMissingUnit`). The loader turns those into `Problem`s with hints.

### 5.2 Tests (write in this order)

| File | Test | Cases |
| --- | --- | --- |
| `enums_test.go` | `TestProtocol_Valid` | http, ws, sse, grpc valid; `""`, `HTTP`, `tcp` invalid |
| | `TestProtocol_Effective` | `""` → http; ws → ws |
| | `TestMethod_Valid`, `TestStatus_Valid`, `TestMode_Valid`, `TestServe_Valid`, `TestFaultType_Valid` | All constants valid; wrong case and unknown invalid |
| | `TestMode_Effective`, `TestServe_Effective` | Defaults |
| | `TestFaultType_AppliesTo` | The full protocol × type table from §4.4 |
| | `TestEnumLists_MatchConstants` | `Protocols()` etc. list every constant exactly once (guards against forgetting to add one) |
| `grpc_test.go` | `TestParseGRPCCode` | `OK`→0, `NOT_FOUND`→5, `5`→5, `16`→16; `not_found`, `17`, `-1`, `""` → error |
| | `TestGRPCCode_String` | Round trip with Parse for 0–16 |
| `duration_test.go` | `TestParseDuration` | `250ms`, `5s`, `1m30s`, `0s` valid; `-1s` → `ErrNegativeDuration`; `5` → `ErrMissingUnit`; `""`, `fast` → error |
| `names_test.go` | `TestValidRouteID` | `users.list`, `a`, `0abc`, `orders.get-rpc` valid; `Users`, `-x`, `.x`, `a b`, `a/b`, `""` invalid |
| | `TestValidStateName` | `success`, `server_error`, `e2` valid; `Server`, `1x`, `a-b`, `""`, and each reserved word invalid |
| | `TestValidVarName` | |
| `rule_test.go` | `TestParseSelector` | Every scope; `header.X-Api-Key` keeps the name as written; `call` with no name; `call.x`, `page`, `cookie`, `foo.bar` → error |
| | `TestOp_Valid` | |
| `state_test.go` | `TestStates_Get`, `TestStates_Names_KeepsFileOrder`, `TestState_EffectiveStatus` | |
| `route_test.go` | `TestRoute_Keys` | `GET /users`; `{id}` and `{userId}` give the same key; CRUD `/cart/items` gives six keys; ws; sse; grpc |
| `project_test.go` | `TestProject_Route`, `TestDefaultConfig` | Found, missing, first and last ids (binary search edges); documented defaults |

**Done when:** all of the above pass and coverage is ≥ 95%. That's easy for pure types; a gap means an untested branch.

---

## 6. `internal/config`: problems and suggestions (task 6)

### 6.1 API

```go
// problem.go
type Severity int8
const (SeverityError Severity = iota; SeverityWarning; SeverityInfo)
func (s Severity) String() string // "error", "warning", "info"

type Problem struct {
	Severity Severity
	Code     string       // stable id, e.g. "unknown-field"; see Codes()
	Src      model.Source
	Msg      string       // what's wrong, in the project's words
	Hint     string       // how to fix; may be empty
}
// String renders "routes/a/route.yaml:7: msg (hint)".
// Warnings get "warning: " after the location. Line 0 prints "file: msg".
func (p Problem) String() string

type Problems []Problem // deliberately not an error (engineering.md §4)
func (ps Problems) Sorted() Problems         // copy; slices.SortStableFunc by File, Line, Code, Msg
func (ps Problems) Count(s Severity) int
func (ps Problems) Fails(strict bool) bool   // errors, or warnings too when strict
func (ps Problems) Summary() string          // "no problems" | "1 problem in 1 file" | "3 problems in 2 files"
func Codes() []string                         // every code the checks can emit

// suggest.go
// Suggest returns the candidate closest to got within edit distance 2, preferring
// the earliest candidate on ties. ok is false when nothing is close enough.
//
// Bounded Levenshtein over runes: candidates whose length differs by more than
// 2 are skipped without computing anything; two rows of the table are kept;
// a candidate is abandoned as soon as a whole row exceeds 2. O(c·n) worst case.
func Suggest(got string, candidates []string) (best string, ok bool)
// HintFor returns `did you mean "x"?` or `expected one of: a, b, c`.
func HintFor(got string, candidates []string) string
```

Inside the package an unexported `collector` gathers problems with helpers such as `c.errorf(src, code, hint, format, args...)` and `c.warnf(...)`. Checks never build a `Problem` by hand, so the format can't drift.

### 6.2 Tests

| Test | Cases |
| --- | --- |
| `TestSeverity_String` | All three; unknown → `severity(9)` |
| `TestProblem_String` | With hint; without hint (no trailing parentheses); warning prefix; line 0 |
| `TestProblems_Sorted` | Sorts by file, then line (numerically: 9 before 10), then code; doesn't modify the receiver |
| `TestProblems_Summary` | 0, 1, many; one file versus several |
| `TestProblems_Fails` | Errors only; warnings only (strict and not); empty |
| `TestSuggest` | `emtpy`→`empty`; `mehtod`→`method`; `xyz`→none; exact match→itself; empty candidates→none; a tie keeps the first; case differences count as edits |
| `TestHintFor` | Close match → "did you mean"; none → "expected one of" with the list in the order given |
| `TestDistance` (internal, in `suggest_internal_test.go`) | `kitten/sitting` → over the limit; `abc/abd`=1; empty strings; multi-byte runes (`"café"` vs `"cafe"` = 1, so it counts runes, not bytes) |
| `BenchmarkSuggest` | 50 candidates of typical field names, plus one 10 KB candidate; the early exit shows up as the long candidate costing about the same as a short one |

---

## 7. `internal/config`: the loader (task 7)

### 7.1 API

```go
// paths.go
const (
	DirName    = ".mockmachina"
	ConfigFile = "config.yaml"
	RoutesDir  = "routes"
	RouteFile  = "route.yaml"
)

// load.go
// Load reads the project in dir (the .mockmachina folder itself).
func Load(dir string) (*model.Project, Problems, error) // LoadFS(os.DirFS(dir))
// LoadFS reads a project from fsys, whose root is the .mockmachina folder.
// err is non-nil only when the project can't be read at all; then the
// project is nil. Otherwise the project is returned even if Problems has errors.
func LoadFS(fsys fs.FS) (*model.Project, Problems, error)
```

### 7.2 Algorithm

1. Read `config.yaml` if present; otherwise use `model.DefaultConfig()`. Decode it with the same node walker as routes.
2. `fs.ReadDir(fsys, "routes")`. A missing `routes/` is fine (zero routes). An unreadable one returns `err`.
3. For each entry: skip names starting with `.`; a file that isn't a folder is skipped silently; a folder without `route.yaml` gets `missing-route-file`.
4. For each `route.yaml`:
   1. `fs.ReadFile`. Detect CRLF (kept for the writer, normalised for parsing).
   2. `yaml.Unmarshal` into a `yaml.Node`. Syntax errors are rewritten (§7.3) and the file is skipped.
   3. Walk the document node. The walker has one function per model type (`decodeRoute`, `decodeState`, `decodeBody`, `decodeLatency`, `decodeFault`, `decodeRules`, `decodeConditions`, `decodeFrames`, …). Each function:
      - checks the node kind (`expected a mapping`, `expected a list`);
      - for mappings, compares keys with the known fields for that type (from `fields.go`), allowing `x-*` where P0-17 permits it, and reports unknown keys with a suggestion;
      - records `Src` from `node.Line`;
      - decodes leaves with `node.Decode` into the leaf type, rewriting type errors as `expected a number`;
      - resolves `AliasNode`s; rejects `<<` merge keys (`merge-key`);
      - reports duplicate keys once (yaml.v3 also catches these, but its message isn't ours).
   4. Resolve bodies: for `BodyFile`, `path.Join(route dir, file)`, then `fs.ValidPath` (else `body-outside`), then `fs.ReadFile` (else `body-missing`), then store `Data`. For an omitted body, try `<state>.json` and set `Implicit`. Over 5 MiB adds the warning `body-large`.
5. Sort routes by ID. Run the checks (§8). Return.

### 7.3 Rewriting YAML library errors

| Library says (example) | We say | Code |
| --- | --- | --- |
| `yaml: line 4: found character that cannot start any token` (from a tab) | `route.yaml:4: bad indentation (YAML needs spaces, not tabs)` | `yaml-syntax` |
| `yaml: line 7: mapping values are not allowed in this context` | `route.yaml:7: unexpected ":" (quote values that contain ": ")` | `yaml-syntax` |
| `mapping key "x" already defined at line 3` | `route.yaml:9: "x" is defined twice (first on line 3)` | `duplicate-key` |
| Any other message | `route.yaml:N: invalid YAML: <library text without the "yaml: line N:" prefix>` | `yaml-syntax` |

Each row gets a test. The last row is the safety net; any library message we see often gets promoted to its own row.

### 7.4 `fields.go`

```go
// knownFields returns the yaml names of t's fields, in declaration order,
// skipping `yaml:"-"`. Used by the decoder, the writer and the schema test.
func knownFields(t reflect.Type) []string
```

Reflection runs once per type: results are cached in a small map built by `sync.OnceValue`, because the decoder asks for the same dozen types for every node it walks.

It's exported to tests via `export_test.go`. That name, like `*_internal_test.go`, is one the `testpackage` linter allows to use the internal package (`var KnownFields = knownFields`). This is the standard Go pattern for testing unexported code from an external test package without widening the API.

### 7.5 Tests

All tests in this section use `testkit.MapFS` except the last three.

| # | Test | Asserts |
| --- | --- | --- |
| 1 | `TestLoadFS_EmptyProject` | No routes, default config, no problems |
| 2 | `TestLoadFS_ConfigDefaults` / `_ConfigOverrides` / `_ConfigUnknownField` | |
| 3 | `TestLoadFS_OneRoute` | id, method, path, summary, status, `Dir`, `Src` |
| 4 | `TestLoadFS_KeepsStateOrder` | c, a, b stay c, a, b |
| 5 | `TestLoadFS_RecordsLines` | Route, state, rule and body `Src.Line` |
| 6 | `TestLoadFS_UnknownField` | Golden message with suggestion, at route, state and nested levels |
| 7 | `TestLoadFS_Extensions` | Route- and state-level `x-*` kept in order; `x-` inside `latency` → unknown field |
| 8 | `TestLoadFS_WrongKinds` | A list where a mapping belongs, a string where a number belongs: friendly messages |
| 9 | `TestLoadFS_YAMLErrors` | One case per §7.3 row |
| 10 | `TestLoadFS_EmptyFile` / `_OnlyComments` | `empty-route-file`, no panic |
| 11 | `TestLoadFS_AliasesResolve` / `_MergeKeyRejected` | |
| 12 | `TestLoadFS_BodyForms` | File, inline mapping, inline list, `generate`, omitted with `<state>.json`, omitted without |
| 13 | `TestLoadFS_BodyOutside` | `../x.json` and `/etc/passwd` → `body-outside` |
| 14 | `TestLoadFS_BodyMissing` | |
| 15 | `TestLoadFS_DurationForms` | `latency: 120ms`; `{base, jitter}`; `latency: 120` → `missing unit` hint |
| 16 | `TestLoadFS_FaultForms` | Shorthand; object; rate outside (0, 1] |
| 17 | `TestLoadFS_Rules` | Selectors, plain and operator matchers, bad selector, two operators in one mapping |
| 18 | `TestLoadFS_StreamsAndGRPC` | ws, sse and grpc examples from §4.5 decode completely |
| 19 | `TestLoadFS_SkipsHiddenAndFiles` | `.DS_Store`, `routes/README.md`, `routes/.draft/` |
| 20 | `TestLoadFS_MissingRouteFile` | |
| 21 | `TestLoadFS_CollectsEveryProblem` | Three independent mistakes → exactly three problems |
| 22 | `TestLoadFS_SortsRoutes` | |
| 23 | `TestLoad_DirNotFound` | `errors.Is(err, fs.ErrNotExist)`, project nil |
| 24 | `TestLoad_Shop` | `testdata/projects/shop` from disk: zero problems; deep-compared to `testdata/golden/shop.model.txt` (a stable dump) |
| 25 | `FuzzLoadFS_Route` | Any bytes as `route.yaml` never panic; seeded with every fixture's `route.yaml` |

---

## 8. `internal/config`: checks (task 8)

### 8.1 Mechanism

```go
// checks.go
type check struct {
	code string
	run  func(p *model.Project, c *collector)
}
var checks = []check{ /* one entry per row in §8.2, in this order */ }
```

Codes produced while decoding (`yaml-syntax`, `unknown-field`, …) are listed in `Codes()` too, so the meta-test below covers them as well.

### 8.2 Every problem code in Phase 0

| Code | Severity | Raised by | Example message |
| --- | --- | --- | --- |
| `yaml-syntax` | error | decode | `bad indentation (YAML needs spaces, not tabs)` |
| `duplicate-key` | error | decode | `"active" is defined twice (first on line 3)` |
| `merge-key` | error | decode | `merge keys (<<) aren't supported (repeat the fields or use an alias)` |
| `empty-route-file` | error | decode | `route.yaml is empty` |
| `missing-route-file` | error | decode | `routes/users.list has no route.yaml` |
| `unknown-field` | error | decode | `unknown field "mehtod" (did you mean "method"?)` |
| `wrong-type` | error | decode | `"status" should be a number, got "ok"` |
| `invalid-duration` | error | decode | `latency "120" has no unit (write 120ms)` |
| `invalid-selector` | error | decode | `unknown rule selector "page" (start with path., query., header., cookie., body., var. or use call)` |
| `invalid-matcher` | error | decode | `a matcher holds one operator; found gt and lt (write two rules)` |
| `missing-field` | error | check | `HTTP route needs "path"` |
| `invalid-enum` | error | check | `status "agred" isn't valid (did you mean "agreed"?)`; with no close match: `status "aproved" isn't valid (expected one of: draft, agreed, implemented, deprecated)` |
| `invalid-id` | error | check | `id "Users" should be lowercase letters, digits, dots and dashes` |
| `id-mismatch` | error | check | `id "users.list" doesn't match folder "users-list"` |
| `invalid-state-name` | error | check | `state "Server Error" should be lowercase with underscores, e.g. server_error` |
| `reserved-state-name` | error | check | `state name "no" is a YAML keyword; pick another name` |
| `no-states` | error | check | `users.list has no states` |
| `active-missing` | error | check | `active state "emtpy" doesn't exist (did you mean "empty"? states: success, empty)` |
| `rule-state-missing` | error | check | `rule 2 points to state "locked", which doesn't exist` |
| `rules-without-mode` | warning | check | `users.list has rules but mode is "active", so they never run (set mode: rules)` |
| `mode-rules-without-rules` | error | check | `mode is "rules" but there are no rules` |
| `invalid-regex` | error | check | `rule 1: "[a-" isn't a valid regex: missing closing ]` |
| `invalid-method` | error | check | `method "get" should be uppercase: GET` |
| `invalid-path` | error | check | `path "users" must start with "/"`; unbalanced braces; repeated `{id}` |
| `field-not-for-protocol` | error | check | `"code" applies to grpc routes; this route is http` |
| `fault-not-for-protocol` | error | check | `fault "drop" applies to ws and sse; this route is http` |
| `invalid-status-code` | error | check | `status 999 isn't an HTTP status (100–599)` |
| `invalid-grpc-code` | error | check | `code "NOTFOUND" isn't a gRPC code (did you mean "NOT_FOUND"?)` |
| `invalid-rate` | error | check | `fault rate 1.5 must be above 0 and at most 1` |
| `invalid-var-name` | error | check | |
| `body-missing` | error | load | `state "slow" body file "slow.json" not found` |
| `body-outside` | error | load | `body "../secrets.json" points outside the project` |
| `body-generate-not-for-protocol` | error | check | `body: generate needs a schema, which ws routes don't have` |
| `duplicate-route` | error | check | `GET /users is defined in users.list and users.all` |
| `crud-missing` | error | check | `method CRUD needs a "crud" section` |
| `config-version` | error | check | `config version 2 needs a newer mockmachina (this is v0.0.0-dev)` |
| `invalid-port` | error | check | `ports.mock and ports.control are both 4001` |
| `invalid-host` / `invalid-locale` / `invalid-url` / `invalid-seed` | error | check | |
| `ref-missing` | error | check | `$ref "../../schemas/usr.yaml#/User" points to a file that doesn't exist (did you mean user.yaml?)`. Phase 0 checks only that the file exists, not the fragment |
| `no-body` | warning | check | `state "success" has no body; add success.json or body:` |
| `body-large` | warning | load | `success.json is 7.2 MB; large bodies slow down reloads` |
| `no-failure-state` | warning | check | `users.list has no 4xx or 5xx state; apps can't test errors` |
| `no-owners` | warning | check | `users.list has no owners; reviews can't be routed` |

### 8.3 Fixtures and tests

- **One folder per code:** `testdata/projects/broken-<code>/.mockmachina/…` containing exactly that one mistake. Its golden file `testdata/golden/broken-<code>.txt` is hand-written first (engineering.md §10).
- **`testdata/projects/broken/`** is the playbook's composite example (three problems in two files) for the exit script.

| Test | Asserts |
| --- | --- |
| `TestChecks_Golden` | For each `broken-*` folder: output equals the golden file |
| `TestChecks_EveryCodeHasFixture` | Every entry in `Codes()` has a `broken-<code>` folder, and every folder is a known code. A new check can't ship without its fixture |
| `TestChecks_FixtureHasOnlyItsCode` | Each `broken-<code>` produces only problems with that code. Keeps fixtures honest |
| `TestChecks_GoodFixturesClean` | `minimal` and `shop` have zero problems |
| `TestChecks_Order` | Problems come out sorted regardless of the order checks ran in |

---

## 9. `internal/config`: writing (task 9)

### 9.1 API

```go
// write.go
// WriteFileAtomic writes data to a temp file in path's folder, syncs it, and
// renames it over path. On any failure the original file is untouched and the
// temp file is removed.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error

// SetActive changes the active state of one route by editing only the value of
// the active key, keeping every other byte, comment and line ending as it was.
// It returns *UnknownRouteError or *UnknownStateError (with suggestions).
func SetActive(dir, routeID, state string) error

type UnknownRouteError struct{ ID string; Suggestion string }
type UnknownStateError struct{ Route, State string; Valid []string; Suggestion string }

// marshal.go
// MarshalRoute renders r in canonical form: fields in model order, 2-space
// indent, states in order, extensions last. Used by add and import from Phase 1.
func MarshalRoute(r *model.Route) ([]byte, error)
```

### 9.2 How `SetActive` keeps the diff to one line

Re-encoding a `yaml.Node` keeps comments but can still change indentation and quoting elsewhere in the file. So `SetActive` doesn't re-encode. Instead it:

1. Parses the file to find the `active` value node's line and column.
2. Finds that token's extent in the original bytes: a plain scalar runs to whitespace or `#`; a quoted scalar runs to its closing quote.
3. Splices in the new name. State names are always safe plain scalars, thanks to P0-07.
4. If `active` is missing, inserts `active: <name>` on its own line just before `states:`, using that file's line ending.
5. Writes with `WriteFileAtomic`, keeping the file's permissions.

### 9.3 Tests

| Test | Asserts |
| --- | --- |
| `TestWriteFileAtomic_CreatesAndReplaces` | |
| `TestWriteFileAtomic_LeavesNoTempFiles` | The folder listing after success has only the target |
| `TestWriteFileAtomic_FailureKeepsOriginal` | The target is a folder, so the rename fails: error returned, folder untouched, no temp file |
| `TestWriteFileAtomic_Perm` | Skipped on Windows (`runtime.GOOS`), with a reason |
| `TestSetActive_ChangesOneLine` | Byte diff between before and after is exactly the `active` line |
| `TestSetActive_QuotingStyles` | `success`, `"success"`, `'success'`, `success # note`, `active:   success` |
| `TestSetActive_MissingKeyInserts` | |
| `TestSetActive_KeepsCRLF` | `testdata/…/crlf-route.yaml` stays CRLF |
| `TestSetActive_UnknownState` | `errors.As` → `*UnknownStateError` with `Suggestion` |
| `TestSetActive_UnknownRoute` | `*UnknownRouteError` |
| `TestSetActive_ThenLoad` | Load after the change sees the new active state |
| `TestMarshalRoute_Golden` | Each `shop` route's canonical form equals `testdata/golden/marshal/<id>.yaml` |
| `TestMarshalRoute_RoundTrip` | For every good fixture route: load → marshal → load is equal (`cmp.Diff`, ignoring `Src`, `Body.Data` and `Body.Implicit`) |
| `TestMarshalRoute_Idempotent` | Marshalling twice gives identical bytes |

---

## 10. Schemas (task 10)

- `schema/route.schema.json` and `schema/config.schema.json`, JSON Schema 2020-12, with `$id` set to their raw GitHub URLs.
- `additionalProperties: false` everywhere, plus `patternProperties: {"^x-": {}}` where P0-17 allows it.
- Shared parts go under `$defs` (`duration`, `body`, `frame`, `condition`, `fault`).
- Durations use `pattern: "^([0-9]+(\\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$"`.
- Protocol-specific required fields use `if`/`then` on `protocol` (and on `method: CRUD`).

| Test (in `internal/config`, `schema_test.go`) | Asserts |
| --- | --- |
| `TestRouteSchema_CoversModel` | Every name from `knownFields` for `Route`, `State` and each nested type appears in the matching schema `properties`, and the other way round |
| `TestConfigSchema_CoversModel` | The same for `Config` |
| `TestSchemas_AreValid2020_12` | Both compile with the metaschema |
| `TestRouteSchema_GoodFixturesValidate` | Every `route.yaml` in `minimal` and `shop`, and every example in `docs/file-format.md` |
| `TestRouteSchema_RejectsStructuralMistakes` | The `broken-unknown-field`, `broken-invalid-enum`, `broken-missing-field`, `broken-invalid-duration` fixtures fail validation |

**YAML to JSON for validation:** decode with yaml.v3 into `any`, marshal to JSON, then decode with `json.Decoder.UseNumber()`. jsonschema expects `json.Number` for exact numeric checks.

---

## 11. `internal/buildinfo` and `internal/cli` (tasks 3 and 11)

### 11.1 `buildinfo`

```go
type Info struct {
	Version  string // "v0.1.0", or "(devel)"
	Commit   string // 12-character revision, or ""
	Modified bool
}
func (i Info) String() string // "v0.1.0 (a1b2c3d4e5f6)" | "(devel) (fe8193232351, modified)"
func Read() Info                              // debug.ReadBuildInfo + the ldflags override
func FromBuildInfo(bi *debug.BuildInfo, override string) Info // pure; what tests call
var version string // set with -ldflags -X for release builds
```

| Test | Cases |
| --- | --- |
| `TestFromBuildInfo` | nil → `(devel)`; tagged module version; `(devel)` with `vcs.revision`; `vcs.modified=true`; override wins; long revision is shortened |
| `TestInfo_String` | Each shape |

### 11.2 `cli`

*As built in task 3 (authoritative where it differs from the plan below):*

- `Run(ctx context.Context, env Env) int` with `type Env struct{ Args []string; Stdout, Stderr io.Writer; Info buildinfo.Info }`. A struct, not five parameters (engineering.md §15), and tests pass buffers instead of capturing the process output.
- `NewRootCmd()` takes no parameters; only `Run` needs the build info.
- Exit codes, `ExitError`, `ExitCode` and the exported `UsageError(err) error` live in `exit.go`.
- Unknown commands are usage errors (the root has `cobra.ArbitraryArgs` and a `RunE` that rejects arguments); without that, Cobra silently accepted `mockmachina nope`.
- Users see errors as Fang renders them: an ERROR box, the first letter capitalised, a final full stop (`Unknown flag: --nope.`). Tests pin that text.
- "Did you mean" suggestions for unknown commands come in task 11, test-first, when `lint` exists to be suggested.
- `just smoke` and the CI step check `--version` output, not just the exit status.
- `golang.org/x/text` is pinned at v0.39.0: v0.24.0 (via Fang) had GO-2026-5970, reachable from `cli.Run`, and v0.42.0 needs Go 1.26. `just lint` and CI now fail if `go.mod`'s `go` line changes.

```go
// run.go
// Run executes the command line and returns the process exit code. It is the
// only place Fang is used (ADR 004).
func Run(ctx context.Context, args []string, info buildinfo.Info) int

const (ExitOK = 0; ExitFailure = 1; ExitUsage = 2)
type ExitError struct{ Code int; Err error } // Error, Unwrap
func ExitCode(err error) int // nil → 0; *ExitError (wrapped) → its code; usage errors → 2; else → 1

// root.go
func NewRootCmd(info buildinfo.Info) *cobra.Command
// SilenceUsage and SilenceErrors on (Fang prints errors); flag errors wrapped
// as usage errors; Args validators wrapped the same way.

// find.go
// FindProject walks up from start looking for a .mockmachina folder. Pure: it
// never reads the working directory itself, so tests pass a t.TempDir().
func FindProject(start string) (string, error) // ErrNoProject
var ErrNoProject = errors.New("no .mockmachina folder found")
// projectDir returns flagDir if set, else FindProject(os.Getwd()). The only
// os.Getwd call in the codebase (lint exclusion in §3.4).
func projectDir(flagDir string) (string, error)

// lint.go
func newLintCmd() *cobra.Command
// Flags: --dir (default: FindProject from the working directory), --strict.
// Prints sorted problems to stdout, then Summary. Returns *ExitError{1} when
// Problems.Fails(strict).
```

**The `lint` command in detail:**

```text
mockmachina lint [--dir DIR] [--strict]

  routes/orders.get/route.yaml:3: unknown field "mehtod" (did you mean "method"?)
  routes/users.list/route.yaml:7: active state "emtpy" doesn't exist (did you mean "empty"? states: success, empty, unauthorized)
  routes/users.list/route.yaml:12: state "slow" body file "slow.json" not found
  routes/users.list/route.yaml:2: warning: users.list has no owners; reviews can't be routed
  4 problems in 2 files
```

| Situation | stdout | stderr | Exit |
| --- | --- | --- | --- |
| Clean | `no problems` | | 0 |
| Warnings only | Problems + summary | | 0 |
| Warnings with `--strict` | Problems + summary | | 1 |
| Errors | Problems + summary | | 1 |
| No project found | | `no .mockmachina folder found in <dir> or any parent (pass --dir)` | 1 |
| `--dir` doesn't exist | | `reading <dir>: no such file or directory` | 1 |
| Unknown flag | | Fang's usage error | 2 |

**Fang's version flag:** Fang provides `--version` from `fang.WithVersion(info.String())`. There's no `version` subcommand in Phase 0.

### 11.3 Tests

| File | Test |
| --- | --- |
| `root_test.go` | `TestNewRootCmd_UsesBinaryName`, `_SilencesCobraOutput`, `_HasLintCommand` |
| `run_test.go` | `TestExitCode` (nil, plain, `ExitError` 1 and 2, wrapped, usage error, wrapped usage error) |
| `find_test.go` | `TestFindProject_InStart`, `_InParent`, `_NotFound` (`errors.Is(err, ErrNoProject)`), `_StopsAtFilesystemRoot`, `_IgnoresFileNamedLikeDir` (a *file* called `.mockmachina` is skipped). All use `t.TempDir()` |
| `lint_test.go` | `TestLint_OutputGolden` (shop and broken through `NewRootCmd` with `SetOut`/`SetErr`/`SetArgs`), `TestLint_StrictTurnsWarningsIntoFailure` |
| `script_test.go` | Runs every `testdata/script/*.txtar` |

**Scripts** (`testdata/script/`):

| Script | Checks |
| --- | --- |
| `version.txtar` | `--version` prints a version and exits 0 |
| `help.txtar` | `--help` lists `lint`, exits 0 |
| `usage_unknown_flag.txtar` | Exit 2 |
| `usage_unknown_command.txtar` | Exit 2 |
| `lint_ok.txtar` | `no problems`, exit 0 |
| `lint_broken.txtar` | All problems and the summary, exit 1 |
| `lint_warnings.txtar` / `lint_strict.txtar` | 0 / 1 |
| `lint_find_parent.txtar` | Run from `app/lib/src`, finds `app/.mockmachina` |
| `lint_no_project.txtar` | Message on stderr, exit 1 |
| `lint_dir_missing.txtar` | |
| `exit_phase0_version.txtar` | The playbook's exit criterion 1 |
| `exit_phase0_invalid_route.txtar` | The playbook's exit criterion 2, verbatim |

---

## 12. `internal/clock` and `internal/seed` (task 12)

```go
// clock
type Clock interface {
	Now() time.Time
	// Sleep waits for d or until ctx is done, whichever is first. It checks ctx
	// first, so a cancelled context returns at once even when d <= 0.
	Sleep(ctx context.Context, d time.Duration) error
}
type Real struct{}
var _ Clock = Real{}
```

There's no fake clock: tests run the real one inside a `synctest` bubble, where time is virtual. Phase 1 may add timer methods when the reload debounce needs them, test-first.

| Test | Asserts |
| --- | --- |
| `TestReal_SleepWaitsExactly` | In a bubble, `Now` advances by exactly `d` |
| `TestReal_SleepCancelled` | Cancel during a 1 h sleep: returns `context.Canceled` with no real time passing |
| `TestReal_SleepAlreadyCancelled` | Returns at once, even for `d = 0` |
| `TestReal_SleepNonPositive` | `d <= 0` returns nil immediately |

```go
// seed
// Rand returns a generator determined only by its arguments, so the same
// inputs give the same values regardless of goroutine scheduling.
func Rand(seed uint64, route string, call uint64, purpose string) *rand.Rand // math/rand/v2
// New returns a random non-zero seed, for when config.seed is 0.
func New() uint64
```

**Derivation:** SHA-256 over the inputs, each length-prefixed (so `("ab","c")` and `("a","bc")` differ). The first 16 bytes become the two PCG seeds. **This is part of the compatibility contract:** changing it changes every seeded run, so it's pinned by a golden test.

| Test | Asserts |
| --- | --- |
| `TestRand_SameInputsSameValues` | |
| `TestRand_EachInputMatters` | Table changing one argument at a time |
| `TestRand_NoConcatenationCollision` | |
| `TestRand_Golden` | `Rand(42, "users.list", 1, "latency")` gives exact pinned values |
| `TestRand_IndependentOfScheduling` | 100 goroutines started in shuffled order equal a serial run (`-race`) |
| `TestNew_NonZero` | |

---

## 13. `internal/testkit` (task 4) and `internal/archtest` (task 13)

```go
// testkit
// Path returns the absolute path of a file under the module-root testdata/.
func Path(t testing.TB, elem ...string) string
// Project returns the .mockmachina folder of testdata/projects/<name>.
func Project(t testing.TB, name string) string
// MapFS builds an in-memory project. Keys are slash paths; values have a
// common leading indent removed, so test files can indent YAML naturally.
func MapFS(t testing.TB, files map[string]string) fstest.MapFS
// Golden compares got with testdata/golden/<name>, rewriting it under -update.
func Golden(t testing.TB, got []byte, name string)
```

*As built in task 4:* `Path`, `MapFS` and `Golden`, plus `CompareGolden(tb, got, path)` and `WriteGolden(tb, got, path)`. Those two replace a single function with an `update bool` parameter, which engineering.md §15 forbids. `Project` waits until the loader needs it (task 7). `Path` finds the module root by walking up from the working directory to `go.mod`, with a narrow forbidigo exclusion for `os.Getwd` in `testkit/path.go`. gosec's G301/G306 are excluded globally: 0o755 folders and 0o644 files are intended for files shared in git. Tests use a `fakeTB` that records failures and ends the goroutine on `Fatalf`, as `testing.T` does.

`-update` is registered in `testkit` with `flag.Bool` at package level. That's allowed: it's test-only code, and it's the standard pattern.

| Test | Asserts |
| --- | --- |
| `TestMapFS_Dedents` | |
| `TestGolden_PassesAndFails` | Uses a fake `testing.TB` to check that a mismatch is reported, with a diff |
| `TestPath_FindsModuleRoot` | Works from any package folder |

**archtest** (`imports_test.go`) runs `go list -json ./...` from the module root, decodes `ImportPath`, `Imports` and `TestImports`, and checks each against the §2 table. Standard-library imports are always allowed (detected by having no dot in the first path element).

| Test | Asserts |
| --- | --- |
| `TestImports_FollowPackageMap` | Each package's non-test imports are within its allow-list |
| `TestImports_TestkitOnlyInTests` | No non-test file imports `testkit` |
| `TestImports_EveryPackageListed` | A new package without a row in the table fails, so the table stays complete |

---

## 14. Fixtures

| Fixture | Contents | Used by |
| --- | --- | --- |
| `projects/minimal` | `config.yaml` absent; one route `health.get` with one state `ok` and `ok.json` | loader, lint_ok |
| `projects/shop` | `config.yaml` with every field; routes `health.get`, `users.list` (§4.3, all states), `users.get`, `orders.create` (callbacks, `set`), `cart` (CRUD), `chat.room` (ws), `prices.stream` (sse), `orders.get-rpc` (grpc); `schemas/user.yaml` and `schemas/error.yaml` (empty mappings until Phase 2, there for `ref-missing`); `data/cart_items.json` | everything |
| `projects/broken` | The playbook's composite: three errors in two files | exit script |
| `projects/broken-<code>` | One per code in §8.2 | checks golden |
| `projects/warnings` | Only `no-owners` and `no-failure-state` | lint_warnings, lint_strict |
| `projects/crlf` | `crlf-route.yaml` committed with CRLF endings | SetActive |

---

## 15. `docs/file-format.md` (task 14)

The draft is §4 of this spec, plus:

- an introduction for frontend developers (what a route and a state are);
- the naming rules, and why state names can't be YAML words;
- one complete route per protocol;
- the problem code table from §8.2, as "what each lint message means";
- examples are fenced with ` ```yaml route `, which is how `TestFileFormat_ExamplesLoad` finds and loads them.

**Freeze:** after one backend and one frontend developer review it, the format is frozen. From then on, any change needs an ADR and a migration (`config.yaml` `version: 2`).

---

## 16. Build order

Each task is one branch and one pull request (engineering.md §13). "Done" always means: every listed test was red before its code, everything is green on all CI jobs, lint is clean, there are no comments in the code (engineering.md §11), and it has been reviewed.

| # | Task | Needs | Size | Spec |
| --- | --- | --- | --- | --- |
| 0 | Scaffold, first test ✅ | – | S | – |
| 1 | ADRs 001–007, `engineering.md`, this spec: review and close §1 | 0 | S | §1 |
| 2 | Plumbing: `justfile`, `.golangci.yml`, `.gitattributes`, `.editorconfig`, LICENSE, README, CONTRIBUTING, CHANGELOG, CI seen red once | 1 | M | §3 |
| 3 | `buildinfo`, `cli.Run`, `ExitCode`, `--version` through Fang, `main.go` | 2 | S | §11.1, §11.2 |
| 4 | `testkit` | 2 | S | §13 |
| 5 | `model` | 4 | M | §5 |
| 6 | `config`: problems and suggestions | 4 | S | §6 |
| 7 | `config`: loader | 5, 6 | L | §7 |
| 8 | `config`: checks and broken fixtures | 7 | L | §8 |
| 9 | `config`: writer and canonical marshal | 7 | M | §9 |
| 10 | Schemas | 7 | M | §10 |
| 11 | `lint` command, `FindProject`, scripts | 8 | M | §11.2 |
| 12 | `clock`, `seed` | 2 | S | §12 |
| 13 | `archtest` | 3–12 | S | §13 |
| 14 | `docs/file-format.md`, docs test, reviews → **freeze** | 10 | M | §15 |

Tasks 3, 4 and 12 can run in parallel after 2. Tasks 5 and 6 can run in parallel after 4. Tasks 9 and 10 can run in parallel with 8.

---

## 17. What I check in each review

1. **Test-first evidence:** the commit sequence or description shows each test failing first. New behaviour without a test is sent back.
2. **Test quality:** names describe behaviour; tables have named cases; `t.Parallel` where allowed; `t.Helper` in helpers; no sleeps, fixed ports or temp files outside `t.TempDir`.
3. **Errors:** wrapped with `%w` and context; no log-and-return; user problems as `Problem` with code, line and hint.
4. **API:** names follow engineering.md §3 and make comments unnecessary; no comments in code (§11); no new package-level state.
5. **Boundaries:** imports match §2; YAML only in `config`; Fang only in `run.go`.
6. **Cross-platform:** slash paths in output; no OS-specific assumptions without a skip and a reason.
7. **Spec drift:** if the code had to differ from this spec, the spec is updated in the same pull request.

---

## 18. Exit checklist

- [x] §1 decisions closed (2026-10-06); none changes an ADR; P0-01 updates the playbook
- [ ] Tasks 1–14 done
- [ ] `exit_phase0_version.txtar` and `exit_phase0_invalid_route.txtar` green on Linux, macOS and Windows, with both Go versions
- [ ] `docs/file-format.md` and both schemas reviewed by one backend and one frontend developer: **format frozen**
- [ ] Playbook Phase 0 marked done, with anything that changed recorded there
- [ ] Phase 1 spec written at this same depth before Phase 1 coding starts
