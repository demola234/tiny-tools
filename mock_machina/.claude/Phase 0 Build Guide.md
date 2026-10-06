---
tags: [mockmachina, phase-0, build-guide, go, tdd]
date: 2026-10-06
phase: 0
status: in-progress
---

# Phase 0 Build Guide

> The authoritative detail for every task now lives in [[Phase 0 Spec]]. This guide remains the friendly walkthrough; where they differ, the spec wins.

How to build Phase 0 of [[MockMechina Phase Playbook]] task by task. The playbook says *what* and *why*; this guide says *how*, with Go pointers for each step. The architecture is in [[MockMachina System Design]] (https://claude.ai/artifact/KtinJTkvKK6WppQqvV52L4). You write the code; bring each task back for review when its tests pass.

## How we work: test first, always

Every piece of production code comes in a TDD wrapper. No `.go` file gains behaviour until a test asks for it.

**The loop**

1. **Red.** Write one test for the next small behaviour. Run it and watch it fail *for the right reason*: an assertion failure that names the missing behaviour. A compile error only counts as red for the very first test in a new package.
2. **Green.** Write the least code that passes it. Hard-coding is fine if the next test will force the general version.
3. **Refactor.** Clean up the test and the code while everything stays green. Then commit.

**Rules**

- **One behaviour per test.** The test name says what it is: `TestSuggest_ReturnsClosestWithinTwoEdits`, not `TestSuggest2`.
- **Test through the public API.** Use the external test package (`package config_test`) by default, so tests use the package the way callers do. Drop to an internal test (`package config`) only for a tricky unexported helper.
- **Table-driven** when cases share a shape: `[]struct{ name, in, want }` plus `t.Run(tc.name, ...)` and `t.Parallel()`.
- **Fixtures and goldens.** Fixture projects live in `testdata/projects/<name>/`, expected output in `testdata/golden/`. A `-update` flag rewrites goldens; review the diff before committing it.
- **Every bug starts as a failing test** that reproduces it. Fix it only after you've seen that test fail.
- **CLI behaviour is tested with testscript** (`.txtar` files), so the real binary runs exactly as users run it.
- **Never commit red to `main`.** A red test and the code that turns it green go in the same commit (or the same PR).
- **Coverage is a signal, not a target.** CI prints `go test -cover` per package. A package with low coverage is a question for review, not a gate.

**Tooling for the loop:** `just tdd PKG` runs tests on save. Use `go run gotest.tools/gotestsum@latest --watch --format testname -- ./internal/PKG/...`, or `watchexec -e go -- go test ./internal/PKG/...` if you prefer.

**Seams that make TDD possible.** These are part of the design, so build them in from the first test:

| Seam | Production | In tests |
| --- | --- | --- |
| Filesystem for loading | `config.LoadFS(os.DirFS(dir))` | `fstest.MapFS{...}` built in the test, no disk |
| Time | `clock.Real{}` | `testing/synctest` bubble, or a fake `Clock` |
| Randomness | `seed.RandFor(...)` | Fixed seed, assert exact values |
| Output | `cmd.OutOrStdout()` | `cmd.SetOut(&buf)` |
| Network (Phase 1) | Server takes a `net.Listener` | `net.Listen("tcp", "127.0.0.1:0")` |

## Decisions settled on 2026-10-06

| ADR | Outcome |
| --- | --- |
| 001 Storage | Own route files, one folder per route |
| 002 Templates | Handlebars-like syntax, own small parser (Phase 5) |
| 003 YAML | `go.yaml.in/yaml/v3` v3.0.5. **v4 is still only `v4.0.0-rc.6`**, so the playbook's fallback applies. Move to v4 when it tags stable; the API is near-identical |
| 004 CLI | Cobra v1.10 + `charm.land/fang/v2` (v2.0.1), Fang only in `main.go` |
| 005 Licence | Apache-2.0 |
| 006 Name | **MockMachina**, stays in `tiny-tools` for now |

What 006 fixes everywhere:

| Thing | Value |
| --- | --- |
| Module path | `github.com/demola234/tiny-tools/mock_machina` |
| Binary | `mockmachina` |
| Entry point | `cmd/mockmachina/main.go` |
| Project folder users create | `.mockmachina/` |
| OpenAPI extension prefix | `x-mockmachina-*` |
| Release tags (Go sub-module rule) | `mock_machina/v0.1.0`, not `v0.1.0` |

Two more changes from the playbook: **`just` replaces `make`** (it runs the same way on the Windows CI runner) and **CI lives at the repo root** (`tiny-tools/.github/workflows/`), filtered to `mock_machina/**`.

---

## Task 0: Get the scaffold building

Right now `go build ./...` fails: `go.mod` is empty and `template/resolve.go` and `template/catalog.go` are 0-byte files with no `package` clause.

1. Install a current Go: `brew upgrade go` (1.27.1 is current; you have 1.23.2).
2. Reset the layout to match the playbook:

   ```text
   delete:  template/  importer/  tui/  runtime/  resource/  config/schema.go
   rename:  cmd/mock_machina/  →  cmd/mockmachina/
   keep:    internal/cli/root.go
   delete:  internal/cli/init.go, internal/cli/start.go   (Phase 1)
   create:  internal/model/  internal/config/  internal/clock/  internal/seed/
   ```

3. `go mod init github.com/demola234/tiny-tools/mock_machina`, then edit the `go` line to `go 1.25.0`.

**Tests first**

```go
// internal/cli/root_test.go
func TestNewRootCmd_UsesBinaryName(t *testing.T) {
	cmd := cli.NewRootCmd()
	if cmd.Use != "mockmachina" {
		t.Fatalf("Use = %q, want %q", cmd.Use, "mockmachina")
	}
}
```

Red (`NewRootCmd` doesn't exist), then write the smallest `NewRootCmd`, then green.

**Done when:** `go build ./... && go vet ./... && go test ./...` is clean.

## Task 1: ADRs

Six files in `docs/decisions/` (`001-contract-storage.md` … `006-name-and-module.md`), each with Context, Options, Decision, Consequences. Add a seventh, **`007-test-first.md`**, recording the rules above, so the working agreement is part of the project and not just this guide.

## Task 2: Repo plumbing and CI

Files: `justfile`, `.golangci.yml`, `.gitattributes`, `LICENSE`, `README.md`, `CONTRIBUTING.md`, `../.github/workflows/mock_machina.yml`.

**justfile recipes:** `tools`, `test` (`go test -race -shuffle=on -cover ./...`), `tdd PKG`, `lint`, `build`, `ci`, and `update-golden` (`go test ./... -update`). Put `set windows-shell := ["pwsh", "-c"]` at the top.

**CI outline:**

```yaml
on:
  push:          { paths: ["mock_machina/**", ".github/workflows/mock_machina.yml"] }
  pull_request:  { paths: ["mock_machina/**", ".github/workflows/mock_machina.yml"] }
jobs:
  test:
    strategy: { matrix: { os: [ubuntu-latest, macos-latest, windows-latest] } }
    runs-on: ${{ matrix.os }}
    defaults: { run: { working-directory: mock_machina } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: mock_machina/go.mod, cache-dependency-path: mock_machina/go.sum }
      # tidy check, vet, golangci-lint, test, build + run --version
```

**.gitattributes:** `testdata/** text eol=lf`, so Windows doesn't check out goldens with CRLF.

**Test the test runner.** TDD for CI means proving it goes red: push a branch with a deliberately failing assertion, confirm all three OS jobs fail, then remove it. A CI that can't fail protects nothing.

**Done when:** CI is green on all three OSes, and has been seen red once.

## Task 3: `main.go`, root command, `--version`

**Tests first**

1. `testdata/script/version.txtar`:

   ```text
   exec mockmachina --version
   stdout 'v0.0.0-dev'
   ```

2. `TestExitCode` table in `internal/cli`:

   | case | err | want |
   | --- | --- | --- |
   | success | `nil` | 0 |
   | plain error | `errors.New("x")` | 1 |
   | exit error | `&ExitError{Code: 2}` | 2 |
   | wrapped exit error | `fmt.Errorf("lint: %w", &ExitError{Code: 2})` | 2 |

   The last row forces you to use `errors.As`, not a type assertion.

**Then the code.** `main.go` stays tiny, and Fang appears only here:

```go
// cmd/mockmachina/main.go
var (
	version = "v0.0.0-dev" // set by -ldflags at build time
	commit  = "none"
)

func main() { os.Exit(cli.Run(context.Background(), os.Args[1:], version, commit)) }
```

`cli.Run` builds the root command, calls `fang.Execute(ctx, root, fang.WithVersion(version), fang.WithCommit(commit))`, and returns `ExitCode(err)`. testscript calls the same `cli.Run` (see task 10), so the script tests exercise the real wiring.

- `-ldflags "-X main.version=... -X main.commit=..."` only works on package-level string **variables**.
- Never call `os.Exit` outside `main`. It skips deferred cleanup and makes code untestable.
- Build the command tree in `NewRootCmd()`, not in `init()` with globals. Each test needs a fresh tree.

**Done when:** both tests pass and CI runs `mockmachina --version` on all three OSes.

---

Tasks 4, 5 and 11 don't depend on each other. Do them in any order once task 3 is green.

## Task 4: Model types (`internal/model`)

Standard library only. These are plain structs with yaml tags. Decoding tests live in `config` (task 6), because `model` can't import the YAML library.

**Tests first** (all in `model_test`):

- `TestStatus_Valid`, `TestProtocol_Valid`, `TestMode_Valid`, `TestServe_Valid`: tables of good and bad values.
- `TestProtocol_DefaultsToHTTP`: the zero value `Protocol("")` behaves as `http` through a `Effective()` method.
- `TestParseGRPCCode`: `"NOT_FOUND"` → 5, `"5"` → 5, `"not_found"` → error (names are case-sensitive), `"17"` → error.
- `TestStates_Lookup`: `States.Get("empty")` finds by name; `States.Names()` keeps file order.
- `TestRouteKey`: `"GET /users/{id}"` and `"GET /users/{userId}"` produce the same conflict key. Task 7's duplicate check depends on this.

**Design notes:**

- **Ordered states:** `type States []NamedState` with `NamedState{Name string; State State}`. A Go map would lose the file's order.
- **Enums:** `type Status string` plus constants and `Valid() bool`. Keep them as strings so the validator can report *all* bad values, not stop at the first decode failure.
- **`body` has three forms** (file name, inline value, `generate`): `Body{File string; Inline any; Generate bool}`, filled by the loader.
- `Source{File string; Line int}` on every value that can appear in an error message.
- `omitempty` on everything optional.

**Done when:** every enum, parser and helper has its table test, and `go test ./internal/model -cover` is close to 100%. Pure types make that easy.

## Task 5: Errors and suggestions (`internal/config`)

**Tests first:**

- `TestSuggest` table: `emtpy` vs `[success empty]` → `empty`; `mehtod` vs the route fields → `method`; `xyz` → `""`; an exact match → itself; an empty list → `""`.
- `TestProblem_String`: with a hint → `routes/a/route.yaml:7: msg (hint)`; without a hint, no trailing parentheses.
- `TestProblems_SortsByFileThenLine`.
- `TestProblems_Summary`: `"3 problems in 2 files"`, `"1 problem in 1 file"` (the singular form is a classic forgotten case; let the test catch it).
- `TestProblems_HasErrors`: warnings alone → false; with `strict` → true.

**Design notes:**

```go
type Problem struct {
	Severity Severity
	File     string // relative to .mockmachina, forward slashes
	Line     int
	Msg, Hint string
}
type Problems []Problem // implements error
```

- Call `filepath.ToSlash` once, when making a `Problem`. Write a test that feeds it a Windows-style path, so the behaviour is covered on every OS, not only on the Windows CI runner.
- Levenshtein distance ≤ 2 for suggestions: about 20 lines with a two-row table, no dependency.

## Task 6: Loader (`internal/config`)

```go
func LoadFS(fsys fs.FS) (*model.Project, Problems)          // what tests call
func Load(dir string) (*model.Project, Problems)             // return LoadFS(os.DirFS(dir))
```

Loading through `fs.FS` is the key TDD seam. Most loader tests build a project in memory with `fstest.MapFS` and never touch disk, so they're fast and identical on every OS. It also helps safety: `fs.ValidPath` rejects `..` and absolute paths, so a body path that escapes the project fails to open by construction.

**Tests first**, in this order. Each one forces the next piece of the loader:

1. `TestLoad_EmptyProject`: an empty `MapFS` gives zero routes and no problems.
2. `TestLoad_OneRoute`: one minimal `route.yaml` gives one route with its id, method and path.
3. `TestLoad_KeepsStateOrder`: states `c, a, b` in the file come back as `c, a, b`.
4. `TestLoad_RecordsLineNumbers`: `route.States.Get("empty").Src.Line == 9`.
5. `TestLoad_UnknownField`: `mehtod:` on line 3 → `routes/x/route.yaml:3: unknown field "mehtod" (did you mean "method"?)`.
6. `TestLoad_AllowsXFields`: `x-team: payments` gives no problem and is kept in `Extra`.
7. `TestLoad_EmptyFile`, `TestLoad_TabsIndent`: a friendly message, no panic.
8. `TestLoad_BodyFileRead`: `success.json` bytes end up on the state.
9. `TestLoad_BodyOutsideProject`: `body: ../secrets.json` gives the "points outside the project" message.
10. `TestLoad_CollectsEveryProblem`: a project with three mistakes returns exactly three problems, not one.
11. `TestLoad_ShopFixture`: the full `testdata/projects/shop` loads clean from disk through `Load(dir)`.

**Design notes:**

- Unmarshal into a `yaml.Node`, then walk it yourself: check for unknown keys (allow `x-*`), record `node.Line`, and `Decode` the known parts. `KnownFields(true)` would give the library's words, not yours.
- List the known fields by reading struct tags with `reflect`. Task 9 reuses the same helper.
- A top-level unmarshal gives a `DocumentNode`; the mapping is `doc.Content[0]`. An empty file gives `Kind == 0`.

## Task 7: Validation checks

Each check is `func(*model.Project, *Problems)` in a slice, so adding a check means one function and one slice entry.

**Tests first: one golden test that grows.** Write it before any check:

```go
func TestChecks_Golden(t *testing.T) {
	dirs, _ := filepath.Glob("../../testdata/projects/broken-*")
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			_, probs := config.Load(dir)
			golden.Assert(t, probs.String(), filepath.Base(dir)+".txt") // honours -update
		})
	}
}
```

Then for each row in the playbook's checks table:

1. Add `testdata/projects/broken-<check>/` with exactly that one mistake.
2. Hand-write the expected message in `testdata/golden/broken-<check>.txt`. Write it by hand, *before* the code exists. That's where you design the error message, and `-update` would only copy whatever the code happened to produce.
3. Red, then write the check, then green.

**Done when:** every row in the checks table has a fixture and a golden file, and `shop` still loads clean.

## Task 8: Safe writes

**Tests first:**

- `TestWriteFileAtomic_ReplacesContent`.
- `TestWriteFileAtomic_LeavesNoTempFiles`: after success, the folder holds only the target.
- `TestWriteFileAtomic_FailureKeepsOriginal`: make the write fail (pass a `writeFn` that returns an error; that seam exists for this test), then check the original bytes are unchanged and no temp file remains.
- `TestSetActive_KeepsCommentsAndOrder`: golden byte comparison of a commented `route.yaml` after changing `active`. Only that one line may differ.
- `TestRoundTrip`: load → write → load gives an equal project (`cmp.Diff` empty) for every good fixture.

**Design notes:** create the temp file in the **same directory** with `os.CreateTemp`, then `Sync`, `Close`, `os.Rename`. Edit the `yaml.Node` to keep comments, not the struct.

## Task 9: `schema/route.schema.json`

**Tests first.** Both exist before the schema has any content:

1. `TestSchema_CoversModel`: every yaml tag on `model.Route` and `model.State` (via task 6's reflect helper) appears in the schema's `properties`. Red immediately, which gives you the list of fields to write.
2. `TestSchema_FixturesValidate`: every good fixture's `route.yaml`, converted to JSON, validates with `santhosh-tekuri/jsonschema/v6` (v6.0.3, test-only for now).
3. `TestSchema_RejectsBrokenFixtures`: the structural broken fixtures (unknown field, bad enum) *fail* validation. This proves the schema has teeth.

Header line for route files:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/schema/route.schema.json
```

## Task 10: `lint` command

**Tests first: scripts before the command exists.**

```go
// internal/cli/script_test.go
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"mockmachina": func() { os.Exit(cli.Run(context.Background(), os.Args[1:], "v0.0.0-dev", "test")) },
	})
}
func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{Dir: "../../testdata/script"})
}
```

`testscript.Main` takes `func()`; older posts show `RunMain` with `func() int`.

Scripts, in order:

1. `lint_ok.txtar`: a clean project exits 0 and prints `no problems`.
2. `lint_broken.txtar`: three mistakes exit 1 and print all three lines and `3 problems in 2 files`.
3. `lint_warnings.txtar`: warnings only exit 0 and print them.
4. `lint_strict.txtar`: the same project with `--strict` exits 1.
5. `lint_default_dir.txtar`: no `--dir` finds `.mockmachina/` in the current folder.
6. `exit_phase0_invalid_route.txtar`: the playbook's exit criterion, verbatim.

```text
! exec mockmachina lint --dir proj
stdout 'routes/users.list/route.yaml:7: active state "emtpy"'
stdout '3 problems in 2 files'

