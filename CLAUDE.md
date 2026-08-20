# ACM CaaS PoC

Go project validating ACM capabilities for CaaS. Code graduates into the ComputeRequest controller.

## Style

Follow conventions from `~/claude/lab/docs/style.md` (not committed to this repo):
- Pure functions for logic, structs for I/O — never mixed
- Factory functions (`New*`) for structs
- Error wrapping with `fmt.Errorf` and `%w`
- Guard clauses over deep nesting
- Tests named after behavior, table-driven when multiple inputs
- No utils/helpers/common packages
- Import ordering: stdlib, third-party, local

## Project Rules

- **Dynamic client only** — `k8s.io/client-go/dynamic`, no typed ACM/Hive imports
- **No os/exec** — everything through the Go API, never shell out to kubectl/oc
- **Credentials via .env** — loaded by godotenv, never hardcoded
- **Build tags** — `//go:build integration` for API tests, `//go:build slow` for provisioning waits
- **MCP long-running tools** return immediately, caller polls with status tools
- **Resource construction** in `builder.go` via unstructured maps, not YAML templates

## Commands

```bash
go test ./...                                          # unit tests
go test -tags integration ./integration/... -timeout 10m  # API tests
go test -tags slow ./integration/... -timeout 90m         # full e2e
go build -o bin/acmlab ./cmd/acmlab                    # build CLI
./bin/acmlab mcp serve                                 # start MCP server
```

## Module

`github.com/pablofelix/acm-caas-poc` — Go 1.22
