# Project Structure

```
acm-caas-poc/
│
├── cmd/acmlab/                    # CLI entry point
│   ├── main.go                    # Cobra root command, .env loading, global flags
│   ├── fleet.go                   # fleet list, fleet status <name>
│   ├── provision.go               # provision create/destroy/status/list/image-sets
│   ├── policy.go                  # policy list/apply/status/remove
│   ├── tenant.go                  # tenant deploy/status/list/remove
│   ├── monitor.go                 # monitor list/status
│   └── mcp.go                     # mcp serve (MCP server on stdio)
│
├── internal/
│   ├── config/
│   │   ├── config.go              # Config struct, LoadFromEnv() with defaults
│   │   └── config_test.go         # Env parsing, defaults, validation errors
│   │
│   ├── client/
│   │   ├── client.go              # Dynamic k8s client wrapper (Create/Get/List/Delete/Patch/Watch)
│   │   ├── client_test.go         # CRUD operations with dynamicfake
│   │   ├── client_integration_test.go
│   │   └── gvr.go                 # GVR constants for all ACM resources
│   │
│   ├── fleet/                     # UC-04: Fleet observability
│   │   ├── fleet.go               # Inspector — ListClusters, GetCluster
│   │   └── fleet_test.go
│   │
│   ├── provisioning/              # UC-01: Cluster provisioning (multi-platform)
│   │   ├── provisioning.go        # Manager — Create, Destroy, Status, List, WaitForProvision
│   │   ├── builder.go             # Builds ClusterDeployment, ManagedCluster, KlusterletAddonConfig, Secrets
│   │   ├── credentials.go         # IBM Cloud CredentialsRequest definitions (5 components)
│   │   ├── ibmiam.go              # IBM Cloud IAM REST client (Service IDs, Policies, API Keys)
│   │   ├── ibmcreds.go            # Orchestrates IAM credential generation and cleanup
│   │   └── provisioning_test.go
│   │
│   ├── policy/                    # UC-02: Governance policy management
│   │   ├── policy.go              # Manager — Apply, Remove, Status, List, SetRemediation
│   │   ├── builder.go             # Builds Policy + PlacementRule + PlacementBinding
│   │   └── policy_test.go
│   │
│   ├── tenant/                    # UC-03: Tenant RBAC isolation
│   │   ├── tenant.go              # Manager — Deploy, Remove, Status, List
│   │   ├── builder.go             # Builds ManifestWork with Namespace/RoleBinding/NetworkPolicy/ResourceQuota
│   │   └── tenant_test.go
│   │
│   ├── monitoring/                # UC-06: Cluster resource monitoring
│   │   ├── monitoring.go          # Monitor — ListClusterResources, GetClusterResources
│   │   └── monitoring_test.go
│   │
│   ├── observability/             # UC-06: Thanos-based observability
│   │   ├── observability.go       # Manager — Enable, Disable, Status
│   │   ├── builder.go             # Builds MultiClusterObservability + object storage Secret
│   │   └── observability_test.go
│   │
│   └── mcp/
│       ├── server.go              # NewServer() — registers all MCP tools
│       └── server_test.go
│
├── features/                      # Gherkin .feature files (for godog)
│   └── fleet.feature              # UC-04 scenarios
│
├── docs/
│   ├── specs/
│   │   └── 2026-08-19-acm-caas-poc-design.md   # Design spec (7 UCs)
│   ├── adr/
│   │   ├── 001-dynamic-client-over-typed.md
│   │   ├── 002-mcp-server-for-interactive-testing.md
│   │   ├── 003-gherkin-driven-with-godog.md
│   │   ├── 004-env-based-configuration.md
│   │   ├── 005-uc-packages-as-controller-foundation.md
│   │   ├── 006-idempotent-operations.md
│   │   └── 007-minio-for-observability-object-storage.md
│   ├── project-structure.md        # This file
│   └── acmlab-commands.md          # CLI + MCP command reference
│
├── .env.example                    # Template for environment variables
├── .gitignore                      # .env, style.md, vendor/, bin/, docs/superpowers/
├── go.mod
└── go.sum
```

## Design Principles

- **Three consumers, one codebase:** CLI, MCP server, and (future) ComputeRequest controller all use the same `internal/<uc>/` packages
- **Dynamic client only:** `k8s.io/client-go/dynamic` — no typed ACM/Hive imports (see ADR-001)
- **Resource construction in Go maps:** `builder.go` files build unstructured resources — no YAML templates
- **Configuration from environment:** `.env` + `godotenv` (see ADR-004)
- **Idempotent operations:** All create/apply operations use createIfNotExists or update-or-create (see ADR-006)
- **Multi-platform provisioning:** Platform-specific logic (IBM Cloud IAM) isolated in dedicated files; shared flow (ManagedCluster, KlusterletAddonConfig) applies to all clouds
- **Build tags for test scope:** `go test ./...` = unit tests only. `integration` and `slow` tags for live hub tests (see ADR-003)
