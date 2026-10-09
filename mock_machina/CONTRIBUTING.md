# Contributing

## Setup

You need:

- Go 1.25 or newer
- [just](https://github.com/casey/just)
- golangci-lint v2.14.0: run `just tools`, or install it another way as long as `golangci-lint --version` reports 2.14.0

Then run everything CI runs:

```sh
just ci
```

`just` with no arguments lists every recipe.

## How we write code

Read [docs/engineering.md](docs/engineering.md) first. The short version:

- **Test first, always** ([ADR 007](docs/decisions/007-test-first.md)). Write one failing test, see it fail for the right reason, write the least code that passes it, refactor, commit. `just tdd <package>` re-runs a package's tests on every save.
- **Keep it simple.** Clear code over clever code; the right data structure for how the data is used, with its cost stated.
- **Every bug fix starts with a test** that reproduces the bug.

## Golden files

Expected outputs live in `testdata/golden/`. When you change output on purpose:

```sh
just update-golden
git diff testdata/golden   # read every change before committing
```

Golden files for error messages are written by hand *before* the code that produces them, because that's where the message is designed.

## Decision records

Decisions that are expensive to change get an ADR in [docs/decisions/](docs/decisions/). Copy an existing one (Context, Options, Decision, Consequences), number it next, add it to the index, and link it from your pull request.

## Commits and pull requests

- Branches: `mockmachina/p<phase>-<task>-<slug>`, for example `mockmachina/p0-05-model`.
- Commits: [Conventional Commits](https://www.conventionalcommits.org/) with the `mockmachina` scope, for example `feat(mockmachina): load route files with line numbers`.
- A test and the code that makes it pass go in the same commit. Every commit builds and passes.
- One task per pull request. CI must be green on Linux, macOS and Windows before merging.
