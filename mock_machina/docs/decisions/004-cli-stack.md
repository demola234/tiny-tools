# ADR 004: Cobra with Fang, Fang confined to one function

- Status: Accepted
- Date: 2026-10-06

## Context

MockMachina is used mostly from a terminal. It needs subcommands, flags, shell completion, `--version`, and help and errors that are pleasant to read. Every command must also run without prompts in scripts and CI.

## Options

1. **Plain Cobra** (`github.com/spf13/cobra`).
2. **Cobra with Charm's Fang** (`charm.land/fang/v2`), which adds styled help and errors, `--version`, completions and man pages to a Cobra command.

## Decision

Option 2: Cobra v1.10 with Fang v2. Other Charm libraries are used at v2 (`charm.land/log/v2`, and `charm.land/huh/v2` for prompts from Phase 1).

Fang is called in exactly one place, `cli.Run`. `main.go` calls `cli.Run`, and so do the testscript tests, so the tests exercise the real wiring.

## Consequences

- Styled help, errors and completions with almost no code of our own.
- Fang's authors mark it experimental. Because it's called in one function, removing it means replacing one call with `root.ExecuteContext(ctx)`.
- This changes the playbook's wording ("Fang only in `main.go`") to "Fang only in `cli.Run`", which serves the same purpose and keeps the entry point testable.
- Commands never call `os.Exit`. They return errors, and `cli.ExitCode` maps them to exit codes in one place.
