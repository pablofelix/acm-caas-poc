# ACM CaaS PoC — Architecture

## Overview

This project evaluates ACM (Advanced Cluster Management) 2.17 as the provisioning
and management layer for a Cluster-as-a-Service (CaaS) platform. The PoC validates
use cases against a live hub cluster using reusable Go packages that are designed
to graduate into a ComputeRequest controller.

## System Context

```
                    ┌─────────────────────────────────────┐
                    │         ACM 2.17 Hub Cluster        │
                    │                                     │
                    │  ManagedCluster   ClusterDeployment  │
                    │  ManifestWork     Policy             │
                    │  ManagedClusterInfo  Placement       │
                    │  ClusterImageSet  ConfigMap (state)  │
                    └──────────────┬──────────────────────┘
                                   │ Kubernetes API
                                   │
                    ┌──────────────┴──────────────────────┐
                    │       internal/client/               │
                    │   Dynamic client wrapper (ADR-001)   │
                    │   k8s.io/client-go/dynamic           │
                    └──────────────┬──────────────────────┘
                                   │
                    ┌──────────────┴──────────────────────┐
                    │       internal/<uc>/                 │
                    │   Use case packages (ADR-005)        │
                    │   fleet, provisioning, policy,       │
                    │   tenant, lifecycle, monitoring,     │
                    │   importing, decommission, scaling,  │
                    │   registry, observability, upgrade   │
                    └──────────────┬──────────────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                     │
     ┌────────┴───────┐  ┌────────┴───────┐  ┌─────────┴────────┐
     │   cmd/acmlab/  │  │  internal/mcp/ │  │  Future:         │
     │   Cobra CLI    │  │  MCP Server    │  │  ComputeRequest  │
     │                │  │  (stdio)       │  │  Controller      │
     └────────────────┘  └────────────────┘  └──────────────────┘
```

Three consumers share the same UC packages — this is the core architectural
principle (ADR-005). The CLI validates use cases from the terminal, the MCP
server enables AI-assisted interactive testing, and the future controller
will reuse the same business logic behind a CRD reconciler.

## Layers

### Configuration (`internal/config/`)

All settings come from environment variables loaded via `godotenv` (ADR-004).
No hardcoded values, no flags for credentials. The `Config` struct is passed
to every UC package constructor.

### Client (`internal/client/`)

A thin wrapper over `k8s.io/client-go/dynamic` (ADR-001). Provides CRUD + Watch
operations on any Kubernetes resource via GVR (GroupVersionResource) constants
defined in `gvr.go`. No ACM-specific logic lives here — just generic dynamic
client operations.

Why dynamic over typed: eliminates heavy transitive dependencies
(`controller-runtime`, `operator-sdk`), avoids version pinning conflicts, and
works with any ACM version without recompiling. The tradeoff is no compile-time
validation of resource field names.

### Batch Utilities (`internal/batch/`)

Shared parallel-execution engine used by the CLI for multi-cluster operations.
Provides `Execute()` for concurrent operations across a cluster list,
`PrintSummary()` for human-readable output, and `ToJSON()` for machine output.
Cluster lists can come from CLI args or a file via `LoadFile()`.

### Use Case Packages (`internal/<uc>/`)

Each package exposes a struct with a `*client.Client` dependency and methods
for the operations in its use case. Resource construction lives in `builder.go`
files using Go maps — no YAML templates.

Pattern:
```
internal/<uc>/
  <uc>.go          # Manager struct + operations
  builder.go       # Builds unstructured resources from parameters
  <uc>_test.go     # Unit tests with dynamicfake
```

Packages with workflow state (decommission) add a `state.go` for ConfigMap-based
state management (ADR-008).

### Consumers

**CLI** (`cmd/acmlab/`) — one file per use case, Cobra commands. Each command
constructs the UC manager and calls its methods.

**MCP Server** (`internal/mcp/`) — registers tools that map 1:1 to UC package
methods. Uses `github.com/mark3labs/mcp-go` for the stdio transport.

## Use Case Catalogue

