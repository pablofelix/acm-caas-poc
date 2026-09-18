# ACM Use Cases — ACM CaaS PoC

>
> Use cases to validate ACM capabilities for the CaaS platform.
> Each use case exercises ACM's Go API (`open-cluster-management.io/api`,
> `github.com/openshift/hive/apis`) and maps directly to what the
> ComputeRequest controller will do in the final solution.
>
> Implementation approach: each use case becomes a Go package that
> uses `k8s.io/client-go` + ACM typed clients to create, watch, and
> assert on ACM resources. The same code graduates into the
> ComputeRequest controller's reconcile logic.

## UC-01: Provision a spoke cluster on-demand

**Feature**: Cluster provisioning via ACM Go API

As a platform operator  
I want to provision spoke clusters programmatically via the ACM API  
So that the ComputeRequest controller can automate this

### Scenario: Create a ClusterDeployment and wait for provisioning

**Given** I have a typed client for `hive.openshift.io/v1`  
**And** cloud credentials exist as a Secret in the spoke namespace  
**And** a ClusterImageSet for the target OCP version exists  
**When** I create a ClusterDeployment resource via the Go API  
**Then** the ClusterDeployment is accepted by Hive  
**And** I can watch the `status.conditions` until `Provisioned = True`  
**And** a ManagedCluster resource is automatically created on the hub

### Scenario: Delete a ClusterDeployment and verify cleanup

**Given** a ClusterDeployment exists with status `Provisioned = True`  
**When** I delete the ClusterDeployment via the Go API  
**Then** Hive deprovisions the cloud infrastructure  
**And** the ManagedCluster is removed from the hub

### Scenario: Destroy multiple clusters in parallel

**Given** I have three provisioned clusters  
**When** I run `acmlab provision destroy spoke1 spoke2 spoke3`  
**Then** all three clusters are destroyed in parallel  
**And** a summary table shows per-cluster results

### ACM Go types

```go
github.com/openshift/hive/apis/hive/v1.ClusterDeployment
github.com/openshift/hive/apis/hive/v1.ClusterImageSet
open-cluster-management.io/api/cluster/v1.ManagedCluster
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.platform + spec.capacity
  -> controller builds ClusterDeployment
  -> controller watches until Provisioned = True
  -> controller updates ComputeRequest.status.phase = Ready
```

---

## UC-02: Enforce image registry restrictions on managed clusters

**Feature**: Image registry policy enforcement via ACM Go API

As a platform operator  
I want to restrict container images to approved registries  
So that the ComputeRequest controller can enforce supply-chain security

### Scenario: Create an image registry restriction policy targeting dev clusters

**Given** I have a typed client for `policy.open-cluster-management.io/v1`  
**And** a spoke cluster exists with label `env=dev`  
**When** I create a Policy that only allows images from approved registries  
**And** I bind it to clusters with label `env=dev` via PlacementRule  
**Then** the policy is distributed to matching clusters  
**And** the Policy status shows compliance state per cluster

### Scenario: Detect non-compliant image usage

**Given** an image registry policy is distributed to a spoke  
**When** a pod running an image from an unapproved registry exists  
**Then** `Policy.status.compliant = NonCompliant`  
**And** the per-cluster status identifies the violating namespace and pod

### Scenario: Enforce approved registries only

**Given** an image registry policy with `remediationAction = enforce`  
**When** a user tries to deploy a pod with image from docker.io  
**Then** the admission controller blocks the pod  
**And** the Policy status remains Compliant

### Approved registries (example)

- `registry.redhat.io`
- `quay.io/your-org`
- `us.icr.io/your-namespace` (IBM Cloud Container Registry)

### ACM Go types

```go
open-cluster-management.io/governance-policy-propagator/api/v1.Policy
open-cluster-management.io/api/cluster/v1beta1.Placement
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.security.allowedRegistries = ["registry.redhat.io", "quay.io/myorg"]
  -> controller creates Policy with ConfigurationPolicy enforcing AllowedContainerImagesPolicy
  -> controller creates PlacementBinding targeting the spoke
  -> controller watches Policy.status.compliant
  -> controller updates ComputeRequest.status.conditions[PolicyCompliant]
```

---

## UC-03: Deploy tenant isolation to spokes from the hub

**Feature**: Tenant RBAC isolation deployment via ManifestWork

As a platform operator  
I want to prepare spoke clusters with tenant namespaces, RBAC, and network policies  
So that the ComputeRequest controller can isolate teams on shared clusters

### Scenario: Create a ManifestWork with tenant isolation resources

**Given** I have a typed client for `work.open-cluster-management.io/v1`  
**And** a spoke cluster is registered and accepted  
**When** I create a ManifestWork in the spoke's namespace containing:

| Resource       | Name                 | Purpose                              |
|----------------|----------------------|--------------------------------------|
| Namespace      | team-alpha           | Isolated tenant namespace             |
| RoleBinding    | team-alpha-admin     | Grants edit role to team-alpha group  |
| NetworkPolicy  | deny-cross-namespace | Blocks traffic from other namespaces  |
| ResourceQuota  | team-alpha-quota     | Limits CPU/memory per tenant          |

**Then** the ManifestWork is synced to the spoke  
**And** `status.conditions` shows `Applied = True`  
**And** `status.resourceStatus` lists each manifest's apply result

### Scenario: Update tenant resource limits via ManifestWork

**Given** a tenant isolation ManifestWork exists with ResourceQuota `cpu=4, memory=8Gi`  
**When** I update the ManifestWork to set ResourceQuota `cpu=8, memory=16Gi`  
**Then** the spoke ResourceQuota is updated to the new limits  
**And** the ManifestWork status reflects the updated state

### Scenario: Onboard a new team to an existing spoke

**Given** a spoke cluster already has tenant "team-alpha" deployed  
**When** I create a second ManifestWork with isolation for "team-beta"  
**Then** both tenants coexist on the spoke with independent RBAC and quotas  
**And** NetworkPolicies prevent cross-tenant traffic

### Manifests deployed per tenant

- `Namespace` — isolated workspace for the team
- `RoleBinding` — binds `edit` ClusterRole to the team's group (e.g., LDAP/OIDC group)
- `NetworkPolicy` — default-deny ingress from other namespaces, allow only within tenant
- `ResourceQuota` — CPU, memory, and pod limits per tenant

### ACM Go types

```go
open-cluster-management.io/api/work/v1.ManifestWork
open-cluster-management.io/api/work/v1.ManifestWorkReplicaSet
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.tenants[0].name = "team-alpha"
ComputeRequest.spec.tenants[0].group = "cn=team-alpha,ou=groups,dc=company"
ComputeRequest.spec.tenants[0].quota = {cpu: "8", memory: "16Gi"}
  -> controller builds ManifestWork with Namespace + RoleBinding + NetworkPolicy + ResourceQuota
  -> controller creates ManifestWork in spoke namespace
  -> controller watches Applied condition
  -> controller updates ComputeRequest.status.tenants[0].ready = true

ComputeRequest.spec.features = ["monitoring", "pipelines"]
  -> controller looks up ManifestWork templates for each feature
  -> controller creates additional ManifestWork in spoke namespace
  -> controller updates ComputeRequest.status with feature readiness
```

---

## UC-04: Query fleet status and search resources across clusters

**Feature**: ManagedCluster status and cross-cluster search via ACM Go API

As a platform operator  
I want to query cluster health and search resources across the fleet  
So that the ComputeRequest controller can reflect spoke state and find resources

### Scenario: List managed clusters and read their conditions

**Given** I have a typed client for `cluster.open-cluster-management.io/v1`  
**When** I list ManagedCluster resources on the hub  
**Then** each cluster has conditions: `ManagedClusterConditionAvailable`, `HubAcceptedManagedCluster`, `ManagedClusterJoined`  
**And** I can read labels (cloud, region, gpu-count) from each cluster

### Scenario: Watch for cluster health changes

**Given** a spoke cluster is `Available = True`  
**When** I set up a watch on ManagedCluster resources  
**And** the spoke loses connectivity  
**Then** the watch receives an event with `Available = False`

### Scenario: Search resources across all managed clusters

**Given** ACM Search is enabled on the hub (search-collector addon active)  
**When** I query the Search API for pods with label `app=my-workload`  
**Then** I receive results from all spoke clusters where matching pods exist  
**And** each result includes cluster name, namespace, pod name, and status

### Scenario: Search for resources in CrashLoopBackOff across the fleet

**Given** workloads are deployed across multiple spoke clusters  
**When** I query Search for pods with `status.phase != Running`  
**Then** I can identify failing pods across the entire fleet  
**And** correlate them with the cluster and tenant they belong to

### ACM Go types

```go
open-cluster-management.io/api/cluster/v1.ManagedCluster
open-cluster-management.io/api/cluster/v1.ManagedClusterStatus
```

### ACM Search

- Search API endpoint: `https://<hub>/searchapi/graphql`
- Indexed by the `search-collector` addon on each spoke
- Supports queries by kind, namespace, label, cluster, status
- Provides cross-cluster resource visibility without direct spoke access

### ComputeRequest controller equivalent

```
ComputeRequest.status.binding.clusterName = "spoke-1"
  -> controller watches ManagedCluster "spoke-1"
  -> controller mirrors conditions into ComputeRequest.status.conditions
  -> controller sets ComputeRequest.status.phase = Degraded if Available = False

ComputeRequest.status.workloads
  -> controller uses Search API to find resources matching tenant labels
  -> controller aggregates workload status across clusters
  -> controller updates ComputeRequest.status.workloads[].healthy
```

---

## UC-05: Manage cluster lifecycle (hibernate/resume)

**Feature**: Cluster power management via Hive Go API

As a platform operator  
I want to hibernate and resume clusters programmatically  
So that the ComputeRequest controller can implement idle reclamation

### Scenario: Hibernate a Hive-provisioned cluster by patching powerState

**Given** a ClusterDeployment exists with `powerState = Running`  
**When** I patch `spec.powerState` to `Hibernating` via the Go API  
**Then** Hive stops the compute instances  
**And** `ClusterDeployment.status.powerState = Hibernating`  
**And** the ManagedCluster condition `Available` transitions to `Unknown`

### Scenario: Resume a hibernated cluster

**Given** a ClusterDeployment has `powerState = Hibernating`  
**When** I patch `spec.powerState` to `Running`  
**Then** Hive starts the compute instances  
**And** the spoke reconnects to the hub  
**And** `ManagedCluster Available = True`

### Scenario: Verify lifecycle limitations on imported clusters

**Given** a ManagedCluster "imported-cluster" was imported via UC-07 (no Hive ClusterDeployment)  
**When** I attempt to hibernate the imported cluster  
**Then** the operation fails because no ClusterDeployment exists  
**And** the error is reported clearly to the caller

### Scenario: Hibernate multiple clusters in parallel

**Given** I have three running Hive-provisioned clusters  
**When** I run `acmlab lifecycle hibernate spoke1 spoke2 spoke3 --wait`  
**Then** all three clusters are hibernated in parallel  
**And** a summary table shows per-cluster results

### Important

Hibernate/resume relies on Hive's `ClusterDeployment.spec.powerState`, which only exists for ACM-provisioned clusters. Imported/registered clusters (UC-07) do not have a Hive-managed ClusterDeployment, so lifecycle operations are not available.

The ComputeRequest controller must distinguish between provisioned and imported clusters and offer lifecycle management only where supported.

### ACM Go types

```go
github.com/openshift/hive/apis/hive/v1.ClusterDeployment (spec.powerState)
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.utilization.policy = reclaimable
ComputeRequest.spec.utilization.idleTimeout = 2h
  -> controller checks if cluster was provisioned by Hive (ClusterDeployment exists)
  -> if yes: patches ClusterDeployment.spec.powerState = Hibernating
  -> if no (imported): skips lifecycle, sets condition LifecycleNotSupported
  -> controller updates ComputeRequest.status.phase accordingly

User accesses cluster again:
  -> controller patches powerState = Running (Hive-provisioned only)
  -> controller waits for Available = True
  -> controller updates ComputeRequest.status.phase = Ready
```

---

## UC-06: Monitor cluster resources from the hub

**Feature**: Cluster resource monitoring via ACM Go API

As a platform operator  
I want to query cluster resource usage and node status programmatically  
So that the ComputeRequest controller can make capacity-aware scheduling decisions

### Scenario: Read node count and resource capacity for a managed cluster

**Given** I have a typed client for `internal.open-cluster-management.io/v1beta1`  
**And** a ManagedCluster "spoke-1" is joined and available  
**When** I get the ManagedClusterInfo for "spoke-1"  
**Then** I can read the node list with roles (master, worker)  
**And** I can read CPU and memory capacity per node  
**And** I can read CPU and memory allocatable per node

### Scenario: Compare resource usage across the fleet

**Given** multiple spoke clusters are joined and available  
**When** I list ManagedClusterInfo resources on the hub  
**Then** I can aggregate total CPU capacity across the fleet  
**And** I can identify clusters with available capacity  
**And** I can rank clusters by utilization percentage

### Scenario: Detect resource pressure on a spoke

