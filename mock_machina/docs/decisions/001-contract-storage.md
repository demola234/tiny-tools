# ADR 001: Contracts are stored as our own route files

- Status: Accepted
- Date: 2026-10-06

## Context

Everything MockMachina does reads the contract: the mock server, lint, import and export, diffs in pull requests, the dashboard. The storage format decides how readable those diffs are, which protocols we can describe, and where mock-only data (states, the active state, rules) lives. Changing it later would touch every package.

## Options

1. **Own route files.** One folder per route under `.mockmachina/routes/<id>/`, holding a `route.yaml` and its state bodies. OpenAPI and Postman are import and export formats.
2. **OpenAPI as the stored format,** with a sidecar file for states and other mock-only data.

## Decision

Option 1. Contracts are stored as our own route files, one folder per route.

## Consequences

- A change to one endpoint is a diff in one small file, which is easy to review in a pull request.
- States, rules, CRUD, WebSocket, SSE and gRPC routes are described natively instead of through extensions or a second file that can drift from the first.
- OpenAPI support has to be built as import and export (Phase 2). To make round trips lossless, MockMachina-only fields travel as `x-mockmachina-*` extensions and unknown `x-` fields are kept unchanged.
- The format has to be designed carefully and frozen at the end of Phase 0. `config.yaml` carries a `version` field so later releases can migrate old projects.
- We publish `schema/route.schema.json` so editors can autocomplete and check route files.
