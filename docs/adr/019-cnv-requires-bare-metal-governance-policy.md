# ADR-019: CNV installation via governance policy — bare metal required

## Status

Accepted

## Context

UC-51 deploys VMs to spoke clusters via ManifestWork wrapping KubeVirt VirtualMachine CRDs. The spoke must have OpenShift Virtualization (CNV) installed for the VirtualMachine CRD to exist.

Initial attempt used ManifestWork to deliver the CNV operator (Namespace + OperatorGroup + Subscription). This failed because the klusterlet work service account lacks RBAC to create `operatorgroups.operators.coreos.com` on the spoke — by design, ManifestWork agents have limited permissions for OLM resources.

Second approach used ACM's OperatorPolicy (v1beta1). The auto-generated AllNamespaces OperatorGroup conflicted with CNV's install mode requirements, and the OperatorPolicy couldn't update an existing OperatorGroup to OwnNamespace mode.

Third approach used ACM's governance Policy with ConfigurationPolicy templates (`remediationAction: enforce`). This successfully created all resources (Namespace, OperatorGroup, Subscription, HyperConverged CR) on the spoke. However, OLM on the ROKS (IBM Cloud VPC) spoke cluster could not resolve the `kubevirt-hyperconverged` package — no InstallPlan was created despite the CatalogSource being READY.

Root cause: OpenShift Virtualization requires bare metal nodes with hardware virtualization extensions (Intel VT-x / AMD-V). IBM Cloud VPC workers are virtual machines without nested virtualization support. The `redhat-operators` catalog on ROKS filters out the CNV package for unsupported platforms.

## Decision

1. **Use Policy + ConfigurationPolicy** (not ManifestWork or OperatorPolicy) for installing operators on spoke clusters via ACM. This is the recommended approach per ACM documentation and works around klusterlet RBAC limitations.

2. **CNV requires a bare metal spoke cluster.** The VM lifecycle features (UC-51) cannot be demonstrated on cloud-based virtual infrastructure (ROKS, ROSA, ARO) unless bare metal workers are available.

3. **Vanilla Kubernetes clusters (CAPI)** do not have OLM, so the ConfigurationPolicy approach with Subscription/OperatorGroup does not apply. KubeVirt upstream could be installed via ManifestWork delivering the operator YAML directly, but still requires hardware virtualization support.

## Alternatives considered

| Option | Pros | Cons |
|--------|------|------|
| AWS `.metal` instances (i3.metal, m5.metal) | Hardware virt, works with OCP | Expensive ($4+/hr), only OCP spokes |
| KubeVirt upstream on CAPI + ManifestWork | Works on vanilla K8s | No OLM, needs bare metal or sw emulation |
| Software emulation (QEMU TCG) | No bare metal needed | Very slow, not representative |
| On-prem bare metal OCP spoke | Full support | Requires physical infrastructure |

## Consequences

- **Pro:** Policy + ConfigurationPolicy approach is production-ready and follows ACM best practices for operator governance
- **Pro:** Code is correct and tested — `EnsureCNVOperator` creates the right resources, idempotent, with cleanup via `RemoveCNVOperator`
- **Con:** UC-51 VM demos require a bare metal spoke cluster not currently in the lab
- **Con:** CAPI vanilla K8s clusters need a different installation path (ManifestWork with operator YAML instead of OLM Subscription)
- **Applies to:** Any operator that requires specific hardware capabilities (GPU operators, SR-IOV, DPDK)
