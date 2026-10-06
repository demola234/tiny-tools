# Architecture decision records

Each file records one decision that is expensive to change: why it was needed, what was considered, what was chosen, and what follows from it. A decision is changed by adding a new ADR that supersedes the old one, never by editing the old one's Decision section.

| ADR | Decision | Status |
| --- | --- | --- |
| [001](001-contract-storage.md) | Contracts are stored as our own route files | Accepted |
| [002](002-template-engine.md) | Handlebars-like templates with our own parser | Accepted |
| [003](003-yaml-library.md) | `go.yaml.in/yaml/v3`, moving to v4 when it is stable | Accepted |
| [004](004-cli-stack.md) | Cobra with Fang, Fang confined to one function | Accepted |
| [005](005-licence.md) | Apache-2.0 | Accepted |
| [006](006-name-and-module.md) | MockMachina, inside the tiny-tools repository | Accepted |
| [007](007-test-first.md) | All code is written test-first | Accepted |

## Writing a new ADR

Copy the shape of an existing one: Context, Options, Decision, Consequences. Keep it to about a page. Number it next in sequence, add it to the table above, and link it from the pull request that depends on it.
