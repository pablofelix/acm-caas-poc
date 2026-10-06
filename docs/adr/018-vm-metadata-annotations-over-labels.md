# ADR-018: VM metadata in annotations, not labels

## Status

Accepted

## Context

UC-51 creates ManifestWork resources on the hub to deliver KubeVirt VirtualMachine objects to spoke clusters. Each ManifestWork carries metadata about the VM: container image, disk size. Initially these were stored as Kubernetes labels on the ManifestWork.

Kubernetes label values must match `(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?` — no forward slashes or colons. Container image references (`registry.redhat.io/rhel9/rhel-guest-image:latest`) violate this constraint and cause the ManifestWork creation to be rejected by the API server.

## Decision

Store VM metadata that may contain arbitrary strings (image, disk size) as **annotations** on the ManifestWork, not labels. Use labels only for identifiers that conform to label syntax: `acmlab.redhat.com/vm=true` (selector for list operations) and `acmlab.redhat.com/vm-name=<name>` (human-readable lookup).

Specifically:
- **Labels:** `acmlab.redhat.com/vm`, `acmlab.redhat.com/vm-name`
- **Annotations:** `acmlab.redhat.com/vm-image`, `acmlab.redhat.com/vm-disk`

## Consequences

- **Pro:** ManifestWork creation no longer fails for images with registry URLs
- **Pro:** Labels remain usable as selectors for `kubectl get -l` and ACM search
- **Pro:** Annotations have no character restrictions — any metadata value works
- **Con:** Annotations are not indexable for server-side filtering — list operations must use label selectors and read annotations client-side
- **Applies to:** Any future metadata on ManifestWork or similar resources where the value may contain URLs, paths, or other arbitrary strings