| UC | Name | Package | Status |
|----|------|---------|--------|
| 01 | Cluster provisioning (multi-platform) + preflight | `provisioning` | Done |
| 02 | Governance policy management | `policy` | Done |
| 03 | Tenant RBAC isolation | `tenant` | Done |
| 04 | Fleet status and cross-cluster search | `fleet` | Done |
| 05 | Hibernate/resume lifecycle | `lifecycle` | Done |
| 06 | Cluster resource monitoring | `monitoring` | Done |
| 06 | Thanos-based observability | `observability` | Done |
| 07 | External cluster import/detach | `importing` | Done |
| 08 | Legacy cluster decommissioning | `decommission` | Done |
| 09 | Cluster upgrades (Day-2 OCP) | `upgrade` | Done |
| 10 | Cluster scaling (workers) | `scaling` | Done |
| 11 | Cost tracking and chargeback | — | Planned |
| 13 | Registry mirror (ROKS, air-gapped) | `registry` | Done |
| 25 | ClusterPool pre-warmed clusters | `pool` | In Progress |
| 38 | HyperShift hosted control planes | `provisioning` | In Progress |
| 40 | CAPI cluster provisioning | `provisioning` | In Progress |
| 51 | VM lifecycle via KubeVirt | `virtualization` | Done |

## Data Flow

A typical operation flows through three stages:

```
User/AI ──▶ Consumer (CLI or MCP) ──▶ UC Package ──▶ Client ──▶ K8s API
                                           │
                                      builder.go
                                   (constructs unstructured
                                    resource as Go map)
```

Example — `acmlab decommission start spoke2`:

1. CLI parses args, loads config, creates `client.Client`
2. Constructs `decommission.Manager` with client and config
3. Calls `manager.Start("spoke2", owner, deadline)`
4. Manager creates a ConfigMap in the cluster namespace (state machine)
5. Manager runs `Audit()` which reads ManagedCluster labels and ManagedClusterInfo status
6. Returns the state to the CLI for display

## State Management (ADR-008)

Use cases with multi-step workflows (decommission) use ConfigMap-based state
machines. The ConfigMap lives in the cluster's namespace on the hub and stores:

- Current phase
- Phase history with timestamps
- Audit data (node count, CPU, memory, platform)
- Owner and deadline metadata

Labels (`caas-poc/workflow`, `caas-poc/cluster`) enable cross-namespace queries
to list all active workflows.

Phase transitions are sequential and validated — skipping phases is not allowed.
Each advance executes the phase action before updating the state.

## Idempotency (ADR-006)

All operations use create-if-not-exists or update-or-create patterns. Running
the same command twice produces the same result without errors. This is critical
for both the CLI (user retries) and the future controller (reconciliation loops).

## Observability

Structured logging via `log/slog` (Go stdlib). Every UC package constructor
accepts a `*slog.Logger`, and each public method logs at `Info` level with
the operation name and cluster identifier.

- **CLI**: text handler on stderr, `--verbose` flag switches to debug level
- **MCP server**: JSON handler on stderr (stdout is reserved for stdio transport)
- **Tests**: discard handler to keep output clean

No metrics or tracing at PoC stage — documented as an MVP gap.

## Testing Strategy

- **Unit tests**: `dynamicfake` for all UC packages. Coverage target: 90% for
  business logic, 70% for glue code (client, mcp)
- **Integration tests**: `//go:build integration` tag, run against live hub
- **Gherkin features**: `.feature` files in `features/`, executed via godog
  step definitions in `integration/`
- **Demo scripts**: `docs/demos/` — interactive phase-based scripts per UC
- **Manual reference**: `docs/manual/` — kubectl equivalents for every CLI command

## Design Decisions

| ADR | Decision | Rationale |
|-----|----------|-----------|
| 001 | Dynamic client over typed | Minimal deps, version-agnostic |
| 002 | MCP server for interactive testing | AI-assisted validation via Claude |
| 003 | Gherkin-driven with godog | Executable specs, readable by non-devs |
| 004 | Env-based configuration | No hardcoded creds, 12-factor |
| 005 | UC packages as controller foundation | Shared logic across CLI/MCP/controller |
| 006 | Idempotent operations | Safe retries, controller-ready |
| 007 | MinIO for observability object storage | Local S3-compatible for Thanos |
| 008 | ConfigMap state machine for workflows | Lightweight, no external deps |

## Directory Structure

See [project-structure.md](project-structure.md) for the full file tree.

## Future: ComputeRequest Controller

The UC packages are designed to slot into a Kubernetes controller that watches
a `ComputeRequest` CRD. The controller reconciler will call the same package
methods that the CLI and MCP server use today. The dynamic client can be
replaced with typed clients at that stage without changing the package interfaces.
