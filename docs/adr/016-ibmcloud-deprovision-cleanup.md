# ADR-016: Post-deprovision cleanup for IBM Cloud IPI

## Status

Accepted

## Context

During UC-01 testing (2026-09-23), IBM Cloud IPI provisioning via Hive ClusterDeployment exhibited two reliability issues:

1. **Bootstrap VM timeout** — the bootstrap VM failed to acquire a network address within the 25-minute window. All three master VMs provisioned successfully each time, but the bootstrap machine (which fetches ignition config from a COS bucket) never reported conditions. This caused all three install attempts to fail.

2. **Deprovision ordering bug** — the openshift-install destroy command for IBM Cloud does not delete load balancers before attempting to delete subnets. IBM Cloud enforces a dependency constraint (subnets cannot be deleted while a load balancer is attached), causing the destroyer to loop indefinitely on "Cannot delete the subnet while it contains a load balancer." Manual intervention via `ibmcloud` CLI was required to unblock: delete load balancers, then the RHCOS image instance, then the resource group.

AWS IPI provisioning on the same hub (spoke1) completed in 41 minutes with zero retries and clean deprovision. The issues are specific to the IBM Cloud CAPI provider and openshift-install IBM Cloud destroy logic.

## Decision

For any provisioning path that uses IBM Cloud IPI (Hive or direct), implement a post-deprovision verification step. The provisioner's `Destroy()` method should, after the ClusterDeployment is removed, verify that no orphaned resources remain in IBM Cloud by querying the VPC API using the cluster's infraID.

Resources to check (in dependency order):

1. Instances (VMs) — `ibmcloud is instances`
2. Load balancers — `ibmcloud is load-balancers`
3. Subnets — `ibmcloud is subnets`
4. VPCs — `ibmcloud is vpcs`
5. Floating IPs — `ibmcloud is floating-ips`
6. COS instances — `ibmcloud resource service-instances -g <infraID>`
7. Resource group — `ibmcloud resource groups`

If orphaned resources are found, the cleanup function should delete them in reverse dependency order (LBs before subnets, subnets before VPCs).

This is a workaround for upstream bugs in openshift-install and the IBM Cloud CAPI provider. It should be revisited when these components mature.

## Alternatives considered

- **Tag-based cleanup** — tag all IBM Cloud resources with the infraID during creation, then sweep by tag after destroy. More robust but requires modifying the provisioning path (the installer creates the resources, not our code).
- **Ignore and retry** — delete the ClusterDeployment, let Hive attempt deprovision, and if it gets stuck, just create a new ClusterDeployment with a new name. Leaves orphaned cloud resources that cost money.
- **Avoid IBM Cloud IPI entirely** — use only AWS for Hive provisioning and import existing IBM Cloud clusters via UC-02. Viable but limits the multi-cloud story.

## Consequences

- The provisioner gains a dependency on the IBM Cloud VPC API (REST or SDK) for post-destroy verification
- Destroy operations for IBM Cloud clusters take longer (API calls to verify cleanup)
- Orphaned resources are caught and cleaned up instead of accumulating cost
- This is MVP scope, not PoC — for the PoC, manual cleanup via `ibmcloud` CLI is sufficient