**Given** a spoke cluster is running workloads near capacity  
**When** I read the ManagedClusterInfo resource conditions  
**Then** I can detect nodes with MemoryPressure or DiskPressure  
**And** I can identify clusters approaching resource limits

### Scenario: Enable Thanos-based observability on the hub

**Given** ACM Observability operator is installed on the hub  
**When** I deploy MinIO as object storage in the observability namespace  
**And** I create a Thanos object storage secret with S3 credentials  
**And** I create a MultiClusterObservability CR with minimal instance size  
**Then** the observability stack (Thanos Querier, metrics collector) is deployed  
**And** spoke clusters begin sending metrics to the hub  
**And** the MCO status shows `Ready = True`

### Scenario: Query real-time CPU and memory usage via Thanos

**Given** MultiClusterObservability is Ready on the hub  
**And** spoke clusters are sending metrics  
**When** I query the Thanos API for `cluster:capacity_cpu_cores:sum`  
**Then** I receive real-time CPU usage (not just capacity) per cluster  
**And** I can compare usage vs capacity to calculate utilization percentage

### Scenario: Clean teardown of observability stack

**Given** MultiClusterObservability and MinIO are deployed  
**When** I run the observability teardown  
**Then** the MCO CR is deleted  
**And** MinIO (Deployment, Service, PVC, Secret) is removed  
**And** the observability namespace is deleted  
**And** no leftover resources remain on the hub

### Two data sources for monitoring

| Source  |  Data  |  Setup required |
|---------|--------|-----------------|
| ManagedClusterInfo  |  Node count, CPU/memory capacity, instance type, region, zone, OCP version  |  None — works out of the box |
| Thanos (Observability)  |  Real-time CPU/memory usage, API server latency, etcd health, pod metrics  |  MinIO + MultiClusterObservability CR |

### ACM Go types

```go
internal.open-cluster-management.io/v1beta1.ManagedClusterInfo
cluster.open-cluster-management.io/v1.ManagedCluster (conditions)
observability.open-cluster-management.io/v1beta2.MultiClusterObservability
```

### Key fields in ManagedClusterInfo

- `status.nodeList[].name` — node names
- `status.nodeList[].labels` — node roles and topology
- `status.nodeList[].capacity` — CPU, memory, pods
- `status.nodeList[].conditions` — Ready, MemoryPressure, DiskPressure
- `status.distributionInfo` — OCP version, channel, upgrade status

### Observability setup (idempotent)

- MinIO deployment with PVC for Thanos object storage
- Thanos secret with S3 endpoint pointing to MinIO
- MultiClusterObservability CR with `instanceSize: minimal`
- Teardown removes all resources — no leftovers

### ComputeRequest controller equivalent

```
ComputeRequest.spec.capacity.cpu = "16"
ComputeRequest.spec.capacity.memory = "64Gi"
  -> controller queries ManagedClusterInfo across fleet (capacity)
  -> controller queries Thanos for real-time usage (if observability enabled)
  -> controller finds spoke with sufficient allocatable resources
  -> controller updates ComputeRequest.status.binding.clusterName = best-fit
  -> controller monitors spoke resource pressure
  -> controller sets ComputeRequest.status.conditions[CapacitySufficient]
```

---

## UC-07: Import an existing cluster into ACM

**Feature**: External cluster import via ACM Go API

As a platform operator  
I want to import clusters not provisioned by ACM  
So that the ComputeRequest controller can manage heterogeneous fleets

### Scenario: Import a cluster using auto-import secret

**Given** I have admin access to an external cluster  
**And** I have the external cluster's kubeconfig  
**When** I create a ManagedCluster resource on the hub with appropriate labels  
**And** I create a KlusterletAddonConfig in the cluster's namespace  
**And** I create an auto-import Secret with the external kubeconfig  
**Then** the klusterlet agent is deployed on the external cluster  
**And** `ManagedCluster.status.conditions` shows `ManagedClusterJoined = True`  
**And** `ManagedCluster.status.conditions` shows `ManagedClusterConditionAvailable = True`

### Scenario: Import a cluster with specific addon configuration

**Given** an external cluster needs monitoring and policy addons  
**When** I create a KlusterletAddonConfig with enabled addons  
**Then** the specified addons are deployed on the imported cluster  
**And** addon status is reported back to the hub

### Scenario: Detach an imported cluster

**Given** an imported ManagedCluster "external-1" exists  
**When** I delete the ManagedCluster resource from the hub  
**Then** the klusterlet agent is removed from the external cluster  
**And** the cluster operates independently without ACM management

### Scenario: Import multiple clusters in parallel from a file

**Given** I have a clusters.yaml with three cluster entries  
**When** I run `acmlab import cluster --from-file clusters.yaml --wait`  
**Then** all three clusters are imported in parallel  
**And** a summary table shows NAME / STATUS / MESSAGE per cluster  
**And** the exit code is non-zero if any cluster failed

### ACM Go types

```go
cluster.open-cluster-management.io/v1.ManagedCluster
agent.open-cluster-management.io/v1.KlusterletAddonConfig
v1.Secret (auto-import secret with kubeconfig)
```

### Auto-import secret format

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: auto-import-secret
  namespace: <cluster-name>
type: Opaque
stringData:
  kubeconfig: |
    <external-cluster-kubeconfig>
```

### KlusterletAddonConfig addons

- `applicationManager` — application lifecycle
- `policyController` — policy enforcement on spoke
- `searchCollector` — search indexing
- `certPolicyController` — certificate policy
- `iamPolicyController` — IAM policy

### ComputeRequest controller equivalent

```
ComputeRequest.spec.import.kubeconfig = <secret-ref>
ComputeRequest.spec.import.labels = {cloud: "on-prem", region: "dc-1"}
  -> controller creates ManagedCluster with labels
  -> controller creates KlusterletAddonConfig with required addons
  -> controller creates auto-import Secret with kubeconfig
  -> controller watches ManagedClusterJoined + Available conditions
  -> controller updates ComputeRequest.status.phase = Ready
```

---

## UC-08: Legacy cluster decommissioning lifecycle

**Feature**: Legacy cluster decommissioning via ACM Go API

As a platform operator  
I want to audit, notify, and safely decommission inherited clusters  
So that the CaaS platform can reclaim resources and reduce cost

### Scenario: Import a legacy cluster and audit its usage

**Given** an existing cluster not yet managed by ACM  
**And** I have its kubeconfig  
**When** I import the cluster into ACM via auto-import secret  
**And** I query its ManagedClusterInfo for node count, CPU, and memory  
**And** I list all non-system namespaces and their workload counts  
**Then** I get an audit report with: owner (from labels/annotations), last workload activity, resource utilization  
**And** clusters with utilization below threshold are flagged as decommission candidates

### Scenario: Notify cluster owner before decommissioning

**Given** a legacy cluster is flagged as a decommission candidate  
**When** I send a notification to the cluster owner with a reclaim deadline  
**Then** the notification includes: cluster name, current usage summary, deadline date, action required  
**And** the notification is tracked in the audit log

### Scenario: Backup cluster state before deletion

**Given** a legacy cluster is approved for decommissioning  
**When** I export all non-system namespace resources as YAML  
**And** I list all PersistentVolumes with storage class, size, and bound claims  
**And** I export cluster-scoped resources (ClusterRoles, CRDs, OAuth config)  
**Then** the backup is stored in a designated location  
**And** the backup manifest lists all exported resources with their sizes

### Scenario: Decommission the cluster

**Given** a legacy cluster has been backed up and the owner notified  
**And** the reclaim deadline has passed with no response  
**When** I drain all worker nodes  
**And** I delete the cluster via cloud provider API or Hive ClusterDeployment  
**And** I remove the ManagedCluster, ManifestWorks, and policies from ACM  
**Then** the cluster no longer appears in the fleet  
**And** the decommission event is recorded in the audit log

### Scenario: Decommission is idempotent and handles partial state

**Given** a decommission was interrupted midway (e.g., cluster deleted but ACM not cleaned)  
**When** I run decommission again  
**Then** it completes the remaining cleanup steps without errors  
**And** no resources are left behind in ACM

### Decommission workflow steps

1. **Import** into ACM (reuses UC-07)
2. **Audit** — workloads, utilization, owner identification
3. **Notify** — owner notification with deadline
4. **Backup** — namespace resources, PVs, cluster-scoped config
5. **Drain** — cordon + drain nodes
6. **Delete** — destroy cluster via cloud API
7. **Cleanup** — remove from ACM (ManagedCluster, ManifestWorks, policies)

### ACM resources (dynamic client)

```
cluster.open-cluster-management.io/v1         ManagedCluster        — detach/delete
internal.open-cluster-management.io/v1beta1   ManagedClusterInfo    — usage audit
work.open-cluster-management.io/v1            ManifestWork          — cleanup
v1                                            ConfigMap             — state machine (see ADR-008)
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.lifecycle.decommission = true
ComputeRequest.spec.lifecycle.decommissionDeadline = "2026-09-15"
  -> controller audits cluster usage and identifies owner
  -> controller sends notification (webhook/email)
  -> controller waits for deadline
  -> controller exports backup manifest
  -> controller drains and deletes cluster
  -> controller cleans up ACM resources
  -> controller updates ComputeRequest.status.phase = Decommissioned
```

---

## UC-09: Cluster upgrades (Day-2 operations)

**Feature**: Managed cluster OCP upgrades via ACM Go API

As a platform operator  
I want to orchestrate OCP version upgrades across the fleet from the hub  
So that the CaaS platform keeps clusters on supported versions

### Scenario: Check available upgrades for a managed cluster

**Given** a managed cluster is running OCP 4.21.x on channel stable-4.21  
**When** I query the cluster's ManagedClusterInfo for distributionInfo  
**And** I check the available upgrade versions from the channel  
**Then** I get the current version, channel, and list of available target versions

### Scenario: Upgrade a single cluster via ClusterCurator

**Given** a managed cluster is running OCP 4.21.28  
**And** OCP 4.21.29 is available in the stable-4.21 channel  
**When** I create a ClusterCurator CR with `desiredUpdate = 4.21.29`  
**Then** the ClusterCurator orchestrates the upgrade on the spoke  
**And** the curator status shows progress (pre-hook, upgrade, post-hook)  
**And** ManagedClusterInfo.distributionInfo reflects the new version when complete

### Scenario: Batch upgrade clusters by label

**Given** multiple managed clusters have label `upgrade-group=batch-1`  
**When** I create ClusterCurator CRs for all clusters in batch-1  
**Then** upgrades proceed in parallel across the batch  
**And** I can monitor per-cluster progress from the hub  
**And** clusters that fail upgrade are flagged without blocking others

### Scenario: Upgrade with pre and post hooks

**Given** I need to run health checks before and after upgrade  
**When** I create a ClusterCurator with prehook and posthook Ansible jobs  
**Then** the prehook runs before the upgrade starts  
**And** the posthook runs after the upgrade completes  
**And** if the prehook fails, the upgrade is aborted

### ACM Go types

```go
cluster.open-cluster-management.io/v1.ManagedCluster — cluster selection
internal.open-cluster-management.io/v1beta1.ManagedClusterInfo — version info
cluster.open-cluster-management.io/v1beta1.ClusterCurator — upgrade orchestration
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.version.desired = "4.21.29"
ComputeRequest.spec.version.channel = "stable-4.21"
  -> controller checks current version via ManagedClusterInfo
  -> controller creates ClusterCurator with desiredUpdate
  -> controller monitors curator status conditions
  -> controller updates ComputeRequest.status.version.current
```

---

## UC-10: Cluster scaling (add/remove workers)

**Feature**: Managed cluster worker node scaling via ACM Go API

As a platform operator  
I want to scale worker nodes in managed clusters from the hub  
So that the CaaS platform can adjust capacity to tenant demand

### Scenario: Query current node pool configuration

**Given** a managed cluster was provisioned via Hive with a MachinePool  
**When** I query the MachinePool for the cluster  
**Then** I get the current replica count, instance type, and autoscaling config

### Scenario: Scale up workers by increasing MachinePool replicas

**Given** a managed cluster has a MachinePool with 3 replicas  
**When** I patch the MachinePool to set `replicas = 5`  
**Then** 2 new worker nodes are provisioned  
**And** ManagedClusterInfo.nodeList shows 5 worker nodes when scaling completes

### Scenario: Scale down workers

**Given** a managed cluster has a MachinePool with 5 replicas  
**And** cluster utilization is below 30%  
**When** I patch the MachinePool to set `replicas = 3`  
**Then** 2 worker nodes are drained and removed  
**And** workloads are redistributed to remaining nodes

### Scenario: Enable autoscaling on a MachinePool

**Given** a managed cluster has a MachinePool with fixed replicas  
**When** I patch the MachinePool to enable autoscaling with `min=3, max=10`  
**Then** the MachinePool switches from fixed replicas to autoscaling  
**And** the cluster scales automatically based on pod scheduling pressure

### Scenario: Scale a cluster without Hive (imported cluster)

**Given** a managed cluster was imported (not provisioned by Hive)  
**When** I try to scale its workers from the hub  
**Then** the operation reports that scaling is not available for imported clusters  
**And** suggests using the cluster's native scaling mechanism

### ACM/Hive Go types

```go
hive.openshift.io/v1.MachinePool — replica count, instance type, autoscaling
internal.open-cluster-management.io/v1beta1.ManagedClusterInfo — node verification
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.capacity.workers = 5
ComputeRequest.spec.capacity.autoscaling = {min: 3, max: 10}
  -> controller patches MachinePool replicas or autoscaling config
  -> controller monitors ManagedClusterInfo.nodeList for convergence
  -> controller updates ComputeRequest.status.capacity.currentWorkers
