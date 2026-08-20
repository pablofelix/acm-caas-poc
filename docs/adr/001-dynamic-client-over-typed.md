# ADR-001: Use dynamic client instead of typed ACM/Hive imports

## Status

Accepted

## Context

ACM resources (ManagedCluster, ClusterDeployment, ManifestWork, Policy) have typed Go clients in their respective modules (`open-cluster-management.io/api`, `github.com/openshift/hive/apis`). Using typed clients gives compile-time safety and IDE autocompletion. However, these modules bring heavy transitive dependencies (controller-runtime, operator-sdk), version pinning conflicts with the k8s.io module set, and tight coupling to specific ACM versions.

## Decision

Use `k8s.io/client-go/dynamic` with `unstructured.Unstructured` for all ACM resource operations. GVR constants are defined in `internal/client/gvr.go`. Resource construction uses Go maps in `builder.go` files, not YAML templates.

## Consequences

- **Pro:** Minimal dependency tree — only `k8s.io/client-go` and `k8s.io/apimachinery`
- **Pro:** Works with any ACM version without recompiling
- **Pro:** Same client handles any CRD — easy to add Tekton, ArgoCD, or custom resources
- **Pro:** No version conflicts between ACM modules and k8s.io modules
- **Con:** No compile-time validation of resource structure — field names are strings
- **Con:** More verbose resource construction (maps vs struct literals)
- **Con:** No IDE autocompletion for ACM-specific fields

## Notes

When the code graduates to the ComputeRequest controller, typed clients can replace the dynamic ones. The package interfaces stay the same — only the implementation changes.
