# acm-caas-poc

PoC that validates Red Hat ACM 2.17 capabilities for a Containers-as-a-Service platform. The code is structured to graduate into a ComputeRequest controller.

## Use Cases

| UC | Description | Status |
|----|-------------|--------|
| UC-01 | Cluster provisioning via ClusterDeployment (multi-platform) | Implemented |
| UC-02 | Governance policy management | Implemented |
| UC-03 | Tenant RBAC isolation via ManifestWork | Implemented |
| UC-04 | Fleet status and observability | Implemented |
| UC-05 | Hibernate/resume lifecycle (Hive-only) | Planned |
| UC-06 | Cluster resource monitoring & observability | Implemented |
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
bin/acmlab fleet status spoke1

# 5. Monitor cluster resources
bin/acmlab monitor list

# 6. Manage governance policies
bin/acmlab policy list
bin/acmlab policy apply my-policy --remediation enforce --registries "registry.redhat.io,quay.io"
bin/acmlab policy status my-policy
bin/acmlab policy remove my-policy

# 7. Deploy tenant isolation to a spoke
bin/acmlab tenant deploy team-alpha --cluster spoke1 --team alpha-devs --cpu 8 --memory 16Gi
bin/acmlab tenant status team-alpha --cluster spoke1
bin/acmlab tenant list --cluster spoke1
bin/acmlab tenant remove team-alpha --cluster spoke1

# 8. Provision a spoke cluster (IBM Cloud, AWS, GCP, Azure)
bin/acmlab provision image-sets
bin/acmlab provision create spoke1 --pull-secret ~/pull-secret.json --region us-south
bin/acmlab provision create spoke2 --platform aws --pull-secret ~/pull-secret.json --region us-east-1
bin/acmlab provision status spoke1
bin/acmlab provision list
bin/acmlab provision destroy spoke1
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

Available tools: `acm_fleet_status`, `acm_list_managed_clusters`, `acm_get_managed_cluster`, `acm_hub_health`, `acm_list_cluster_resources`, `acm_cluster_resources`, `acm_list_policies`, `acm_get_policy`, `acm_apply_policy`, `acm_remove_policy`, `acm_set_policy_remediation`, `acm_provision_create`, `acm_provision_destroy`, `acm_provision_status`, `acm_provision_list`, `acm_list_image_sets`.

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
