# ADR 006: MockMachina, inside the tiny-tools repository

- Status: Accepted
- Date: 2026-10-06

## Context

The name appears in the module path, the binary, the folder users create in their projects, the OpenAPI extension prefix and release tags. Early planning used two spellings ("MockMechina" and "Mock-Machina"), and assumed a repository of its own.

## Options

1. Keep the planning name "MockMechina" in a new repository.
2. "MockMachina" in a new repository.
3. "MockMachina" inside tiny-tools, moving out if it grows.

## Decision

Option 3.

| Thing | Value |
| --- | --- |
| Product name | MockMachina |
| Module path | `github.com/demola234/tiny-tools/mock_machina` |
| Binary | `mockmachina` |
| Entry point | `cmd/mockmachina/main.go` |
| Project folder users create | `.mockmachina/` |
| OpenAPI extension prefix | `x-mockmachina-*` |
| Release tags | `mock_machina/vX.Y.Z` (Go's rule for modules in a subdirectory) |

## Consequences

- Planning documents that say "MockMechina" or `.mockmechina` mean the values above.
- CI lives at the repository root and runs only for changes under `mock_machina/**`.
- All Go code is under `internal/`, so no one can import our packages. Moving to a new repository later changes only our own import paths, plus the install command in the docs.
- Before the first public release, check that the name is free on Homebrew, ghcr.io and Docker Hub, and decide on a domain for docs and the published schema. Until then, the schema URL points at the raw file on GitHub.
