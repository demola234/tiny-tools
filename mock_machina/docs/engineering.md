# Engineering standards

How MockMachina's Go code is written. These rules apply to every phase and every pull request. Each one has a reason; if a rule gets in the way of good code, change the rule here in a pull request instead of quietly breaking it.

Related: [ADR 007](decisions/007-test-first.md) (test-first), [ADR 004](decisions/004-cli-stack.md) (CLI stack), [ADR 006](decisions/006-name-and-module.md) (module and naming).

## 1. Toolchain and dependencies

| Item | Rule |
| --- | --- |
| `go` directive | `go 1.25.0`. This is the oldest Go we support for `go install`. Raising it is a deliberate change in its own pull request |
| Language and library features | Nothing newer than Go 1.25, even though local toolchains are newer. `go vet` (the `stdversion` check) and gopls flag standard-library calls that are too new |
| CI Go versions | Every test job runs twice: with the `go.mod` version (`1.25.x`) and with `stable` |
| golangci-lint | v2.14.0, installed as a binary (`just tools`) and in CI through `golangci/golangci-lint-action`. Not through `go tool`: upstream advises against it, because its dependencies clash with ours |
| Go-based dev tools | Run with `go run pkg@version`, with versions pinned in the `justfile` (gotestsum, govulncheck, actionlint). **Not** with the `tool` directive: tool dependencies join our module graph, and govulncheck v1.8.0 alone raised our `go` line to 1.26 |
| New dependency | Needs a sentence in the pull request on why the standard library isn't enough. Prefer small, maintained, permissively licensed modules with few dependencies of their own |
| `go.mod` hygiene | `go mod tidy -diff` must print nothing (checked in CI) |
| Vulnerabilities | govulncheck runs in CI on every pull request (on stable Go, since the tool itself needs 1.26) |

Test-only dependencies are allowed in `_test.go` files: `github.com/google/go-cmp`, `github.com/rogpeppe/go-internal/testscript`, `github.com/santhosh-tekuri/jsonschema/v6`. We don't use testify. The standard library plus `cmp.Diff` gives clearer failures with less magic.

## 2. Layout

```text
mock_machina/
├── cmd/mockmachina/      main only: signals, build info, os.Exit
├── internal/<package>/   all code; nothing is importable from outside
├── internal/testkit/     helpers used only by tests
├── testdata/             fixtures shared by several packages (projects, golden, script)
├── schema/               published JSON Schemas
└── docs/                 user docs, decisions/
```

- **No `pkg/`, `util`, `common`, `helpers`, `types` or `misc` packages.** A package is named for what it provides (`config`, `seed`), and you should be able to describe it in one sentence without "and".
- **One package, one job.** Packages are split by responsibility, not by kind of thing. There's no `errors` package or `interfaces` package.
- **Every package has a `doc.go`** whose package comment says what the package does and what it never does (for example, "model has no file access").
- **Package-local fixtures** go in that package's `testdata/`. Fixtures used by several packages go in the module-root `testdata/`, reached through `testkit.Path`.

## 3. Naming

- **Packages:** short, lowercase, one word, no underscores.
- **No stutter:** `config.Load`, not `config.LoadConfig`; `model.Route`, not `model.RouteModel`.
- **Initialisms keep their case:** `ID`, `HTTP`, `URL`, `JSON`, `YAML`, `RPC`, `GRPC`. So `RouteID` and `ServeHTTP`, never `RouteId`.
- **No `Get` prefix on getters:** `p.Route(id)`, not `p.GetRoute(id)`.
- **Receivers:** one or two letters, the same for every method of a type (`r *Route`, `ps Problems`). Never `this` or `self`.
- **Errors:** sentinel values are `ErrSomething`; error types are `SomethingError`.
- **Single-method interfaces** end in `-er` when that reads naturally.
- **Tests:** `Test<Thing>_<Behaviour>`, such as `TestSuggest_ReturnsClosestWithinTwoEdits`. Fuzz tests are `Fuzz<Thing>`; benchmarks are `Benchmark<Thing>`.
- **Files** are named for the main type or job inside them (`problem.go`, `suggest.go`), with tests alongside (`problem_test.go`).

## 4. API design

