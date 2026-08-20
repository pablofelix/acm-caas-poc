# ADR-005: UC packages as reusable foundation for the ComputeRequest controller

## Status

Accepted

## Context

The lab validates ACM capabilities, but the code should not be throwaway. The ComputeRequest controller will need the same operations: provisioning, policy, workload deployment, fleet queries, lifecycle management, monitoring, and cluster import.

## Decision

Each use case is an independent Go package under `internal/` with a clean interface:

| Package | Struct | Key Methods |
|---------|--------|-------------|
| `provisioning` | `Provisioner` | CreateCluster, DeleteCluster, GetStatus |
| `policy` | `Manager` | CreatePolicy, DeletePolicy, GetCompliance |
| `workload` | `Deployer` | Deploy, Undeploy, GetStatus |
| `fleet` | `Inspector` | ListClusters, GetCluster |
| `lifecycle` | `Manager` | Hibernate, Resume, GetPowerState |
| `monitoring` | `Monitor` | GetClusterResources, ListClusterResources |
| `importing` | `Importer` | ImportCluster, DetachCluster, GetImportStatus |

Every package takes a `*client.Client` and `config.Config` in its constructor. Three consumers use the same packages: CLI, MCP server, and (future) controller.

## Consequences

- **Pro:** Lab code graduates directly into the controller — no rewrite
- **Pro:** Interfaces are defined now, so the controller's reconcile loop design is clear
- **Pro:** Unit tests from the lab transfer to the controller without changes
- **Pro:** Swapping implementations (e.g., ACM → Cluster API) only requires new packages behind the same interfaces
- **Con:** Lab code needs cleaner interfaces than typical throwaway PoC code
- **Con:** Changes to a package interface require updating CLI, MCP, and tests
