# ADR-003: Gherkin-driven development with godog

## Status

Accepted

## Context

The lab validates ACM capabilities for the ComputeRequest controller. We need a way to specify what each capability does (for the team and for Jira) and verify it works against a live hub. Options:

1. **Standard Go tests only** — `_test.go` files with assertions
2. **Gherkin specs (documentation only)** — `.feature` files as documentation, tests separate
3. **Executable Gherkin with godog** — `.feature` files that run as integration tests

## Decision

Use godog (`github.com/cucumber/godog`) to execute `.feature` files as integration tests. Feature files live in `features/`, step definitions in `integration/`. Two build tags control scope:

- `//go:build integration` — quick API tests (create resource, verify accepted). ~5 min.
- `//go:build slow` — full lifecycle tests (provision, wait, verify). ~60 min.

`go test ./...` runs only unit tests. Integration tests require explicit tags.

## Consequences

- **Pro:** Use cases are human-readable — shareable with the team and attachable to Jira tickets
- **Pro:** Same scenarios serve as documentation and as executable tests
- **Pro:** Step definitions call the same UC packages as CLI and MCP — single code path
- **Pro:** Build tags prevent accidentally running 60-minute tests
- **Con:** godog adds a dependency and a slight learning curve
- **Con:** Step definitions are more verbose than direct test assertions
