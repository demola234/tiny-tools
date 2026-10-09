# ADR 005: Apache-2.0

- Status: Accepted
- Date: 2026-10-06

## Context

MockMachina is meant for teams, including companies, to adopt. The licence affects whether their legal reviews pass quickly. The tiny-tools repository has no licence yet, and other tools in it may choose differently.

## Options

1. **MIT:** short and permissive.
2. **Apache-2.0:** permissive, with an explicit patent grant and contribution terms.

## Decision

Apache-2.0, in `mock_machina/LICENSE`. It covers the `mock_machina/` directory only, not the rest of tiny-tools.

## Consequences

- The patent grant is the usual reason companies prefer Apache-2.0 for developer tools.
- Third-party code bundled later (for example Swagger UI in Phase 2, also Apache-2.0) ships with its own licence and a `NOTICE` entry.
- If MockMachina moves to its own repository (ADR 006), the licence moves with it unchanged.
