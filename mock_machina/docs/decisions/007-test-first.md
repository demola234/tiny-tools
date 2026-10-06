# ADR 007: All code is written test-first

- Status: Accepted
- Date: 2026-10-06

## Context

MockMachina is a testing tool: teams will rely on it to tell them the truth about their apps. Its own behaviour (error messages, state selection, reload safety, exit codes) is mostly exact output that's easy to pin down with tests and easy to break without them. It also runs on three operating systems, where untested paths fail quietly.

## Options

1. Write tests after the code, aiming for a coverage number.
2. Write every piece of code test-first.

## Decision

Option 2. Every piece of production code is written test-first: red, green, refactor.

- **Red:** write one test for the next small behaviour, run it, and see it fail for the right reason. A compile error counts as red only for the first test in a new package.
- **Green:** write the least code that passes.
- **Refactor** with the tests green, then commit. Never commit a red test to `main`.
- Every bug fix starts with a failing test that reproduces the bug.
- CLI behaviour is tested with testscript (`testdata/script/*.txtar`) running the real command tree.
- Each phase starts with its exit criteria written as failing scripts (`exit_phaseN_*.txtar`).
- Golden files for error messages are written by hand before the check exists. `-update` is for intended changes, reviewed in the diff.

## Consequences

- The design has to provide test seams, and they're part of the architecture:
  - the loader reads an `fs.FS`;
  - time and randomness come from `clock` and `seed`;
  - servers take a `net.Listener`;
  - `cli.Run` returns an exit code instead of exiting.
- Tests never sleep to wait for something and never use fixed ports. They wait on events, use `testing/synctest`, and listen on `127.0.0.1:0`.
- Coverage is reported in CI per package as a signal for review, not as a gate.
- Progress is slower at first and much cheaper to change later. That fits a project that freezes its file format early and builds nine phases on top of it.
