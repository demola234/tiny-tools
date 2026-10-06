# ADR 002: Handlebars-like templates with our own parser

- Status: Accepted
- Date: 2026-10-06

## Context

From Phase 5, response bodies can be templates: echo a path parameter, insert fake data, loop over a list. People write these by hand in JSON files, so the syntax must be familiar and read well inside JSON, for example `{"name": "{{fake 'person.fullName'}}"}`.

## Options

1. **Go `text/template`.** Built in, but its syntax (`{{ .Params.id }}`, `{{ fake "person.fullName" }}`) is unfamiliar to frontend developers, and single-quoted string arguments aren't valid.
2. **A Handlebars library for Go.** The established one was archived in June 2025, so we would own an unmaintained dependency.
3. **Handlebars-like syntax with our own small parser.**

## Decision

Option 3. Templates use a Handlebars-like syntax, parsed by our own code in `internal/template`.

## Consequences

- We own a parser. The grammar is kept deliberately small (expressions, helper calls with arguments, a few block helpers) and is specified in Phase 5 planning before any code is written.
- Templates are parsed when the project loads, so syntax errors are reported with file and line through the normal loader errors, never at request time.
- The parser is a natural fuzzing target. It is fuzzed in CI from the start.
- A state can turn templating off with `template: false` for bodies that contain literal `{{`.