- **Accept interfaces, return concrete types.** An interface is declared in the package that *uses* it, holds only the methods that package needs, and appears only once there are two implementations or a test needs a seam.
- **Make the zero value useful.** Write a constructor only when a type has invariants the zero value can't satisfy.
- **Configuration structs, not functional options.** All our APIs are internal, so a plain struct with documented defaults is simpler and easier to read in tests.
- **No mutable package-level state and no `init()`.** Package-level variables are allowed only for sentinel errors, compiled regular expressions and fixed lookup tables. Dependencies (clock, writers, file systems) are passed in. Enforced by the `gochecknoinits` linter.
- **No `panic` for expected failures.** Panics signal programmer mistakes (impossible states). The server recovers from them per request (Phase 1); nothing else recovers.
- **Return early.** Handle the error or edge case first and keep the main path at the left margin.
- **Slices and maps from callers** are not kept or modified unless the doc comment says so. Copy them when you keep them.
- **Typed nil and interfaces:** never return a typed nil pointer as an `error`. For the same reason, `config.Problems` deliberately doesn't implement `error`.

## 5. Errors

- **Wrap with context using `%w`:** `fmt.Errorf("reading %s: %w", name, err)`. Context says what we were doing; it doesn't start with "failed to" or "error".
- **Error strings are lowercase with no final punctuation,** because they get wrapped into longer messages.
- **Handle an error once.** Either return it (wrapped) or report it, never both. Library packages never log.
- **Callers check with `errors.Is` and `errors.As`,** never by comparing strings. `errorlint` enforces this.
- **Content problems are not errors.** A mistake in a user's project is a `config.Problem` with a file, line, code, message and hint. A Go `error` means MockMachina itself couldn't do its job (for example, the folder couldn't be read).
- **Messages users see** follow the playbook's format: `<file>:<line>: <what's wrong> (<how to fix>)`, in the project's own words, never a raw library message.

## 6. Context and cancellation

- `ctx context.Context` is the first parameter of anything that blocks, waits or does I/O that may be slow.
- A context is never stored in a struct, except in a type whose whole lifetime is one request.
- Tests use `t.Context()`, which is cancelled when the test ends.
- Every wait honours cancellation. `clock.Sleep(ctx, d)` returns `ctx.Err()` when cancelled.

## 7. Concurrency

- **Every goroutine has an owner and a way to stop.** Start them with `errgroup.Group` or `sync.WaitGroup.Go`, and make sure whoever starts one also waits for it.
- **A mutex sits directly above the fields it guards,** with a comment naming them. Prefer the typed atomics (`atomic.Pointer[T]`, `atomic.Uint64`) for single values.
- **Whoever sends on a channel closes it.** Channels in APIs say their direction (`<-chan Event`).
- **No `time.Sleep`, `time.Now`, `time.After` or timers outside `internal/clock`,** and no `math/rand` (v1) anywhere. All randomness comes through `internal/seed`. Enforced by `forbidigo` and `depguard`.
- **`go test -race` in CI on every OS** that supports it.

## 8. Output and logging

- **Only `internal/cli` writes to the terminal,** and only through the command's writers (`cmd.OutOrStdout()`, `cmd.ErrOrStderr()`). Library packages return values, errors and `Problems`, or publish events (from Phase 1).
- **Results go to stdout; diagnostics go to stderr.** `lint`'s problem list is its result, so it goes to stdout. A failure to run goes to stderr.
- **Machine-readable output** (`--json`, `--format json`) is stable. Changing its shape is a breaking change and needs a changelog entry.
- **Colour only when stdout is a terminal,** and never when `NO_COLOR` is set.
- `fmt.Print*` and `println` are banned outside `cmd/` by `forbidigo`.

## 9. Files and paths

- **`path/filepath` for paths on disk; `path` for slash paths** (paths inside an `fs.FS`, and every path shown to users).
- **Paths shown to users** are relative to the `.mockmachina` folder and always use forward slashes, on every OS.
- **Reading goes through `fs.FS`** (`os.DirFS` in production, `fstest.MapFS` in tests). `fs.ValidPath` rejects `..` and absolute paths, so escaping the project fails by construction.
- **Only `internal/config` writes project files,** always with a temp file plus rename (`config.WriteFileAtomic`). Permissions are `0o644` for files and `0o755` for folders.
- **Code never changes the working directory.** Tests use `t.Chdir`.
- **Line endings:** we read files with either LF or CRLF and keep whichever a file already uses when editing it.

## 10. Testing

ADR 007 sets the rules (test-first, red before green). These are the conventions:

- **External test packages by default** (`package config_test`). An internal test is for an unexported helper that is worth testing alone.
- **Table-driven tests** with named cases and `t.Run(tc.name, ...)`. Call `t.Parallel()` in the test and in each subtest unless the test changes process state (`t.Chdir`, `t.Setenv`). `tparallel` checks consistency.
- **Failure messages** read `Func(input) = got, want want`. Comparisons use `cmp.Diff(want, got)` and print `(-want +got)`.
- **Helpers** call `t.Helper()` first. Cleanup uses `t.Cleanup`. Temporary folders use `t.TempDir()`.
- **Golden files** live in `testdata/golden/`. `testkit.Golden(t, got, name)` compares and rewrites on `-update`. Error-message goldens are written by hand before the code exists.
- **No sleeping to wait,** and no real time in timing tests: use `testing/synctest`. **No fixed ports:** listen on `127.0.0.1:0`. **No internet access** in tests.
- **Fuzz tests** (`FuzzX`) for every parser of user input. Seed them from `testdata`, and add each crash they find as a regular test case.
- **Benchmarks** use `for b.Loop()` (Go 1.24+). Hot paths get one from Phase 1.
- **CLI behaviour** is tested with testscript scripts in `testdata/script/`.
- **Coverage** is printed per package in CI. It's a review signal, not a gate.

## 11. Comments and documentation

- **Every exported identifier has a doc comment** that starts with its name and is a full sentence. `revive` checks this.
- **Comments explain why,** not what the next line does.
- **`TODO`s carry an issue reference,** such as `// TODO(#12): ...`. Commented-out code is deleted, not kept.
- **A `//nolint` comment names the linter and gives a reason,** for example `//nolint:gosec // path is validated by fs.ValidPath above`. `nolintlint` enforces both.

## 12. Formatting and linting

- **Formatting:** `gofumpt` (a stricter `gofmt`) and `goimports` with `github.com/demola234/tiny-tools/mock_machina` as the local import group. golangci-lint runs both as formatters; `just fmt` applies them.
- **The lint rules** are in `.golangci.yml`. Linting must be clean before review.

## 13. Commits, branches and pull requests

- **Branches:** `mockmachina/p<phase>-<task>-<slug>`, for example `mockmachina/p0-05-model`.
- **Commits:** Conventional Commits, with `mockmachina` as the scope so they read clearly in the tiny-tools history: `feat(mockmachina): load route files with line numbers`. Types: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `ci`, `build`.
- **Each commit builds and passes tests.** A failing test and the code that makes it pass go in the same commit.
- **One task per pull request,** small enough to review in one sitting. The description links the task in the phase spec and lists the tests added.
- **Merging** needs green CI on all three OSes.

## 14. Versioning and releases

- **Semantic Versioning.** Tags are `mock_machina/vX.Y.Z` (ADR 006).
- **The version shown to users** comes from `internal/buildinfo`. It reads Go's embedded build information (module version, commit, dirty flag), and release builds may override the version with `-ldflags "-X .../internal/buildinfo.version=vX.Y.Z"`.
- **Breaking changes** need a changelog entry and, for the file format after the Phase 0 freeze, a new ADR and a migration. These include: the route file format, CLI flags, exit codes, JSON output, HTTP error bodies and the `X-Mock-*` headers.
- **`CHANGELOG.md`** follows Keep a Changelog from v0.1.0.

## 15. Clean, simple code

The simplest code that passes the tests and reads clearly wins. Cleverness has to earn its place with a benchmark.

- **One job per function, one level of abstraction inside it.** If you need "and" to describe a function, split it. A function that mixes high-level steps with byte-level details gets those details extracted.
- **Short and flat.** Aim for functions that fit on one screen. Use early returns instead of nesting. golangci-lint enforces limits: `gocognit` at 15, `nestif` at 4. Exceeding them is a refactoring signal, not something to `//nolint`.
- **Few parameters.** More than four is a sign that a struct or a type wants to exist. No boolean flag parameters that change what a function does (`load(path, true)`). Write two functions, or pass a named option field.
- **Names do the explaining.** A good name removes the need for a comment. Short names (`i`, `r`, `ctx`) are for small scopes; longer scopes get descriptive names.
- **No premature abstraction.** An interface, generic or helper appears on the third repetition (the rule of three) or when a test needs a seam, not before. Duplication is cheaper than the wrong abstraction.
- **No speculative features.** Build what the current task's tests ask for. Fields that the file format must accept early are *parsed and stored*, but no behaviour is written for them until their phase.
- **Delete freely.** Dead code, unused parameters (`unparam`) and stale comments are removed in the same pull request that makes them dead.
- **Refactor while green.** The refactor step of red-green-refactor is where clean code happens, and it's never skipped.
- **Clear beats clever.** If a reviewer has to run the code in their head twice, rewrite it or add the one comment that explains why.

## 16. Data structures and algorithms

Pick the structure that fits how the data is *used* (looked up, iterated in order, searched by prefix, streamed), know its cost, and keep it simple until a benchmark says otherwise.

**How to choose**

1. **Know the access pattern first.** Lookup by key → map. Ordered iteration of a small set → slice. Both → slice plus an index map, built once.
2. **Know the cost.** Any function that isn't obviously O(n) states its complexity in its doc comment, such as `// O(n·k) where k is the max distance`.
3. **Small n favours slices.** Linear scans over fewer than about 16 items beat maps (no hashing, cache-friendly). Most routes have a handful of states.
4. **Do work once.** Compile, index and sort at load time, not per request. The snapshot exists so the request path only reads.
5. **Standard library first:** `slices`, `maps`, `sort`, `strings.Builder`, `bytes.Buffer`, `container/heap`, `sync.Pool`. Write a custom structure only when a benchmark shows the standard one is the bottleneck.
6. **Measure before optimising.** Benchmarks use `b.Loop()`; claims about speed in a pull request come with `benchstat` output.
7. **Bound the work.** Anything driven by user input (recursion depth, regex, body size, list length) has a documented limit, so a strange file can't hang the loader or the server.
8. **Preallocate** when the final size is known: `make([]T, 0, n)`. `prealloc` flags the obvious cases.

**Choices already made**

| Problem | Structure / algorithm | Cost | Why this and not something fancier |
| --- | --- | --- | --- |
| Ordered states with lookup by name | `[]NamedState`, linear `Get` | O(s), s ≈ 2–8 | Faster than a map at this size, and keeps file order for free |
| Routes by id | `[]*Route` kept sorted by id; `slices.BinarySearchFunc` for lookup | O(log r) lookup, ordered listing | One structure serves both access patterns, so there's no second index to keep in sync |
| Typo suggestions | Levenshtein with a cutoff of 2: skip candidates whose length differs by more than 2, keep two rows, stop when a row's minimum exceeds 2 | O(c·n) worst case, usually much less | Candidate lists are short (field names, state names); a BK-tree or trie would be more code than the whole check |
| Duplicate method + path | `map[conflictKey]routeID` | O(r) | One pass, and both route ids are named in the message |
| Sorting problems | `slices.SortStableFunc` with `cmp.Compare` chains (file, line, code, message) | O(p log p) | Deterministic output for goldens |
| Known fields per type | Reflect once per type, cached with `sync.OnceValue` | O(1) after first use | The decoder asks for the same few types thousands of times |
| Request routing (Phase 1) | `net/http.ServeMux`, a fresh one per snapshot | O(path length) | The standard library's pattern matching is fast and well tested; no router library needed |
| Snapshot swap (Phase 1) | `atomic.Pointer[Snapshot]` | O(1), lock-free reads | Readers never wait for a reload |
| Closest routes for 404s (Phase 1) | Distance against routes with the same method first, keeping the best 3 in a fixed-size array | O(r·L) | n is small; a full sort of all routes is wasted work |
| Event bus (Phase 1) | One buffered channel per subscriber, non-blocking send | O(subscribers) per event | Simple, and a slow subscriber can't block a request |
| Reload debounce (Phase 1) | One timer reset on each file event | O(1) per event | Groups bursts of editor writes |
| Deterministic randomness | SHA-256 of length-prefixed inputs → PCG | O(input length) | Stateless, so no lock and no dependence on goroutine order |
