# acm-caas-poc

PoC that validates Red Hat ACM 2.17 capabilities for a Containers-as-a-Service platform. The code is structured to graduate into a ComputeRequest controller.

## Use Cases

| UC | Description | Status |
|----|-------------|--------|
| UC-01 | Cluster provisioning via ClusterDeployment | Planned |
| UC-02 | Image registry restriction policy | Planned |
| UC-03 | Tenant RBAC isolation via ManifestWork | Planned |
| UC-04 | Fleet status and observability | Implemented |
| UC-05 | Hibernate/resume lifecycle (Hive-only) | Planned |
| UC-06 | Cluster resource monitoring | Planned |
| UC-07 | External cluster import | Planned |

## Quick Start

```bash
# 1. Copy and fill environment config
cp .env.example .env
# Edit .env with your kubeconfig path, context, and credentials

# 2. Build
go build -o bin/acmlab ./cmd/acmlab/

# 3. List managed clusters
bin/acmlab fleet list

# 4. Get cluster details
bin/acmlab fleet status infraops1
```

## MCP Server

The MCP server exposes ACM operations as tools for Claude Code.

```bash
# Start the MCP server (stdio transport)
bin/acmlab mcp serve
```

Register in Claude Code's MCP config:

```json
{
  "mcpServers": {
    "acmlab": {
      "command": "/path/to/bin/acmlab",
      "args": ["mcp", "serve"]
    }
  }
}
```

Available tools: `acm_fleet_status`, `acm_list_managed_clusters`, `acm_get_managed_cluster`, `acm_hub_health`.

See [docs/acmlab-commands.md](docs/acmlab-commands.md) for the full command and tool reference.

## Testing

```bash
# Unit tests only
go test ./...

# Integration tests (requires live hub connection)
go test ./... -tags=integration

# Full lifecycle tests (slow, requires provisioning access)
go test ./... -tags=slow
```

## Architecture

Three consumers share the same `internal/<uc>/` packages:

```
CLI (cmd/acmlab) ──┐
MCP server ────────┤── internal/fleet, provisioning, policy, ...
Controller (future)┘
```

All hub interaction uses `k8s.io/client-go/dynamic` — no typed ACM imports. See [ADR-001](docs/adr/001-dynamic-client-over-typed.md).

## Documentation

- [Design spec](docs/specs/2026-08-19-acm-caas-poc-design.md)
- [Project structure](docs/project-structure.md)
- [CLI & MCP commands](docs/acmlab-commands.md)
- [ADRs](docs/adr/)
