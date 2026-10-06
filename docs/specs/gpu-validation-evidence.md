# GPU Validation Evidence

Status: in progress. Last updated: 2026-09-21.

## Scope

This document records what has been verified without GPU hardware and what requires physical hardware for acceptance. Simulated tests validate logic, contracts, and error handling; they do not constitute GPUaaS acceptance.

## Verified Without GPU Hardware

### GPU Stack Deployment (internal/gpu/stack.go)

- PreflightStack validates cluster exists, has gpu-type label, and ManagedClusterConditionAvailable=True
- DeployStack creates Kueue ManifestWork, Kyverno ManifestWork, health Policy with Placement and PlacementBinding
- RemoveStack deletes all stack resources including queue ManifestWork
- DetectDrift reads Policy compliance status and identifies degraded components
- CreateClusterQueues generates ResourceFlavor and ClusterQueue manifests per GPU type
- Tests use fake dynamic client with synthetic ManagedCluster objects

### GPU Routing (internal/gpu/routing.go)

- BestCluster uses unique random suffix per call to prevent concurrent interference
- Context-bounded timeout wraps entire operation; cleanup uses separate background context
- Error normalisation: API errors during expired operation context map to ErrGPUPending; caller cancellation propagated directly
- queryPlacementDecision consolidates existence check and result parsing in single List call (TOCTOU fix)
- Tests: unique naming, concurrent calls, timeout returns ErrGPUPending, empty decision returns ErrGPUNoMatch

### GPU Inventory and Fitness (internal/gpu/inventory.go)

- ListGPUClusters enumerates ManagedClusters with gpu-type label and evaluates fitness
- GetGPUFitness evaluates single cluster with typed reasons: NoGPUType, NotAvailable, Saturated
- evaluateCluster checks three independent signals: gpu-type label, ManagedCluster Available condition, gpu-available label
- Empty inventory returns empty slice (not error) — absence is not an API failure
- Tests: empty list, GPU-only filtering, unavailable/saturated markers, multiple reasons, cluster not found

### Request Lifecycle Contract (internal/gpu/request.go)

- SubmitRequest validates input, detects duplicates, routes via BestCluster, stores state
- Pending requests carry a reason ("placement pending" or "no matching cluster"); RetryRequest re-attempts routing
- Cancellation during routing is detected: routing result does not overwrite a Cancelled request
- StartRequest transitions Admitted to Running; CompleteRequest requires Running or Admitted state
- GetRequestStatus returns a copy to prevent external mutation
- State machine: Pending -> Admitted -> Running -> Completed; Pending/Admitted/Running -> Cancelled
- Tests: validation, duplicate detection, admitted path with simulated PlacementDecision, pending path, cancel/complete transitions, retry, start, race protection, copy semantics
- **Limitation**: cluster selection via BestCluster does not demonstrate quota admission or GPU workload execution. Admitted state means a cluster was selected, not that Kueue has reserved capacity or that a GPU workload can run

### Integration Steps (integration/)

- lifecycle: state preparation is idempotent, always waits for status convergence
- lifecycle: eventuallyAvailable polls ManagedCluster Available condition (not Hive powerState)
- lifecycle: timeout scenario checks errors.Is(context.DeadlineExceeded)
- provisioning: credential validation checks specific provider keys with non-empty values
- provisioning: clusterReachesProvisioned polls with 30s interval, 20m timeout
- provisioning: clusterDeploymentRemoved uses errors.IsNotFound

## Requires GPU Hardware

### Device Discovery and Node Readiness

- Verify GPU devices appear as allocatable resources on nodes with appropriate driver and device plugin
- Confirm ManagedCluster/ManagedClusterInfo reflects GPU capacity correctly
- Validate that node NotReady or device plugin failure is reflected in inventory

### Stack Operational Validation

- Verify Kueue and Kyverno ManifestWork applies correctly on spoke cluster
- Confirm health Policy detects actual component failures (not just fake compliance)
- Validate that CRDs, RBAC, and admission webhooks are operational

### Workload Admission and Execution

- Submit a GPU workload via Kueue and observe it reaches Running state on GPU node
- Verify quota enforcement: excess requests are queued or rejected
- Confirm workload completion releases quota and is observable

### Multi-tenant Isolation

- Two tenants submit requests; verify correct routing and no cross-tenant access
- Validate ClusterSet-based segregation with actual GPU clusters

### Failure and Recovery

- Remove or fail a GPU node; verify inventory reflects the change
- Confirm health alerts are generated and inventory excludes the failed node
- Restore the node; verify inventory re-includes it after observing required conditions

### Maintenance with Active Workloads

- Initiate maintenance (hibernate/cordon) while GPU workloads are running
- Verify coordinated behaviour: drain, workload migration or completion, then maintenance

## Test Coverage Summary

| Package | Coverage | Notes |
|---------|----------|-------|
| internal/gpu | 87.7% | Includes all new inventory and request code |
| inventory.go | 93-100% per function | All paths tested |
| request.go | 94-100% per function | Admitted path uses simulated PlacementDecision |
