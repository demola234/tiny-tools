# ADR 003: go.yaml.in/yaml/v3, moving to v4 when it is stable

- Status: Accepted
- Date: 2026-10-06

## Context

Route files are YAML. The loader needs the line number of every key to point errors at the right place, and the writer must change one value (for example `active`) without reformatting the file or losing comments. Both need a library that exposes the node tree.

The long-standing `gopkg.in/yaml.v3` path is frozen. Maintenance moved to the YAML organisation under `go.yaml.in/yaml`.

## Options

1. `gopkg.in/yaml.v3`: frozen, security fixes only.
2. `go.yaml.in/yaml/v4`: the maintained future. As of 2026-10-06 it has only release candidates (latest `v4.0.0-rc.6`).
3. `go.yaml.in/yaml/v3`: the same v3 API at the maintained path (`v3.0.5`).

## Decision

Option 3, `go.yaml.in/yaml/v3`. We move to v4 when `v4.0.0` is tagged.

## Consequences

- We get line numbers and comment-preserving edits through `yaml.Node`, from a maintained module.
- Only `internal/config` imports the YAML library, so the move to v4 touches one package. The round-trip and comment-preservation tests will show whether anything changed.
- Moving to v4 doesn't need a new ADR. It's recorded in the pull request that does it.