```

---

## UC-11: Cost tracking and chargeback

**Feature**: Cluster and tenant cost tracking via ACM Go API

As a platform operator  
I want to track resource consumption per cluster and tenant  
So that the CaaS platform can charge teams for actual usage

### Scenario: Calculate CPU-hours per cluster over a time period

**Given** Thanos metrics are available via the MCO observability stack  
**When** I query total CPU usage for cluster cluster-1 over the last 7 days  
**Then** I get the aggregate CPU-hours consumed  
**And** the result is broken down by day

### Scenario: Calculate resource usage per tenant namespace

**Given** tenant team-alpha has a namespace on cluster cluster-1  
**When** I query CPU and memory usage for namespace team-alpha over the last 30 days  
**Then** I get CPU-hours and memory-GiB-hours for the tenant  
**And** I can compare usage against the tenant's ResourceQuota limits

### Scenario: Generate a fleet-wide cost report

**Given** multiple clusters and tenants are being tracked  
**When** I generate a cost report for the current billing period  
**Then** the report lists per-cluster and per-tenant resource consumption  
**And** each entry includes: CPU-hours, memory-GiB-hours, storage-GiB, estimated cost  
**And** the report can be exported as CSV or JSON

### Scenario: Identify idle tenants for cleanup

**Given** cost tracking data is available for all tenants  
**When** I query tenants with zero CPU usage in the last 14 days  
**Then** I get a list of idle tenants with their last activity timestamp  
**And** these tenants are flagged as candidates for decommissioning (links to UC-08)

### Data sources

- Thanos/Prometheus via MCO (CPU, memory metrics over time)
- ManagedClusterInfo (node capacity, instance types for cost calculation)
- ResourceQuota per tenant (allocated vs actual usage)

### ComputeRequest controller equivalent

```
ComputeRequest.status.cost.cpuHours = 1234.5
ComputeRequest.status.cost.memoryGiBHours = 5678.9
ComputeRequest.status.cost.lastUpdated = "2026-08-24T00:00:00Z"
  -> controller queries Thanos for usage metrics periodically
  -> controller aggregates by cluster and tenant namespace
  -> controller updates ComputeRequest.status.cost
  -> external billing system reads status.cost for invoicing
```

---

## UC-12: Manage Identity Providers to all clusters in ACM

**Feature**: Identity Provider configuration and credential rotation via ACM Go API

As a platform operator  
I want to configure and rotate identity providers across all clusters  
So that the CaaS platform can enforce authentication policies and respond to security incidents

### Scenario: Configure GitHub Identity Provider on each cluster

**Given** a cluster is created or imported into ACM  
**When** I create a ManifestWork containing an OAuth configuration with GitHub IdP  
**Then** the GitHub IdP is configured on the cluster  
**And** users can authenticate via GitHub  
**And** the configuration is applied to all current and future clusters

### Scenario: Rotate htpasswd credentials after a leak

**Given** clusters have htpasswd Identity Provider configured  
**And** a credential leak has been detected  
**When** I generate new htpasswd credentials  
**And** I create/update ManifestWork with the new htpasswd Secret  
**And** I bind the ManifestWork to all affected clusters via Placement  
**Then** the htpasswd Secret is updated on all clusters  
**And** old credentials are invalidated  
**And** the rotation is tracked in an audit log

### Scenario: Apply IdP policy to clusters by label

**Given** multiple clusters exist with different environments (dev, staging, prod)  
**When** I create a Policy requiring GitHub IdP for prod clusters  
**And** I bind the policy to clusters with label `env=prod`  
**Then** all prod clusters must have GitHub IdP configured  
**And** non-compliant clusters are flagged  
**And** policy status shows per-cluster compliance

### ACM Go types

```go
work.open-cluster-management.io/v1.ManifestWork — OAuth config deployment
policy.open-cluster-management.io/v1.Policy — IdP requirement enforcement
cluster.open-cluster-management.io/v1beta1.Placement — cluster targeting
v1.Secret — htpasswd credentials
```

### OAuth configuration manifest (GitHub IdP)

```yaml
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: github
    type: GitHub
    mappingMethod: claim
    github:
      clientID: <github-client-id>
      clientSecret:
        name: github-client-secret
      organizations:
      - your-org
```

### htpasswd Secret manifest

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: htpasswd-secret
  namespace: openshift-config
type: Opaque
data:
  htpasswd: <base64-encoded-htpasswd-file>
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.identityProvider.type = "github"
ComputeRequest.spec.identityProvider.github.clientID = "..."
ComputeRequest.spec.identityProvider.github.organizations = ["your-org"]
  -> controller creates ManifestWork with OAuth config + Secret
  -> controller creates ManifestWork in cluster namespace
  -> controller watches Applied condition
  -> controller updates ComputeRequest.status.identityProvider.configured = true

Security incident (credential rotation):
ComputeRequest.spec.identityProvider.rotateCredentials = true
  -> controller generates new htpasswd file
  -> controller updates ManifestWork with new Secret
  -> controller tracks rotation timestamp in status
  -> controller logs rotation event for audit
```

---

## UC-13: Registry mirror for restricted-registry clusters

**Feature**: Image registry mirror for clusters that cannot pull from registry.redhat.io (ROKS, air-gapped, disconnected)

As a platform operator  
I want to configure image registry mirrors for clusters with restricted network access  
So that ACM can import and manage clusters that cannot reach public registries

### Scenario: Identify required images for a cluster

**Given** a ManagedCluster has been registered but klusterlet pods are in ImagePullBackOff  
**When** I run `acmlab registry list-images <cluster>`  
**Then** I see all images extracted from the cluster's ManifestWorks  
**And** I know exactly which images must be available on the spoke

### Scenario: Mirror images to a reachable registry

**Given** I have a target registry accessible from the spoke (e.g., us.icr.io, Quay.io)  
**When** I run `acmlab registry mirror-script <cluster> --target <registry>`  
**Then** I get a bash script with `skopeo copy` commands for each required image  
**And** after running the script, all ACM images are available in the target registry

### Scenario: Configure ManagedClusterImageRegistry on the hub

**Given** ACM images are mirrored to a reachable registry  
**And** I have a pull secret for the target registry  
**When** I run `acmlab registry configure <cluster> --mirror <registry> --pull-secret <path>`  
**Then** a ManagedClusterImageRegistry CR is created on the hub  
**And** ACM rewrites image references in klusterlet ManifestWorks to use the mirror  
**And** no changes are required on the spoke

### Scenario: Chicken-and-egg on unavailable clusters

**Given** a cluster is not yet imported (Available=Unknown) and images are blocked  
**When** I configure the registry mirror before the cluster becomes available  
**Then** the Placement uses tolerations to select unavailable clusters  
**And** the MCIR takes effect before the klusterlet finishes its first import

### Scenario: Remove registry mirror configuration

**Given** a ManagedClusterImageRegistry is configured for a cluster  
**When** I run `acmlab registry remove <cluster>`  
**Then** the ManagedClusterImageRegistry, Placement, and pull secret are deleted  
**And** ACM reverts to using original image references

### Scenario: Configure registry mirror for multiple clusters in parallel

**Given** I have mirrored the required images  
**When** I run `acmlab registry configure --from-file clusters.yaml --mirror quay.io/myorg`  
**Then** the registry mirror is configured on all clusters in parallel  
**And** a summary table shows per-cluster results

### ROKS findings (PoC)