-- proj/routes/users.list/route.yaml --
id: users.list
...
```

## Task 11: `clock`, `seed`, import rules

**Tests first:**

- `TestRandFor_SameInputsSameSequence`: two calls with equal inputs give equal first-10 values.
- `TestRandFor_DiffersByEachInput`: changing `route`, `call` or `purpose` changes the sequence (table over each field).
- `TestRandFor_IndependentOfGoroutineOrder`: 100 goroutines each compute call `i`; the results equal a serial run. Run with `-race`.
- `TestRealClock_SleepEndsOnCancel`: inside a `synctest` bubble, `Sleep(ctx, time.Hour)` returns `context.Canceled` once `ctx` is cancelled, with no real waiting.
- `TestImportRules`: from `go list -json ./...`, every package's imports are within its allow-list. Red the first time you deliberately add a forbidden import, so you know it works.

**Design notes:** `RandFor` hashes its inputs into two `uint64`s for `rand.NewPCG` (`math/rand/v2`). It's deterministic because the generator comes from the inputs, not from shared state.

## Task 12: Docs

**Tests first:** `TestDocs_ExamplesLoad` reads `docs/file-format.md`, extracts every fenced block tagged `yaml route`, writes each into a `fstest.MapFS` project, and loads it with no problems. Docs examples can then never drift from the real format.

Then write the doc, one complete route per protocol (HTTP, WS, SSE, gRPC, CRUD, rules), and get it reviewed by one backend and one frontend developer. That review freezes the format.

## Go habits worth setting from day one

- `gofmt` on save; golangci-lint runs `goimports`, `errorlint`, `thelper` and `tparallel` (the last two catch test-helper mistakes).
- Mark helpers with `t.Helper()` so failures point at the calling line.
- `t.TempDir()` for anything that must touch disk; `fstest.MapFS` for anything that doesn't.
- Wrap errors with `%w` so `errors.Is` and `errors.As` still work.
- `go test -race` locally before pushing.

## Progress

- [x] 0 Scaffold builds (first test green) — fe81932
- [x] 1 ADRs 001–007 (drafted, awaiting your edit)
- [ ] 2 Plumbing and CI (seen red once)
- [ ] 3 `--version`
- [ ] 4 Model
- [ ] 5 Errors and suggestions
- [ ] 6 Loader
- [ ] 7 Checks
- [ ] 8 Safe writes
- [ ] 9 Schema
- [ ] 10 `lint`
- [ ] 11 clock, seed, import rules
- [ ] 12 Docs
