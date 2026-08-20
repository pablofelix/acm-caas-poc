# Project Structure

```
acm-caas-poc/
│
├── cmd/acmlab/                    # CLI entry point
│   ├── main.go                    # Cobra root command, .env loading, global flags
│   ├── fleet.go                   # fleet list, fleet status <name>
│   ├── mcp.go                     # mcp serve (MCP server on stdio)
│   ├── provision.go               # (planned) provision create/delete/status
│   ├── policy.go                  # (planned) policy create/delete/compliance
│   ├── workload.go                # (planned) workload deploy/undeploy/status
│   ├── lifecycle.go               # (planned) lifecycle hibernate/resume
│   ├── monitor.go                 # (planned) monitor list/status
│   └── import.go                  # (planned) import cluster/detach/status
│
├── internal/
│   ├── config/
│   │   ├── config.go              # Config struct, LoadFromEnv() with defaults
│   │   └── config_test.go         # Env parsing, defaults, validation errors
│   │
│   ├── client/
│   │   ├── client.go              # Dynamic k8s client wrapper (Create/Get/List/Delete/Patch/Watch)
│   │   ├── client_test.go         # CRUD operations with dynamicfake
│   │   └── gvr.go                 # GVR constants for all ACM resources
│   │
│   ├── fleet/                     # UC-04: Fleet observability
│   │   ├── fleet.go               # Inspector — ListClusters, GetCluster
│   │   └── fleet_test.go          # Fake ManagedCluster parsing tests
│   │
│   ├── provisioning/              # (planned) UC-01: Cluster provisioning
│   │   ├── provisioning.go        # Provisioner — CreateCluster, DeleteCluster, GetStatus
│   │   ├── builder.go             # Builds ClusterDeployment + install-config as unstructured maps
│   │   └── provisioning_test.go
│   │
│   ├── policy/                    # (planned) UC-02: Image registry policy
│   │   ├── policy.go              # Manager — CreatePolicy, DeletePolicy, GetCompliance
│   │   ├── builder.go             # Builds Policy + PlacementRule + PlacementBinding
│   │   └── policy_test.go
│   │
│   ├── workload/                  # (planned) UC-03: Tenant RBAC isolation
│   │   ├── workload.go            # Deployer — Deploy, Undeploy, GetStatus
│   │   ├── builder.go             # Builds ManifestWork with Namespace/RoleBinding/NetworkPolicy/ResourceQuota
│   │   └── workload_test.go
│   │
│   ├── lifecycle/                 # (planned) UC-05: Hibernate/resume
│   │   ├── lifecycle.go           # Manager — Hibernate, Resume, GetPowerState
│   │   └── lifecycle_test.go
│   │
│   ├── monitoring/                # (planned) UC-06: Cluster resource monitoring
│   │   ├── monitoring.go          # Monitor — GetClusterResources, ListClusterResources
│   │   └── monitoring_test.go
│   │
│   ├── importing/                 # (planned) UC-07: External cluster import
│   │   ├── importing.go           # Importer — ImportCluster, DetachCluster, GetImportStatus
│   │   ├── builder.go             # Builds ManagedCluster + KlusterletAddonConfig + auto-import Secret
│   │   └── importing_test.go
│   │
│   └── mcp/
│       ├── server.go              # NewServer() — registers all MCP tools
│       ├── server_test.go         # Server construction tests
│       ├── tools_crud.go          # (planned) Low-level CRUD tools per resource type
│       └── tools_uc.go            # (planned) High-level UC flow tools
│
├── features/                      # Gherkin .feature files (for godog)
│   ├── fleet.feature              # UC-04 scenarios
│   ├── provisioning.feature       # (planned) UC-01
│   ├── policy.feature             # (planned) UC-02
│   ├── workload.feature           # (planned) UC-03
│   ├── lifecycle.feature          # (planned) UC-05
│   ├── monitoring.feature         # (planned) UC-06
│   └── importing.feature          # (planned) UC-07
│
├── integration/                   # (planned) godog step definitions
│   ├── suite_test.go              # Test runner with build tags
│   ├── fleet_test.go              # Step definitions for fleet.feature
│   └── ...                        # One file per UC
│
├── manifests/                     # (planned) Reference YAMLs — documentation only, not rendered
│   ├── clusterdeployment/
│   ├── policy/
│   └── manifestwork/
│
├── docs/
│   ├── specs/
│   │   └── 2026-08-19-acm-caas-poc-design.md   # Design spec (7 UCs)
│   ├── adr/
│   │   ├── 001-dynamic-client-over-typed.md
│   │   ├── 002-mcp-server-for-interactive-testing.md
│   │   ├── 003-gherkin-driven-with-godog.md
│   │   ├── 004-env-based-configuration.md
│   │   └── 005-uc-packages-as-controller-foundation.md
│   ├── project-structure.md        # This file
│   └── acmlab-commands.md          # CLI + MCP command reference
│
├── .env.example                    # Template for environment variables
├── .gitignore                      # .env, style.md, vendor/, bin/, docs/superpowers/
├── CLAUDE.md                       # Project conventions for Claude
├── go.mod
└── go.sum
```

## Design Principles

- **Three consumers, one codebase:** CLI, MCP server, and (future) ComputeRequest controller all use the same `internal/<uc>/` packages
- **Dynamic client only:** `k8s.io/client-go/dynamic` — no typed ACM/Hive imports (see ADR-001)
- **Resource construction in Go maps:** `builder.go` files build unstructured resources. `manifests/` YAMLs are reference documentation, not templates
- **Configuration from environment:** `.env` + `godotenv` (see ADR-004)
- **Build tags for test scope:** `go test ./...` = unit tests only. `integration` and `slow` tags for live hub tests (see ADR-003)