Attempting to import a ROKS cluster revealed a fundamental network restriction: ROKS workers have no outbound access to external registries — not `registry.redhat.io`, not `quay.io`. All image pulls are intercepted and routed through `us.icr.io/armada-extensions/` (IBM's mirror), which does not include ACM/MCE images.

`ManagedClusterImageRegistry` can rewrite references to point at `us.icr.io`, but the IBM Cloud Container Registry Free plan (512 MB/month) is insufficient for the 6 required images (~300 MB amd64-only, ~750 MB all-arch). Workaround: ICR Standard plan or custom ROKS network configuration.

**Conclusion**: Importing ROKS into an external ACM hub is possible in principle but requires either ICR Standard plan or network changes to allow external registry access from ROKS workers.

### ACM Go types

`imageregistry.open-cluster-management.io/v1alpha1.ManagedClusterImageRegistry`  
`cluster.open-cluster-management.io/v1beta1.Placement`  
`cluster.open-cluster-management.io/v1beta2.ManagedClusterSetBinding`  
`v1.Secret` (pull secret for mirror registry)

### CLI commands

```
acmlab registry list-images <cluster>
acmlab registry mirror-script <cluster> --target <registry>
acmlab registry configure <cluster> --mirror <registry> [--pull-secret <path>]
acmlab registry status <cluster>
acmlab registry remove <cluster>
```

### MCP tools

`acm_registry_list_images`, `acm_registry_configure_mirror`, `acm_registry_mirror_status`, `acm_registry_generate_mirror_script`

### ComputeRequest controller equivalent

```
ComputeRequest.spec.import.restrictedRegistry = true
ComputeRequest.spec.import.mirrorRegistry = "us.icr.io/acm-mirror"
ComputeRequest.spec.import.pullSecretRef = "mirror-pull-secret"
  -> controller calls ListRequiredImages to identify needed images
  -> controller creates ManagedClusterSetBinding in cluster namespace
  -> controller creates Placement with tolerations for unavailable clusters
  -> controller creates ManagedClusterImageRegistry with source→mirror mappings
  -> controller waits for ClustersUpdated=True on MCIR status
  -> controller imports cluster via UC-07 flow
```

---

## UC-14: Automatic cluster reclamation (idle/expired clusters)

**Feature**: Automatic TTL enforcement and zombie cluster detection via ACM governance policies

As a platform operator
I want clusters to be automatically hibernated or deleted after their lifetime expires
So that zombie clusters and idle GPUs stop consuming budget

### Scenario: Enforce cluster TTL via governance policy

**Given** a ManagedCluster is labelled with `caas/ttl-hours=48` and `caas/expiry-date=<timestamp>`
**When** the expiry date passes
**Then** a `ConfigurationPolicy` marks the cluster NonCompliant
**And** the lifecycle controller hibernates the cluster (Hive-provisioned) or detaches it (imported)
**And** the owner and manager receive a notification before reclamation

### Scenario: Detect idle clusters via ACM Search

**Given** a cluster has been running for more than 7 days
**When** ACM Search shows no workload activity (CPU usage near zero)
**Then** the cluster is flagged as a zombie candidate
**And** the owner receives a notification to confirm or extend

### Scenario: Exception workflow

**Given** a cluster is approaching its TTL
**When** the owner annotates `caas/extend-request=true` with a justification
**Then** the policy allows a configurable extension period
**And** the exception is logged in the cluster annotations for audit

### Default TTLs (from platform architecture review)

| Cluster type | Default TTL |
|---|---|
| GPU cluster | 48 hours |
| Standard cluster | 36 hours |
| Long-running CI | configurable, max 7 days |

### ACM types

`policy.open-cluster-management.io/v1.ConfigurationPolicy` — detect expired TTL label
`cluster.open-cluster-management.io/v1.ManagedCluster` — `caas/ttl-hours`, `caas/expiry-date` labels
ACM Search API — cross-cluster idle workload detection

### ComputeRequest controller equivalent

```
ComputeRequest.spec.lifecycle.ttlHours = 48
  -> controller stamps ManagedCluster labels at provision time
  -> ConfigurationPolicy detects expiry
  -> controller triggers hibernate (Hive) or detach (imported)
  -> controller sends notification N hours before expiry
  -> controller updates ComputeRequest.status.phase = Reclaimed
```

---

## UC-15: Resource quota gates via governance policy

**Feature**: Enforce resource limits on managed clusters using ACM governance policies

As a platform operator
I want to detect and alert when clusters exceed approved resource limits
So that no team can silently consume GPU or node resources beyond their quota

### Scenario: Detect unauthorized GPU nodes

**Given** a cluster was provisioned with a single GPU node via Hive MachinePool
**When** a user manually adds a MachineSet with additional GPU nodes
**Then** a `ConfigurationPolicy` detects the MachineSet outside the managed MachinePool
**And** the cluster is marked NonCompliant
**And** the owner is notified with a 2-hour window to remove it

### Scenario: Enforce worker node count limit

**Given** a cluster has a label `caas/max-workers=5`
**When** the cluster's node count exceeds the label value
**Then** the governance policy marks it NonCompliant and sends an alert

### Scenario: Auto-remove unauthorized resources after timeout

**Given** a cluster has been NonCompliant for 2 hours with no owner response
**When** the reclamation controller runs
**Then** the unauthorized nodes are removed via MachinePool patch
**And** the action is logged in the ManagedCluster annotations

### Enforcement model

Educate, don't regulate (from platform architecture review): monitor and notify first, auto-remove only after 2h without response. Exception path: owner submits justification via annotation, whitelist entry created.

### ACM types

`policy.open-cluster-management.io/v1.ConfigurationPolicy` — enforce node/GPU limits
`work.open-cluster-management.io/v1.ManifestWork` — deploy ResourceQuota to spokes
`cluster.open-cluster-management.io/v1.ManagedCluster` — `caas/max-workers`, `caas/max-gpus` labels
ACM Search API — cross-cluster node inventory

### ComputeRequest controller equivalent

```
ComputeRequest.spec.quota.maxWorkers = 5
ComputeRequest.spec.quota.maxGpuNodes = 1
  -> controller stamps quota labels on ManagedCluster at provision time
  -> ConfigurationPolicy monitors node count vs label
  -> controller notifies on violation, removes after 2h
  -> controller updates ComputeRequest.status.conditions[QuotaCompliant]
```

---

## UC-16: Unique identity provider per cluster (security hardening)

**Feature**: Fleet-wide unique credentials and SSO enforcement via ACM ManifestWork and governance policies

As a platform operator
I want each cluster to have unique credentials
So that a single credential leak does not require rotating the entire fleet

### Scenario: Deploy SSO as primary IdP to all clusters

**Given** a cluster is registered in ACM (provisioned or imported)
**When** the IdP controller runs
**Then** a ManifestWork deploys Red Hat SSO OAuth configuration to the cluster
**And** a `Policy` monitors compliance — clusters without SSO are NonCompliant

### Scenario: Generate unique static credentials per cluster

**Given** a new cluster is being provisioned
**When** the provisioning controller creates the ManagedCluster
**Then** unique htpasswd credentials are generated (not shared with other clusters)
**And** they are deployed via ManifestWork as a secondary emergency IdP only

### Scenario: Automated credential rotation on leak

**Given** a credential leak is detected or a rotation is requested
**When** the IdP controller targets the affected cluster
**Then** new credentials are generated for that cluster only
**And** the ManifestWork is updated with the new OAuth Secret
**And** the rotation timestamp is recorded in ManagedCluster annotations

### Security improvement

| Before | After |
|---|---|
| 1 credential leak → rotate ALL clusters | 1 credential leak → rotate 1 cluster |
| Same static credentials across all clusters | Unique credentials per cluster |
| No SSO compliance monitoring | Policy enforces SSO on all clusters |

### ACM types

`work.open-cluster-management.io/v1.ManifestWork` — deploy OAuth config + unique Secret
`policy.open-cluster-management.io/v1.Policy` — enforce SSO presence fleet-wide
`policy.open-cluster-management.io/v1.ConfigurationPolicy` — detect missing SSO config

### ComputeRequest controller equivalent

```
ComputeRequest.spec.security.uniqueCredentials = true
ComputeRequest.spec.security.ssoRequired = true
  -> controller generates unique htpasswd at provision time
  -> controller creates ManifestWork with OAuth config + unique Secret
  -> Policy enforces SSO compliance fleet-wide
  -> controller tracks rotation in ComputeRequest.status.security.lastRotation
```

---

## UC-17: Cost center attribution and budget alerting

**Feature**: Fleet-wide cost attribution via ManagedCluster labels and Thanos budget alerting

As a platform operator
I want to attribute cloud spend to the team that requested each cluster
So that we can generate monthly chargeback reports and alert when teams exceed their budget

### Scenario: Stamp cost center at provision time

**Given** a user requests a cluster via the CaaS interface
**When** the provisioning controller creates the ManagedCluster
**Then** labels `caas/cost-center`, `caas/team`, `caas/owner`, `caas/business-impact` are set
**And** cloud resource tags are synchronized via ManifestWork where the provider supports it

### Scenario: Generate monthly chargeback report

**Given** the Thanos observability stack is running (UC-06)
**When** the monthly report is requested
**Then** CPU-hours and memory-GiB-hours are aggregated per `caas/cost-center` label
**And** estimated cloud cost is calculated per team
**And** the report is exported as CSV/JSON for the finance team

### Scenario: Alert when team exceeds budget threshold

**Given** a team has a label `caas/monthly-budget-usd=5000` on their ManagedClusters
**When** accumulated cost for the current month exceeds the budget
**Then** a `Policy` marks the clusters NonCompliant
**And** the team manager is notified

### ACM types

`cluster.open-cluster-management.io/v1.ManagedCluster` — `caas/cost-center`, `caas/team`, `caas/owner` labels
`policy.open-cluster-management.io/v1.Policy` — budget threshold alerting
`observability.open-cluster-management.io/v1beta2.MultiClusterObservability` — Thanos for usage aggregation

### ComputeRequest controller equivalent

```
ComputeRequest.spec.billing.costCenter = "engineering-platform"
ComputeRequest.spec.billing.monthlyBudgetUSD = 5000
  -> controller stamps cost center labels on ManagedCluster
  -> Thanos aggregates usage by cost-center label
  -> Policy alerts when budget threshold exceeded
  -> controller exports monthly report to ComputeRequest.status.billing
```

---

## UC-18: GPU sharing stack deployment fleet-wide (Kueue + Kyverno via ManifestWork)

**Feature**: Deploy and maintain the GPU sharing infrastructure (Kueue + Kyverno) across all GPU clusters via ACM ManifestWork

As a platform operator
I want the GPU sharing stack to be automatically deployed and maintained on all GPU clusters
So that teams can share GPU resources without manual operator intervention

### Scenario: Deploy Kueue + Kyverno to a new GPU cluster

**Given** a new GPU cluster is provisioned and labelled `gpu-sharing=enabled`
**When** the GPU sharing controller detects the new cluster
**Then** a ManifestWork deploys Kueue CRDs, controller, and initial ClusterQueue configuration
**And** a ManifestWork deploys Kyverno + admission policies (quota enforcement, GPU type validation, priority ceiling)
**And** a ConfigurationPolicy monitors that both operators are healthy

### Scenario: Detect and remediate GPU stack drift

**Given** a GPU cluster has `gpu-sharing=enabled` but Kueue is degraded
**When** the ConfigurationPolicy evaluates compliance
**Then** the cluster is marked NonCompliant
**And** the ManifestWork reconciliation redeploys the affected component

### Scenario: Create initial ClusterQueue per GPU type

**Given** a GPU cluster has H100 and L4 nodes
**When** the GPU sharing stack is deployed
**Then** separate ClusterQueues and ResourceFlavors are created per GPU type
**And** teams can submit workloads targeting a specific GPU type

### ACM types

`work.open-cluster-management.io/v1.ManifestWork` — deploy Kueue + Kyverno to GPU clusters
`policy.open-cluster-management.io/v1.ConfigurationPolicy` — enforce stack health fleet-wide
`cluster.open-cluster-management.io/v1.ManagedCluster` — `gpu-sharing=enabled` label

### ComputeRequest controller equivalent

```
ComputeRequest.spec.gpu.sharingEnabled = true
  -> controller stamps ManagedCluster label gpu-sharing=enabled
  -> ManifestWork deploys Kueue + Kyverno to cluster
  -> ConfigurationPolicy monitors operator health
  -> controller updates ComputeRequest.status.gpu.sharingReady = true
```

---

## UC-19: Multi-cluster GPU workload routing via ACM Placement

**Feature**: Route GPU workloads to the correct cluster based on GPU type, availability, and region using ACM Placement predicates

As a platform engineer
I want users to request GPUs by type without knowing which cluster has them
So that the service is transparent and routes automatically to available capacity

### Scenario: Route H100 request to correct cluster

**Given** multiple GPU clusters exist with different GPU types (`gpu-type=H100`, `gpu-type=L4`, `gpu-type=A100`)
**When** a user requests an H100 GPU
**Then** a Placement with `gpu-type=H100` and `gpu-available=true` predicates selects the correct cluster
**And** the PlacementDecision is returned to the API layer for namespace provisioning

### Scenario: Mark cluster unavailable when saturated

**Given** an H100 cluster reaches 90% GPU utilization
**When** Thanos detects the threshold crossing
**Then** the cluster label is updated to `gpu-available=false`
**And** new requests are routed to alternative H100 clusters or queued

### Scenario: Multi-region GPU selection

**Given** GPU clusters exist in `eu-gb` and `us-south`
**When** a user requests an H100 without region preference
**Then** Placement selects the cluster with the lowest current utilization across both regions

### ACM types

`cluster.open-cluster-management.io/v1beta1.Placement` — GPU type + availability predicates
`cluster.open-cluster-management.io/v1beta1.PlacementDecision` — selected cluster for API layer
`cluster.open-cluster-management.io/v1.ManagedCluster` — `gpu-type`, `gpu-count`, `gpu-region`, `gpu-available` labels
ACM Search API — cross-cluster GPU inventory query

### ComputeRequest controller equivalent

```
ComputeRequest.spec.gpu.type = "H100"
ComputeRequest.spec.gpu.count = 8
  -> controller creates Placement with gpu-type=H100, gpu-available=true
  -> Placement selects cluster with matching GPU
  -> controller reads PlacementDecision -> target cluster
  -> controller provisions namespace on target cluster (UC-21)
  -> controller updates ComputeRequest.status.gpu.cluster = "gpu-h100-eugb"
```

---

## UC-20: AI platform operator version fleet segregation

**Feature**: Manage GPU clusters segregated by operator version to support multi-version testing without cluster sharing conflicts

As a platform operator
I want each GPU cluster to run a specific operator version
So that teams testing different operator versions can get dedicated capacity without operator conflicts

### Scenario: Route workload to cluster with specific operator version

**Given** GPU clusters are labelled `ai-platform-version=2.17`, `ai-platform-version=2.18`, `ai-platform-build=nightly`
**When** a team requests a GPU environment with operator version 2.18
**Then** Placement selects only clusters with `ai-platform-version=2.18`
**And** the workload is submitted to the matched cluster

### Scenario: Enforce single operator version per cluster

**Given** a GPU cluster has `ai-platform-version=2.17`
**When** someone tries to install operator version 2.18 on the same cluster
**Then** a ConfigurationPolicy detects the version mismatch
**And** the cluster is marked NonCompliant with remediation instructions

### Scenario: Provision new cluster for new operator version

**Given** operator version 2.19 is released and no GPU cluster has it
**When** the first request for operator version 2.19 arrives
**Then** the controller provisions a new GPU cluster (UC-01)
**And** deploys operator version 2.19 via ManifestWork
**And** labels the cluster `ai-platform-version=2.19`
**And** adds it to the GPU routing pool (UC-19)

### Context (from platform architecture review)

The AI platform operator is a cluster-wide operator — only one version can be installed per cluster. Different testing teams (nightly builds, weekly builds, stable versions) require different versions simultaneously. Segregation by cluster is the only viable approach without operator conflict.

### ACM types

`cluster.open-cluster-management.io/v1.ManagedCluster` — `ai-platform-version`, `ai-platform-channel`, `ai-platform-build` labels
`policy.open-cluster-management.io/v1.ConfigurationPolicy` — enforce single operator version per cluster
`cluster.open-cluster-management.io/v1beta1.Placement` — route by operator version label
`work.open-cluster-management.io/v1.ManifestWork` — deploy the AI platform operator to new GPU cluster

### ComputeRequest controller equivalent

```
ComputeRequest.spec.gpu.aiPlatformVersion = "2.18"
  -> controller creates Placement with ai-platform-version=2.18 predicate
  -> if no cluster matches: provisions new GPU cluster + deploys operator version 2.18 (UC-01)
  -> controller labels new cluster and adds to routing pool
  -> controller routes workload to matched cluster
```

---

## UC-21: Elastic GPU capacity — automatic on-demand provisioning on saturation

**Feature**: Automatically provision new GPU clusters when shared capacity is saturated, and hibernate them when demand drops

As a platform operator
I want the GPU pool to scale automatically
So that teams are not rejected when shared GPU clusters are full

### Scenario: Detect GPU saturation and provision new cluster

**Given** shared GPU cluster utilization exceeds 85% for 30+ minutes
**When** Thanos triggers a saturation alert via ConfigurationPolicy
**Then** the elastic controller provisions a new GPU cluster (UC-01) with the same GPU type
**And** deploys the GPU sharing stack (UC-18)
**And** labels the cluster and adds it to the GPU routing Placement pool (UC-19)
**And** sends a cost notification to the team manager (UC-17)

### Scenario: Hibernate on-demand cluster after demand drops

**Given** an on-demand GPU cluster was provisioned and utilization drops below 10% for 2+ hours
**When** the elastic controller evaluates the fleet
**Then** the cluster is hibernated (UC-05) to reduce cost
**And** its `gpu-available=false` label is set in ACM

### Scenario: Spot instance burst for temporary spikes

**Given** reserved GPU inventory is full and a burst is needed
**When** the elastic controller provisions a spot-instance GPU cluster
**Then** it is labelled `gpu-cost-tier=spot`
**And** only queued workloads that explicitly accept spot are routed to it

### GPU capacity tiers (from platform architecture review)

| Tier | GPU type | Strategy |
|---|---|---|
| Reserved | H100, A100, B300 | Always on — too expensive to stop/start |
| On-demand | L4, T4 | Provision when needed, hibernate after |
| Spot burst | Any | Spot instances for spiky load |

### ACM types

`observability.open-cluster-management.io/v1beta2.MultiClusterObservability` — Thanos utilization monitoring
`policy.open-cluster-management.io/v1.ConfigurationPolicy` — detect saturation threshold
`hive.openshift.io/v1.ClusterDeployment` — provision new GPU cluster on-demand (UC-01)
`cluster.open-cluster-management.io/v1beta1.Placement` — add new cluster to routing pool
ACM lifecycle (UC-05) — hibernate/resume on-demand clusters

### ComputeRequest controller equivalent

```
ComputeRequest.spec.gpu.elasticCapacity = true
ComputeRequest.spec.gpu.saturationThreshold = 85
  -> Thanos monitors GPU utilization across fleet
  -> ConfigurationPolicy triggers at threshold
  -> controller provisions new GPU cluster (UC-01)
  -> controller deploys sharing stack (UC-18) + labels + routing (UC-19)
  -> controller updates ComputeRequest.status.gpu.elasticCluster = "gpu-ondemand-001"
```

---

## UC-22: ClusterSet management — team isolation and multi-tenancy

**Feature**: Lifecycle management of ManagedClusterSets for team-based fleet isolation

As a platform operator
I want to create and manage ClusterSets per team
So that each team can only see and manage their own clusters

### Scenario: Create a team ClusterSet

**Given** a new engineering team needs cluster access
**When** I run `acmlab clusterset create team-serving --teams serving-ns`
**Then** a ManagedClusterSet is created
**And** a ManagedClusterSetBinding is created in the serving-ns namespace
**And** teams in that namespace can now use Placements scoped to their ClusterSet

### Scenario: Assign a cluster to a team ClusterSet

**Given** cluster spoke3 was provisioned to the default ClusterSet
**When** I run `acmlab clusterset assign spoke3 --to team-serving`
**Then** the cluster's `cluster.open-cluster-management.io/clusterset` label is updated
**And** the cluster moves to the team-serving ClusterSet
**And** it is no longer visible to other teams' Placements

### Scenario: Enforce every cluster belongs to a named ClusterSet

**Given** a ConfigurationPolicy is active
**When** a cluster is found with label `clusterset=default`
**Then** the cluster is marked NonCompliant
**And** an alert is sent to the platform team to assign it

### ACM types

`cluster.open-cluster-management.io/v1beta2.ManagedClusterSet`
`cluster.open-cluster-management.io/v1beta2.ManagedClusterSetBinding`
`cluster.open-cluster-management.io/v1.ManagedCluster` — label `cluster.open-cluster-management.io/clusterset`

### ComputeRequest controller equivalent

```
ComputeRequest.spec.team = "serving"
  -> controller creates ManagedClusterSet if not exists
  -> controller creates ManagedClusterSetBinding in team namespace
  -> controller stamps cluster label at provision time
  -> controller updates ComputeRequest.status.clusterSet = "team-serving"
```

---

## UC-23: Multi-architecture cluster matrix provisioning (QA)

**Feature**: Automated provisioning of a version × architecture × operator version matrix of clusters for QA regression testing

As a QA engineer
I want to provision a full test matrix of clusters with one command
So that I can run regression tests across all supported combinations

### Scenario: Provision a version × architecture matrix

**Given** a matrix.yaml specifying OCP versions [4.18, 4.19], architectures [amd64, ppc64le], and operator versions [3.4, 3.5]
**When** I run `acmlab matrix provision --from-file matrix.yaml`
**Then** all combinations are provisioned in parallel (8 clusters)
**And** each cluster is labelled with `ocp-version`, `arch`, `ai-platform-version`
**And** a summary table shows per-combination status

### Scenario: Route a test workload to a specific combination

**Given** the matrix clusters are Running
**When** a Placement specifies `ocp-version=4.19`, `arch=amd64`, `ai-platform-version=3.5`
**Then** the Placement selects exactly the matching cluster
**And** the test workload is dispatched to that cluster

### Scenario: Destroy the full matrix after testing

**Given** the test run is complete
**When** I run `acmlab matrix destroy --from-file matrix.yaml`
**Then** all matrix clusters are destroyed in parallel

### ACM types

`hive.openshift.io/v1.ClusterDeployment` — one per matrix cell (reuses UC-01)
`cluster.open-cluster-management.io/v1.ManagedCluster` — labels: `ocp-version`, `arch`, `ai-platform-version`
`cluster.open-cluster-management.io/v1beta1.Placement` — combined label predicate routing

### Package

`internal/matrix/` (to be created, wraps UC-01 + batch operations)

---

## UC-24: Per-team compliance reporting via ClusterSet-scoped policies

**Feature**: Governance policies scoped per ClusterSet with per-team compliance reporting

As a platform operator
I want to see compliance status per team
So that I know which team's clusters are violating security or operational policies

### Scenario: Apply a policy scoped to a team ClusterSet

**Given** the team-serving ClusterSet exists
**When** I run `acmlab policy apply gpu-driver-policy --cluster-set team-serving`
**Then** the policy is bound to a Placement scoped to team-serving clusters only
**And** compliance state is reported per cluster in that set

### Scenario: Generate a fleet-wide per-team compliance report

**Given** multiple ClusterSets exist (team-serving, team-training, ci-matrix)
**When** I run `acmlab compliance report --all`
**Then** a table shows per-ClusterSet: total clusters, compliant count, non-compliant count
**And** drill-down shows which policy each cluster is violating

### ACM types

`policy.open-cluster-management.io/v1.Policy`
`policy.open-cluster-management.io/v1.PlacementBinding` — scoped to ClusterSet Placement
`cluster.open-cluster-management.io/v1beta1.Placement` — `clusterSets: [team-set]`

### Package

Extends `internal/policy/` (UC-02)

---

## UC-25: ClusterPool and ClusterClaim — pre-warmed cluster self-service

**Feature**: Hive ClusterPool management for instant cluster access (seconds vs 10+ minutes)

As a developer or QA engineer
I want to claim a pre-warmed cluster instantly
So that I don't wait 10 minutes for provisioning every time I need a cluster

### Scenario: Create a cluster pool

**Given** I need a pool of 3 always-ready IBM Cloud clusters with OCP 4.19
**When** I run `acmlab pool create amd64-419 --size 3 --image-set img4.19-multi --platform ibmcloud`
**Then** Hive provisions 3 clusters and keeps them in the Hibernating state
**And** `acmlab pool list` shows 3 clusters ready to claim

### Scenario: Claim a cluster instantly

**Given** the amd64-419 pool has at least 1 ready cluster
**When** I run `acmlab claim amd64-419 --ttl 48h`
**Then** a ClusterClaim is created and bound in seconds
**And** a kubeconfig is returned immediately
**And** Hive automatically provisions a replacement cluster to maintain pool size

### Scenario: Release a claim back to the pool

**Given** I'm done with my claimed cluster
**When** I run `acmlab claim release my-claim`
**Then** the cluster is returned to the pool (or destroyed if pool is full)
**And** another team member can claim it immediately

### Why this matters

Current flow: provision → wait 10min → use → destroy. With ClusterPool: claim → use instantly → release. For QA running hundreds of test cycles per week, this is the single highest-impact improvement.

### ACM/Hive types

`hive.openshift.io/v1.ClusterPool`
`hive.openshift.io/v1.ClusterClaim`

### Package

`internal/pool/` (to be created)

---

## UC-26: Multi-cluster networking (Submariner) for distributed training

**Feature**: Enable cross-cluster pod networking for distributed training workloads via ACM Submariner Add-On

As a training infrastructure engineer
I want pods in different GPU clusters to communicate directly by IP
So that distributed training jobs (PyTorch distributed, Ray, Kueue multi-cluster) can span multiple clusters

### Scenario: Enable Submariner on a ClusterSet

**Given** two GPU clusters exist in the team-training ClusterSet
**When** I run `acmlab submariner enable --cluster-set team-training`
**Then** ACM deploys the Submariner gateway and route-agent on each cluster
**And** pods in cluster A can reach pods in cluster B by ClusterIP

### Scenario: Verify cross-cluster connectivity

**Given** Submariner is enabled on team-training
**When** I run `acmlab submariner status --cluster-set team-training`
**Then** gateway tunnel health and latency per cluster pair is shown

### Scenario: Distributed training job spanning two GPU clusters

**Given** Submariner is active and Kueue multi-cluster is configured
**When** a PyTorch distributed training job requests 16 GPUs
**Then** Kueue schedules 8 GPUs on cluster-gpu-1 and 8 on cluster-gpu-2
**And** the workers communicate via Submariner tunnels transparently

### Spike required

Before implementation, validate: submariner-addon is installed on hub, IBM Cloud VPC firewall allows IPSec/VXLAN ports, 

### ACM types

`addon.open-cluster-management.io/v1alpha1.ManagedClusterAddOn` — name: submariner
`submariner.io/v1alpha1.SubmarinerConfig`
`cluster.open-cluster-management.io/v1beta2.ManagedClusterSet` — Submariner scopes to a ClusterSet

### Package

`internal/submariner/` (to be created after spike)

---

## UC-27: Operator version pinning via OperatorPolicy

**Feature**: Enforce specific operator versions on managed clusters via ACM OperatorPolicy

As a platform operator
I want to pin operator versions across the fleet
So that clusters run validated operator releases and do not auto-upgrade to untested versions

### Scenario: Pin an operator to a specific version on all clusters

**Given** the GPU sharing operator is installed on multiple clusters
**When** I create an OperatorPolicy requiring version 2.17.3
**And** I bind it to all GPU clusters via Placement
**Then** clusters running a different version are marked NonCompliant
**And** the policy status shows which clusters need remediation

### Scenario: Prevent automatic operator upgrades

**Given** an OperatorPolicy pins the operator to channel stable-2.17
**When** a new version 2.18.0 appears in the fast channel
**Then** the pinned clusters remain on 2.17.x
**And** the policy blocks the upgrade until the pin is updated

### Scenario: Staged rollout of operator upgrade across clusters

**Given** the platform team validates operator version 2.18.0
**When** the OperatorPolicy is updated to 2.18.0 for batch-1 clusters
**Then** batch-1 clusters upgrade to 2.18.0
**And** batch-2 clusters remain on 2.17.x until their policy is updated

### ACM types

```go
policy.open-cluster-management.io/v1beta1.OperatorPolicy
cluster.open-cluster-management.io/v1beta1.Placement — cluster targeting
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.operators[0].name = "gpu-sharing-operator"
ComputeRequest.spec.operators[0].version = "2.17.3"
ComputeRequest.spec.operators[0].channel = "stable-2.17"
  -> controller creates OperatorPolicy with version constraint
  -> controller binds to target clusters via Placement
  -> controller monitors compliance status
  -> controller updates ComputeRequest.status.operators[0].compliant = true
```

---

## UC-28: Certificate expiry detection fleet-wide

**Feature**: Detect and alert on expiring certificates across all managed clusters via ACM CertificatePolicy

As a platform operator
I want to detect certificates approaching expiry across the fleet
So that certificate-related outages are prevented before they occur

### Scenario: Detect certificates expiring within 30 days

**Given** a CertificatePolicy is deployed to all managed clusters
**When** any certificate in a monitored namespace expires within 30 days
**Then** the cluster is marked NonCompliant
**And** the policy status identifies the expiring certificate, namespace, and expiry date

### Scenario: Monitor API server and ingress certificates

**Given** CertificatePolicy targets the openshift-config and openshift-ingress namespaces
**When** the API server or wildcard ingress certificate is approaching expiry
**Then** the platform team is alerted with sufficient lead time to rotate

### Scenario: Fleet-wide certificate health report

**Given** CertificatePolicy is active on all clusters
**When** I query policy compliance across the fleet
**Then** I get a report showing: cluster name, certificate name, days until expiry
**And** clusters are sorted by most urgent expiry first

### ACM types

```go
policy.open-cluster-management.io/v1.CertificatePolicy
policy.open-cluster-management.io/v1.Policy — wraps CertificatePolicy for distribution
cluster.open-cluster-management.io/v1beta1.Placement — cluster targeting
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.security.certExpiryThresholdDays = 30
  -> controller creates CertificatePolicy targeting cluster namespaces
  -> controller wraps in Policy and binds via Placement
  -> controller monitors compliance status
  -> controller updates ComputeRequest.status.conditions[CertificatesHealthy]
```

---

## UC-29: Security baseline via Gatekeeper/OPA constraints

**Feature**: Deploy and enforce Open Policy Agent (Gatekeeper) security constraints across the fleet via ACM ManifestWork and governance policies

As a platform operator
I want a security baseline enforced on all clusters
So that common misconfigurations (privileged containers, host networking, missing resource limits) are prevented fleet-wide

### Scenario: Deploy Gatekeeper with security constraint templates

**Given** a new cluster is registered in ACM
**When** the security baseline controller runs
**Then** a ManifestWork deploys Gatekeeper and a set of ConstraintTemplate CRDs
**And** constraints are created for: no privileged containers, no host networking, required resource limits, no latest tag

### Scenario: Detect violations of security baseline

**Given** Gatekeeper constraints are active on a cluster
**When** a user deploys a pod with `securityContext.privileged: true`
**Then** Gatekeeper blocks the pod admission
**And** a ConfigurationPolicy on the hub detects the violation count
**And** the cluster compliance status reflects the violation

### Scenario: Exempt specific namespaces from constraints

**Given** certain system namespaces require privileged access (e.g., monitoring agents)
**When** the constraint is configured with namespace exclusions
**Then** pods in excluded namespaces are allowed
**And** all other namespaces remain under the security baseline

### ACM types

```go
work.open-cluster-management.io/v1.ManifestWork — deploy Gatekeeper + ConstraintTemplates + Constraints
policy.open-cluster-management.io/v1.ConfigurationPolicy — monitor Gatekeeper violation count
policy.open-cluster-management.io/v1.Policy — distribute and report compliance
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.security.baseline = "strict"
ComputeRequest.spec.security.exemptNamespaces = ["openshift-monitoring"]
  -> controller creates ManifestWork with Gatekeeper + constraints
  -> ConfigurationPolicy monitors violation count
  -> controller updates ComputeRequest.status.conditions[SecurityBaselineCompliant]
```

---

## UC-30: Policy automation — Ansible auto-remediation

**Feature**: Automatic remediation of NonCompliant policies via ACM PolicyAutomation and Ansible Automation Platform

As a platform operator
I want NonCompliant policies to trigger automated remediation playbooks
So that common issues are resolved without manual intervention

### Scenario: Auto-remediate a NonCompliant certificate policy

**Given** a CertificatePolicy (UC-28) detects an expiring certificate
**When** the cluster becomes NonCompliant
**Then** a PolicyAutomation CR triggers an Ansible playbook
**And** the playbook renews the certificate on the spoke
**And** the cluster returns to Compliant status

### Scenario: Auto-remediate drift in security baseline

**Given** a Gatekeeper constraint (UC-29) is manually removed from a spoke
**When** the ConfigurationPolicy detects the missing constraint
**Then** PolicyAutomation triggers a playbook to redeploy the constraint
**And** the remediation is logged with timestamp and cluster name

### Scenario: PolicyAutomation with approval gate

**Given** a critical policy violation requires human approval before auto-remediation
**When** the PolicyAutomation mode is set to `once` with manual trigger
**Then** the violation is logged and a notification sent
**And** remediation only runs after an operator approves

### ACM types

```go
policy.open-cluster-management.io/v1beta1.PolicyAutomation
policy.open-cluster-management.io/v1.Policy — triggers the automation
```

### Ansible Automation Platform integration

- PolicyAutomation CR references an Ansible Automation Platform credential secret
- The secret contains the AAP tower URL and authentication token
- Playbooks run on the spoke cluster via ManagedServiceAccount token (UC-35) or direct kubeconfig
- Supported modes: `once` (manual trigger), `everyEvent` (auto on each violation), `disabled`

### ComputeRequest controller equivalent

```
ComputeRequest.spec.policyAutomation.enabled = true
ComputeRequest.spec.policyAutomation.mode = "everyEvent"
ComputeRequest.spec.policyAutomation.aapSecretRef = "aap-credentials"
  -> controller creates PolicyAutomation CR linked to NonCompliant policies
  -> AAP runs remediation playbook on violation
  -> controller tracks remediation events in ComputeRequest.status.remediations[]
```

---

## UC-31: SCAP scanning via Compliance Operator

**Feature**: Deploy and manage OpenSCAP compliance scanning across the fleet via ACM ManifestWork and governance policies

As a platform operator
I want to run SCAP compliance scans on all clusters
So that the fleet meets regulatory and security benchmarks (CIS, NIST, PCI-DSS)

### Scenario: Deploy Compliance Operator to all clusters

**Given** a cluster is registered in ACM
**When** the compliance controller runs
**Then** a ManifestWork deploys the Compliance Operator
**And** a ScanSettingBinding is created for the CIS benchmark profile
**And** the operator begins scheduled scans

### Scenario: Detect non-compliant SCAP findings

**Given** a ComplianceScan completes on a spoke cluster
**When** findings include FAIL results for CIS benchmark rules
**Then** a ConfigurationPolicy on the hub detects the non-compliant scan results
**And** the cluster is marked NonCompliant with the specific failing rules

### Scenario: Remediate SCAP findings automatically

**Given** a ComplianceScan found remediable issues
**When** the ComplianceRemediation CR is applied
**Then** the failing configurations are corrected on the spoke
**And** a rescan confirms the issues are resolved
**And** PolicyAutomation (UC-30) can trigger this automatically

### Scenario: Fleet-wide compliance report by benchmark

**Given** all clusters run scheduled SCAP scans
**When** I query compliance status across the fleet
**Then** I get a report per benchmark: cluster name, pass count, fail count, score percentage
**And** clusters below a minimum score threshold are flagged

### ACM types

```go
work.open-cluster-management.io/v1.ManifestWork — deploy Compliance Operator + ScanSettingBinding
policy.open-cluster-management.io/v1.ConfigurationPolicy — monitor ComplianceScan results
policy.open-cluster-management.io/v1.Policy — distribute and report compliance
compliance.openshift.io/v1alpha1.ComplianceScan
compliance.openshift.io/v1alpha1.ScanSettingBinding
```

### ComputeRequest controller equivalent

```
ComputeRequest.spec.compliance.benchmarks = ["cis", "nist-800-53"]
ComputeRequest.spec.compliance.schedule = "0 2 * * *"
ComputeRequest.spec.compliance.minScore = 85
  -> controller creates ManifestWork with Compliance Operator + profiles
  -> ConfigurationPolicy monitors scan results
  -> controller updates ComputeRequest.status.compliance.score
  -> controller flags clusters below minScore
```

---

## UC-32: GitOps fleet deployment via ACM ApplicationSet integration

**Feature**: GitOps-driven team tooling deployment using ArgoCD ApplicationSet with ACM Placement as cluster selector

As a platform operator
I want team tooling to be deployed via Git PRs with automatic drift detection
So that cluster configuration stays reconciled without manual acmlab commands

### Scenario: Deploy team tooling via GitOps across a ClusterSet

**Given** a git repo contains Kueue configuration for the training team's clusters
**When** an ApplicationSet targeting the training team's ClusterSet is applied
**Then** ArgoCD creates an Application per cluster automatically
**And** configuration drift is detected and corrected continuously
**And** a git PR to update the config triggers a sync across the fleet

### Why different from ManifestWork

ManifestWork (UC-03, UC-18) is imperative raw-manifest push — no drift detection, no rollback history. ApplicationSet is declarative GitOps — drift is continuously reconciled, rollback is a git revert, and sync health is visible in the ArgoCD dashboard.

### ACM types

`argoproj.io/v1alpha1.ApplicationSet` — with clusterDecisionResource generator pointing to Placement
`cluster.open-cluster-management.io/v1beta1.Placement` — cluster selection
`cluster.open-cluster-management.io/v1beta1.PlacementDecision` — consumed by ApplicationSet generator

### ComputeRequest controller equivalent

```
ComputeRequest.spec.gitops.repoURL = "https://github.com/org/cluster-configs"
ComputeRequest.spec.gitops.path = "teams/training"
ComputeRequest.spec.gitops.clusterSet = "team-training"
  -> controller creates ApplicationSet with clusterDecisionResource generator
  -> ArgoCD deploys and continuously reconciles team config on all clusters
  -> controller updates ComputeRequest.status.gitops.syncStatus
```

---

## UC-33: ManifestWorkReplicaSet with progressive rollout for safe fleet updates

**Feature**: Safe rolling updates of fleet-wide manifests using ManifestWorkReplicaSet rollout strategies

As a platform operator
I want to update the GPU sharing stack across all clusters in a controlled rollout
So that a bad manifest update does not take down the entire GPU fleet simultaneously

### Scenario: Progressive rollout of GPU sharing stack update

**Given** the GPU sharing stack is deployed on 20 GPU clusters via ManifestWorkReplicaSet
**When** a new version of the Kueue configuration is applied
**Then** the rollout proceeds 2 clusters at a time
**And** stops automatically if more than 10% of clusters report Degraded health
**And** operators can inspect and roll back before proceeding

### Scenario: Rollout with CEL-based health check

**Given** a ManifestWorkReplicaSet with a progressDeadline of 5 minutes
**When** a cluster does not report healthy within the deadline
**Then** the rollout pauses and alerts the operator
**And** no additional clusters are updated until the issue is resolved

### Why this matters

Without ManifestWorkReplicaSet, UC-18 (GPU sharing stack) pushes to all clusters simultaneously with no rollback. A bad manifest takes all GPU clusters offline at once. ManifestWorkReplicaSet's Progressive strategy is the production-safe alternative.

### ACM types

`work.open-cluster-management.io/v1alpha1.ManifestWorkReplicaSet` — with RolloutStrategy
`cluster.open-cluster-management.io/v1beta1.Placement` — cluster selection

### Package

Replaces `work.open-cluster-management.io/v1.ManifestWork` in UC-18 and UC-03 for production use

---

## UC-35: ManagedServiceAccount + cluster-proxy for credential-free spoke access

**Feature**: Automatic short-lived credential provisioning for hub-to-spoke API access without static kubeconfigs

As a platform operator
I want the hub to access spoke clusters using auto-rotated credentials
So that no static kubeconfig or long-lived credentials need to be managed per cluster

### Scenario: Hub-initiated spoke access without static credentials

**Given** the ManagedServiceAccount addon is enabled on a spoke cluster
**When** a hub-side operation needs spoke API access (diagnose, cert recovery, policy automation)
**Then** the hub authenticates using an automatically rotated short-lived token
**And** no static kubeconfig for that spoke is required on the hub

### Scenario: Policy automation with auto-provisioned spoke credentials

**Given** PolicyAutomation (UC-30) triggers an Ansible job to remediate a NonCompliant cluster
**When** the playbook needs to run kubectl commands against the spoke
**Then** ManagedServiceAccount provides a fresh rotated token to the playbook
**And** the token expires after the job completes

### ACM types

`authentication.open-cluster-management.io/v1beta1.ManagedServiceAccount`
`addon.open-cluster-management.io/v1alpha1.ManagedClusterAddOn` — name: managed-serviceaccount, cluster-proxy

### Package

`internal/access/` (to be created; enhances lifecycle, importing, and policyautomation packages)

---

## UC-36: ACM hub backup and restore for disaster recovery

**Feature**: Scheduled backup of all ACM hub state (clusters, policies, manifests) with automated restore

As a platform operator
I want the ACM hub state to be backed up regularly
So that the entire fleet can be recovered in minutes if the hub cluster is lost

### Scenario: Hub backup and restore after disaster

**Given** daily hub backups are scheduled to object storage
**When** the hub cluster is accidentally destroyed
**Then** restoring the backup on a new cluster reconnects all Hive-provisioned clusters automatically
**And** imported clusters are flagged for re-import via acmlab batch import

### Scenario: Verify backup completeness

**Given** a hub backup exists
**When** `acmlab hub backup verify --from <backup>` is run
**Then** all critical resources (ManagedClusters, ClusterDeployments, Policies, ManifestWorks) are confirmed present
**And** the backup age and storage location are reported

### ACM types

`cluster.open-cluster-management.io/v1beta1.BackupSchedule`
`cluster.open-cluster-management.io/v1beta1.Restore`
OADP `DataProtectionApplication` — for object storage backend (S3, IBM COS)

### Package

`internal/backup/` (to be created)

---

## UC-37: Worker node flavor change via MachinePool rolling replacement

**Feature**: Change worker node instance type post-provisioning via Hive MachinePool platform update

As a platform operator
I want to change the worker node flavor of a running cluster
So that I can right-size compute without reprovisioning the cluster

### Scenario: Change worker instance type

**Given** a cluster has workers of type `bx2-4x16` and needs more CPU
**When** I run `acmlab scaling set-flavor cluster-1 --worker-type cx2-8x16`
**Then** Hive creates new workers with the new instance type
**And** old workers are drained and removed
**And** the cluster remains available throughout the rolling replacement

### Limitations

- Control plane flavor cannot be changed post-provisioning
- Only works for Hive-provisioned clusters with a MachinePool
- For clusters using HyperShift (UC-38), use NodePool flavor update instead

### ACM/Hive types

`hive.openshift.io/v1.MachinePool` — patch `spec.platform.<provider>.type`

### Package

Extends `internal/scaling/` (UC-10)

---

## UC-38: HyperShift (HostedCluster) provisioning — control-plane-free clusters

**Feature**: Provision OCP clusters with hosted control planes using HyperShift, eliminating control plane VM costs

As a platform operator
I want to provision clusters where the control plane runs as pods on the hub
So that I reduce cost and provisioning time for short-lived clusters

### Scenario: Provision a HyperShift cluster

**Given** the hypershift-addon is enabled on the ACM hub
**When** I run `acmlab provision create cluster-1 --type hypershift --workers 2 --worker-type bx2-4x16`
**Then** a HostedCluster and NodePool are created (no control plane VMs)
**And** the cluster is available in 3-4 minutes (vs 10+ minutes for Hive IPI)
**And** control plane runs as pods on the hub cluster

### Scenario: Scale HyperShift worker nodes

**Given** a HyperShift cluster exists
**When** I run `acmlab scaling set cluster-1 --replicas 4`
**Then** the NodePool replicas are updated (not MachinePool)

### Why this matters

HyperShift eliminates ~30% of cluster cost (no control plane VMs). Ideal for short-lived QA matrix clusters (UC-23) where many clusters are needed simultaneously.

### Prerequisites (spike needed)

Verify `hypershift-addon` is enabled on hub and IBM Cloud NodePool support is available.

### ACM types

`hypershift.openshift.io/v1beta1.HostedCluster`
`hypershift.openshift.io/v1beta1.NodePool`

### Package

Extends `internal/provisioning/` with HyperShift provisioning path

---

## UC-39: Cloud-provider native scaling for imported clusters

**Feature**: Scaling imported clusters using cloud-provider native APIs via ManifestWork or direct API calls

As a platform operator
I want to scale imported clusters that have no Hive MachinePool
So that the CaaS platform can adjust capacity for clusters not provisioned by ACM

### Scenario: Scale an imported cluster via cloud-provider API

**Given** a ManagedCluster was imported (not provisioned by Hive)
**And** the cluster's cloud provider API credentials are available
**When** I trigger a scaling operation targeting the cloud-provider native API
**Then** worker nodes are added or removed via the provider's scaling mechanism
**And** ManagedClusterInfo reflects the updated node count

### Scenario: Detect scaling is unavailable for imported clusters without provider config

**Given** an imported cluster has no cloud-provider API credentials configured
**When** I attempt to scale via native API
**Then** the operation reports that cloud-provider credentials are required
**And** suggests configuring the provider or using the cluster's native scaling tools

### Relationship to UC-10

UC-10 scales Hive-provisioned clusters via MachinePool patches. UC-39 covers the complementary path for imported clusters where no MachinePool exists.

### ACM types

`cluster.open-cluster-management.io/v1.ManagedCluster` (imported, no ClusterDeployment)
`internal.open-cluster-management.io/v1beta1.ManagedClusterInfo` (node verification)
`work.open-cluster-management.io/v1.ManifestWork` (deploy scaling manifests if applicable)

### Package

Extends `internal/scaling/` (UC-10)

---

## UC-40: Cluster API (CAPI) provisioning for vanilla Kubernetes

**Feature**: Provision non-OpenShift Kubernetes clusters using Cluster API as an alternative to Hive

As a platform operator
I want to provision vanilla Kubernetes clusters via CAPI
So that the CaaS platform supports workloads that do not require OpenShift

### Scenario: Provision a CAPI cluster

**Given** CAPI controllers are installed on the hub cluster
**And** an infrastructure provider (e.g., AWS, vSphere) is configured
**When** I create a CAPI Cluster, KubeadmControlPlane, and MachineDeployment
**Then** a vanilla Kubernetes cluster is provisioned
**And** the cluster is registered as a ManagedCluster in ACM
**And** I can manage it alongside OpenShift clusters in the fleet

### Scenario: Destroy a CAPI cluster

**Given** a CAPI-provisioned cluster exists
**When** I delete the CAPI Cluster resource
**Then** the infrastructure is deprovisioned
**And** the ManagedCluster is removed from ACM

### Relationship to UC-01

UC-01 provisions OpenShift clusters via Hive ClusterDeployment. UC-40 covers the alternative path for vanilla Kubernetes via Cluster API.

### ACM types

`cluster.x-k8s.io/v1beta1.Cluster`
`controlplane.cluster.x-k8s.io/v1beta1.KubeadmControlPlane`
`cluster.x-k8s.io/v1beta1.MachineDeployment`
`cluster.open-cluster-management.io/v1.ManagedCluster`

### Package

Extends `internal/provisioning/` with CAPI provisioning path

---

## UC-41: Kubernetes cluster hibernate via CAPI scale-to-zero

**Feature**: Hibernate vanilla Kubernetes clusters provisioned via CAPI by scaling worker nodes to zero

As a platform operator
I want to hibernate CAPI-provisioned clusters by scaling workers to zero
So that I can reduce cost for idle vanilla Kubernetes clusters

### Scenario: Hibernate a CAPI cluster by scaling to zero

**Given** a CAPI-provisioned cluster has a MachineDeployment with replicas > 0
**When** I patch the MachineDeployment to set replicas to 0
**Then** all worker nodes are drained and removed
**And** the control plane remains running (or is also scaled down depending on provider)
**And** the ManagedCluster condition `Available` transitions to `Unknown`

### Scenario: Resume a CAPI cluster by scaling up

**Given** a CAPI cluster has MachineDeployment replicas set to 0
**When** I patch the MachineDeployment to restore the original replica count
**Then** worker nodes are provisioned
**And** the cluster becomes Available again

### Relationship to UC-05

UC-05 hibernates Hive-provisioned OpenShift clusters via `ClusterDeployment.spec.powerState`. UC-41 covers the equivalent for CAPI-provisioned vanilla Kubernetes clusters using MachineDeployment scale-to-zero.

### ACM types

`cluster.x-k8s.io/v1beta1.MachineDeployment` (replica count)
`cluster.open-cluster-management.io/v1.ManagedCluster` (availability tracking)

### Package

Extends `internal/lifecycle/` with CAPI hibernate path

---

## UC-42: Workload disaster recovery via Velero and GitOps

**Feature**: Recover workloads on a replacement cluster after catastrophic failure

As a platform operator
I want to restore workloads to a new cluster when the original is lost
So that the CaaS platform can guarantee business continuity

### Scenario: Detect a failed cluster and trigger recovery

**Given** a ManagedCluster "prod-eu-1" has condition `Available = False` for longer than the SLA threshold
**And** a Velero BackupStorageLocation exists with recent backups for "prod-eu-1"
**When** I initiate disaster recovery for "prod-eu-1"
**Then** a replacement cluster "prod-eu-1-dr" is provisioned via UC-01

### Scenario: Restore workloads from Velero backup

**Given** a replacement cluster "prod-eu-1-dr" is provisioned and Available
**And** Velero is deployed on the replacement cluster via ManifestWork
**When** I create a Velero Restore targeting the latest backup
**Then** namespaces, workloads, and PersistentVolumeClaims are restored
**And** the restore status is reported back to the hub

### Scenario: GitOps re-deploys configuration to the replacement cluster

**Given** the replacement cluster "prod-eu-1-dr" inherits the same labels and ClusterSet as the original
**When** ArgoCD ApplicationSet evaluates the Placement
**Then** all applications that targeted "prod-eu-1" now deploy to "prod-eu-1-dr"
**And** the configuration drift is zero

### Scenario: Verify recovery completeness

**Given** workloads are restored and GitOps has converged
**When** I run a recovery report for "prod-eu-1-dr"
**Then** the report shows restored namespaces, running pods, and bound PVCs
**And** any gaps between the original and restored state are flagged

### Relationship to other UCs

UC-36 handles hub-level backup/restore (OADP). UC-42 covers workload-level disaster recovery on spoke clusters: provisioning a replacement (UC-01), restoring data (Velero), and re-deploying config (UC-32 GitOps). UC-08 handles decommissioning the failed original.

### ACM types

`cluster.open-cluster-management.io/v1.ManagedCluster` (health detection, label inheritance)
`api/work/v1.ManifestWork` (Velero operator deployment)
`apps.open-cluster-management.io/v1beta1.Placement` (GitOps cluster selection)
`velero.io/v1.Restore`, `velero.io/v1.BackupStorageLocation` (backup/restore)

### Package

New `internal/recovery/` package: DetectFailure, ProvisionReplacement, DeployVelero, RestoreWorkloads, VerifyRecovery

---

## UC-43: Cluster relocation (planned workload migration)

**Feature**: Migrate workloads from one cluster to another in a planned, controlled manner

As a platform operator
I want to relocate workloads between clusters without downtime
So that I can move tenants across regions for cost, compliance, or capacity reasons

### Scenario: Initiate a planned relocation

**Given** a source cluster "prod-us-1" with running workloads
**And** a target cluster "prod-eu-2" is provisioned and Available
**When** I initiate a relocation from "prod-us-1" to "prod-eu-2" for namespace "tenant-alpha"
**Then** a relocation plan is created listing workloads, volumes, and network endpoints

### Scenario: Replicate workloads to the target cluster

**Given** a relocation plan exists for "tenant-alpha"
**When** I execute the replication phase
**Then** Velero backs up namespace "tenant-alpha" from "prod-us-1"
**And** Velero restores namespace "tenant-alpha" on "prod-eu-2"
**And** GitOps ApplicationSet detects the new cluster and deploys configuration

### Scenario: Validate target before cutover

**Given** workloads are replicated to "prod-eu-2"
**When** I run a pre-cutover validation
**Then** pod readiness, PVC bindings, and endpoint health are verified on the target
**And** the report confirms the target is ready for traffic

### Scenario: Cutover and decommission source namespace

**Given** pre-cutover validation passes
**When** I execute the cutover
**Then** the source namespace is cordoned (new workloads blocked)
**And** Placement labels are updated so policies and applications target "prod-eu-2"
**And** the source namespace on "prod-us-1" is drained and deleted
**And** a final report confirms zero workloads remain on the source

### Scenario: Cross-cluster networking during migration (Submariner)

**Given** both clusters are connected via Submariner (UC-26)
**When** the migration is in the replication phase
**Then** workloads on the source can communicate with workloads on the target
**And** the migration can proceed without a hard cutover for stateless services

### Relationship to other UCs

UC-43 composes UC-01 (provision target), UC-26 (Submariner connectivity), UC-32 (GitOps redeploy), UC-42 (Velero backup/restore mechanics), and UC-08 (decommission source namespace). The difference from UC-42: this is a planned, zero-downtime migration, not a reactive recovery.

### ACM types

`cluster.open-cluster-management.io/v1.ManagedCluster` (label updates for Placement)
`api/work/v1.ManifestWork` (Velero deployment, cordon/drain)
`submariner.io/v1alpha1.SubmarinerConfig` (cross-cluster networking)
`apps.open-cluster-management.io/v1beta1.Placement` (traffic cutover)
`velero.io/v1.Backup`, `velero.io/v1.Restore` (data replication)

### Package

New `internal/migration/` package: CreatePlan, Replicate, ValidateTarget, Cutover, DecommissionSource

---

## UC-34: Placement tolerations and taints

**Feature**: Taint-based workload exclusion and toleration-based scheduling for managed clusters

As a platform operator
I want to taint clusters to prevent general workloads from scheduling on them
So that I can reserve specialised clusters (e.g. GPU nodes) for approved workloads only

### Scenario: Taint a GPU cluster and schedule tolerant workloads

**Given** a managed cluster gpu-spoke1 with GPU resources
**When** I add a taint `gpu-workloads=reserved:NoSchedule`
**Then** standard Placements skip this cluster
**And** only Placements with a matching toleration schedule workloads to it

### ACM types

`cluster.open-cluster-management.io/v1.ManagedCluster` — spec.taints[]
`cluster.open-cluster-management.io/v1beta1.Placement` — spec.tolerations[]

### Package

`internal/fleet/taints.go` — AddTaint, RemoveTaint, ListTaints, CreateTolerantPlacement

---

## UC-53: Cluster templating via ClusterDeploymentCustomization

**Feature**: Reusable cluster profiles using Hive ClusterDeploymentCustomization CRDs

As a platform operator
I want to define standard cluster profiles (small, medium, large)
So that teams can provision clusters with consistent configurations

### Scenario: Create and apply a cluster template

**Given** a template "small-profile" with installConfigPatches setting 2 worker replicas
**When** I apply the template to a ClusterDeployment
**Then** the cluster is provisioned with the patched configuration

### ACM types

`hive.openshift.io/v1.ClusterDeploymentCustomization` — spec.installConfigPatches[]

### Package

`internal/provisioning/templating.go` — CreateTemplate, GetTemplate, ListTemplates, RemoveTemplate, ApplyTemplate

---

## UC-54: Global ManagedClusterSet

**Feature**: A single ClusterSet that automatically includes all managed clusters

As a platform operator
I want a global ClusterSet that matches every cluster without manual assignment
So that cross-team Placements can target the entire fleet

### Scenario: Enable global ClusterSet and bind to namespaces

**Given** a fleet with 20 managed clusters across multiple teams
**When** I enable the global ClusterSet
**Then** a ManagedClusterSet with selectorType=LabelSelector and empty matchLabels is created
**And** it matches all clusters automatically
**And** I can bind it to team namespaces for Placement use

### ACM types

`cluster.open-cluster-management.io/v1beta2.ManagedClusterSet` — spec.clusterSelector.selectorType=LabelSelector
`cluster.open-cluster-management.io/v1beta2.ManagedClusterSetBinding`

### Package

`internal/clusterset/global.go` — EnableGlobal, BindGlobal, UnbindGlobal, GlobalStatus

---

## UC-55: ManifestWork ordering (ordinal-based sequencing)

**Feature**: Dependency-aware manifest deployment using ordinal sequencing

As a platform operator
I want to control the order in which manifests are applied on a spoke cluster
So that namespaces are created before deployments, and CRDs before CRs

### Scenario: Deploy an application stack with ordered manifests

**Given** three manifests: Namespace (ordinal 0), ConfigMap (ordinal 1), Deployment (ordinal 2)
**When** I create an ordered ManifestWork
**Then** the manifests are applied in ordinal order
**And** each resource uses ServerSideApply update strategy

### ACM types

`work.open-cluster-management.io/v1.ManifestWork` — spec.manifestConfigs[].resourceIdentifier.ordinal

### Package

`internal/rollout/ordering.go` + `ordering_builder.go` — CreateOrderedWork, GetOrderedWork, ListOrderedWork, RemoveOrderedWork

---

## UC-56: Cluster discovery via OpenShift Cluster Manager

**Feature**: Discover unmanaged OpenShift clusters registered with Red Hat via OCM API

As a platform operator
I want to discover all OpenShift clusters not yet managed by ACM
So that I can identify and import unmanaged clusters into the fleet

### Scenario: Enable discovery and import a cluster

**Given** an OCM API token from `ocm login --use-device-code` + `ocm token`
**When** I enable discovery in a namespace
**Then** DiscoveredCluster CRDs appear for OpenShift clusters registered with Red Hat
**And** I can import them into ACM as ManagedClusters

### What OCM discovers

- OpenShift clusters: UPI, IPI, ROSA, ARO — any OpenShift registered with Red Hat
- Does NOT discover: EKS, GKE, AKS, or vanilla Kubernetes (use UC-57 for those)

### ACM types

`discovery.open-cluster-management.io/v1.DiscoveryConfig` — spec.credential, spec.filters.lastActive
`discovery.open-cluster-management.io/v1.DiscoveredCluster` — read-only, auto-populated by discovery operator

### Package

`internal/discovery/discovery.go` + `builder.go` — EnableDiscovery, DisableDiscovery, ListDiscovered, ImportDiscovered, DiscoveryStatus

---

## UC-57: Cloud-native cluster discovery (AWS, IBM Cloud, kubeconfig)

**Feature**: Discover unmanaged Kubernetes clusters from cloud provider APIs and kubeconfig files

As a platform operator
I want to discover EKS, ROSA, IKS, ROKS clusters and kubeconfig-accessible clusters not managed by ACM
So that I can identify and import them into the fleet regardless of their type

### Scenario: Scan AWS and IBM Cloud for unmanaged clusters

**Given** configured aws/rosa/ibmcloud CLI credentials
**When** I scan cloud providers
**Then** I see all clusters with a flag indicating whether each is already managed by ACM

### Scenario: Scan kubeconfig directory

**Given** a directory containing kubeconfig files for various clusters
**When** I scan the directory
**Then** I see all clusters from kubeconfig files cross-referenced against ACM ManagedClusters
**And** unmanaged clusters can be auto-imported

### Supported providers

- AWS: EKS (via `aws eks`) and ROSA (via `rosa describe cluster`)
- IBM Cloud: IKS and ROKS (via `ibmcloud ks cluster ls`)
- Kubeconfig: any cluster type (OpenShift, EKS, GKE, AKS, vanilla Kubernetes)

### Package

`internal/discovery/cloud_discovery.go` — ScanClusters, AutoImport, ScanKubeconfigs, AutoImportKubeconfig

---

## Summary

| UC  |  What it validates  |  ACM Go module  |  ComputeRequest field |
|-----|---------------------|-----------------|----------------------|
| UC-01  |  Programmatic cluster provisioning  |  `hive/apis/hive/v1`  |  spec.platform, spec.capacity |
| UC-02  |  Image registry policy enforcement  |  `governance-policy-propagator/api/v1`  |  spec.security.allowedRegistries |
| UC-03  |  Tenant isolation via ManifestWork  |  `api/work/v1`  |  spec.tenants, spec.features |
| UC-04  |  Fleet health queries + cross-cluster search  |  `api/cluster/v1` + Search API  |  status.conditions, status.workloads |
| UC-05  |  Programmatic lifecycle management  |  `hive/apis/hive/v1`  |  spec.lifecycle, spec.utilization |
| UC-06  |  Cluster monitoring (capacity + real-time usage via Thanos)  |  `ManagedClusterInfo` + `MultiClusterObservability`  |  status.capacity, scheduling |
| UC-07  |  External cluster import  |  `api/cluster/v1` + `agent/v1`  |  spec.import |
| UC-08  |  Legacy cluster decommissioning  |  `api/cluster/v1` + `ManagedClusterInfo`  |  spec.lifecycle.decommission |
| UC-09  |  Cluster upgrades (Day-2)  |  `ClusterCurator` + `ManagedClusterInfo`  |  spec.version.desired |
| UC-10  |  Cluster scaling (workers)  |  `hive/v1.MachinePool` + `ManagedClusterInfo`  |  spec.capacity.workers |
| UC-11  |  Cost tracking / chargeback  |  `MCO/Thanos` + `ManagedClusterInfo`  |  status.cost |
| UC-12  |  Identity Provider management  |  `api/work/v1` + `policy/v1`  |  spec.identityProvider |
| UC-13  |  Registry mirror for restricted clusters (ROKS, air-gapped)  |  `imageregistry.open-cluster-management.io/v1alpha1`  |  spec.import.mirrorRegistry |
| UC-14  |  Automatic cluster reclamation (idle/expired)  |  `ConfigurationPolicy` + ACM Search + `ManagedCluster` labels  |  spec.lifecycle.ttlHours |
| UC-15  |  Resource quota gates via governance policy  |  `ConfigurationPolicy` + `ManifestWork` + ACM Search  |  spec.quota.maxWorkers |
| UC-16  |  Unique IdP per cluster (security hardening)  |  `ManifestWork` + `Policy` (SSO enforcement)  |  spec.security.uniqueCredentials |
| UC-17  |  Cost center attribution + budget alerting  |  `ManagedCluster` labels + `MCO/Thanos` + `Policy`  |  spec.billing.costCenter |
| UC-18  |  GPU sharing stack deployment fleet-wide (Kueue + Kyverno)  |  `ManifestWork` + `ConfigurationPolicy`  |  spec.gpu.sharingEnabled |
| UC-19  |  Multi-cluster GPU workload routing via Placement  |  `Placement` + `PlacementDecision` + `ManagedCluster` labels  |  spec.gpu.type |
| UC-20  |  AI platform operator version fleet segregation  |  `Placement` + `ConfigurationPolicy` + `ManifestWork`  |  spec.gpu.aiPlatformVersion |
| UC-21  |  Elastic GPU capacity (auto-provision on saturation)  |  `MCO/Thanos` + `ConfigurationPolicy` + `ClusterDeployment`  |  spec.gpu.elasticCapacity |
| UC-22  |  ClusterSet management (team isolation)  |  `ManagedClusterSet` + `ManagedClusterSetBinding`  |  spec.team |
| UC-23  |  Multi-architecture cluster matrix (QA)  |  `ClusterDeployment` + `ManagedCluster` labels + `Placement`  |  spec.matrix |
| UC-24  |  Per-team compliance reporting  |  `Policy` + `PlacementBinding` scoped to ClusterSet  |  spec.compliance.clusterSet |
| UC-25  |  ClusterPool + ClusterClaim (pre-warmed)  |  `hive/v1.ClusterPool` + `hive/v1.ClusterClaim`  |  spec.pool |
| UC-26  |  Multi-cluster networking (Submariner)  |  `ManagedClusterAddOn` + `SubmarinerConfig`  |  spec.submariner |
| UC-27  |  Operator version pinning (OperatorPolicy)  |  `policy/v1beta1.OperatorPolicy`  |  spec.operatorPolicy |
| UC-28  |  Certificate expiry detection fleet-wide  |  `policy/v1.CertificatePolicy`  |  spec.certPolicy |
| UC-29  |  Security baseline via Gatekeeper/OPA  |  `ManifestWork` + `ConfigurationPolicy` + OPA constraints  |  spec.securityBaseline |
| UC-30  |  Policy automation (Ansible auto-remediation)  |  `policy/v1beta1.PolicyAutomation`  |  spec.policyAutomation |
| UC-31  |  SCAP scanning via Compliance Operator  |  `ManifestWork` + `ConfigurationPolicy` + ComplianceScan  |  spec.compliance |
| UC-32  |  GitOps fleet deployment via ApplicationSet  |  `ApplicationSet` + `Placement` + `PlacementDecision`  |  spec.gitops |
| UC-33  |  ManifestWorkReplicaSet progressive rollout  |  `work/v1alpha1.ManifestWorkReplicaSet`  |  spec.rollout |
| UC-35  |  Credential-free spoke access (ManagedServiceAccount)  |  `ManagedServiceAccount` + `cluster-proxy` addon  |  spec.access |
| UC-36  |  Hub backup and restore  |  `BackupSchedule` + `Restore` + OADP  |  spec.backup |
| UC-37  |  Worker node flavor change (rolling replacement)  |  `hive/v1.MachinePool` platform patch  |  spec.workers.type |
| UC-38  |  HyperShift (HostedCluster) provisioning  |  `hypershift.io/v1beta1.HostedCluster` + `NodePool`  |  spec.type=hypershift |
| UC-39  |  Cloud-provider native scaling for imported clusters  |  `ManagedCluster` + `ManagedClusterInfo` + cloud API  |  spec.scaling.cloudProvider |
| UC-40  |  CAPI provisioning for vanilla Kubernetes  |  `cluster.x-k8s.io/v1beta1.Cluster` + `MachineDeployment`  |  spec.type=capi |
| UC-41  |  CAPI hibernate via scale-to-zero  |  `cluster.x-k8s.io/v1beta1.MachineDeployment`  |  spec.lifecycle.capiHibernate |
| UC-42  |  Workload disaster recovery (Velero + GitOps)  |  `ManifestWork` + `Placement` + `velero.io/v1`  |  spec.recovery |
| UC-43  |  Cluster relocation (planned migration)  |  `ManifestWork` + `Placement` + `SubmarinerConfig` + `velero.io/v1`  |  spec.migration |
| UC-44  |  Disconnected cluster GitOps (Argo CD Agent)  |  `argocd.argoproj.io` + `ApplicationSet` agent mode  |  spec.gitops.pullBased |
| UC-45  |  Fleet right-sizing recommendations  |  `MultiClusterObservability` + MCOA PrometheusRules  |  spec.observability.rightSizing |
| UC-46  |  Cluster Proxy (spoke service exposure)  |  `ManagedClusterAddOn` cluster-proxy + `ProxyConfig`  |  spec.access.proxy |
| UC-47  |  Add-on lifecycle management  |  `AddOnDeploymentConfig` + `ClusterManagementAddOn`  |  spec.addons |
| UC-48  |  Placement scoring (resource-based scheduling)  |  `Placement` + `AddOnPlacementScore` prioritisers  |  spec.placement.scoring |
| UC-49  |  PolicySet compliance profiles  |  `PolicySet` + `PlacementBinding` + grouped policies  |  spec.compliance.profile |
| UC-50  |  ClusterCurator day-2 automation hooks  |  `ClusterCurator` pre/post hooks  |  spec.lifecycle.curator |
| UC-51  |  Virtual Machine management via KubeVirt  |  `kubevirt.io/v1.VirtualMachine`  |  spec.vm |
| UC-52  |  Multi-cluster observability and alerting  |  `MultiClusterObservability` + `ObservabilityAddon`  |  spec.observability |
| UC-34  |  Placement tolerations and taints  |  `ManagedCluster` taints + `Placement` tolerations  |  spec.placement.tolerations |
| UC-53  |  Cluster templating (ClusterDeploymentCustomization)  |  `hive/v1.ClusterDeploymentCustomization` installConfigPatches  |  spec.provisioning.template |
| UC-54  |  Global ManagedClusterSet  |  `ManagedClusterSet` selectorType=LabelSelector  |  spec.clusterset.global |
| UC-55  |  ManifestWork ordering (ordinal-based sequencing)  |  `ManifestWork` resourceIdentifier ordinals  |  spec.rollout.ordering |
| UC-56  |  Cluster discovery via OCM  |  `discovery.open-cluster-management.io/v1.DiscoveryConfig`  |  spec.discovery.ocm |
| UC-57  |  Cloud-native cluster discovery  |  AWS EKS/ROSA + IBM Cloud IKS/ROKS CLIs + kubeconfig scanning  |  spec.discovery.cloud |

## Go Dependencies (for the lab repo)

```go
// go.mod — key dependencies
// Note: using k8s.io/client-go/dynamic only — no typed ACM/Hive imports
require (
    k8s.io/client-go             v0.30.x   // dynamic client, kubeconfig loading
    k8s.io/apimachinery          v0.30.x   // unstructured, GVR, watch
    github.com/spf13/cobra       v1.10.x   // CLI framework
    github.com/cucumber/godog    v0.15.x   // Gherkin test runner
    github.com/mark3labs/mcp-go  v0.28.x   // MCP server (stdio)
    github.com/joho/godotenv     v1.5.x    // .env loading
)
```
