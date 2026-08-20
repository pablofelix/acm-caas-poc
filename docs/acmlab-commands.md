# acmlab Command Reference

## CLI Commands

### `acmlab fleet list`

Lists all ManagedCluster resources on the hub.

```
$ acmlab fleet list
NAME              AVAILABLE  JOINED  ACCEPTED  VERSION
infraops1         True       True    True      4.17.8
local-cluster     False      True    True      4.17.4
```

### `acmlab fleet status <name>`

Shows detailed status for a specific cluster: labels, conditions, version.

```
$ acmlab fleet status infraops1
Name:       infraops1
Available:  true
Joined:     true
Accepted:   true
Version:    4.17.8
Labels:
  cloud=IBM
  vendor=OpenShift
Conditions:
  ManagedClusterConditionAvailable: True
  ManagedClusterJoined: True
```

### `acmlab mcp serve`

Starts the MCP server on stdio. Registered as `acmlab` in Claude Code's MCP config.

### `acmlab provision create <name>` (planned)

Provisions a spoke cluster via ClusterDeployment + install-config Secret.

Options:
- `--region` — cloud region (default: from config)
- `--instance-type` — worker instance type (default: from config)
- `--workers` — number of workers (default: from config)
- `--image-set` — ClusterImageSet name (default: from config)

### `acmlab provision delete <name>` (planned)

Deletes a spoke cluster by removing its ClusterDeployment.

### `acmlab provision status <name>` (planned)

Shows provisioning progress: ClusterDeployment conditions, install log reference.

### `acmlab policy create <name>` (planned)

Creates an image registry restriction policy with PlacementRule and PlacementBinding.

Options:
- `--registries` — comma-separated list of allowed registries
- `--target-labels` — label selector for target clusters

### `acmlab policy delete <name>` (planned)

Removes a policy and its associated PlacementRule and PlacementBinding.

### `acmlab policy compliance <name>` (planned)

Shows policy compliance status across targeted clusters.

### `acmlab workload deploy <name>` (planned)

Deploys tenant isolation resources (Namespace, RoleBinding, NetworkPolicy, ResourceQuota) via ManifestWork to a target cluster.

Options:
- `--cluster` — target cluster name
- `--tenant` — tenant name
- `--cpu-limit` — ResourceQuota CPU limit
- `--memory-limit` — ResourceQuota memory limit

### `acmlab workload undeploy <name>` (planned)

Removes the ManifestWork from a target cluster.

### `acmlab workload status <name>` (planned)

Shows ManifestWork applied status on the target cluster.

### `acmlab lifecycle hibernate <name>` (planned)

Sets `spec.powerState: Hibernating` on a ClusterDeployment. Hive-provisioned clusters only.

### `acmlab lifecycle resume <name>` (planned)

Sets `spec.powerState: Running` on a ClusterDeployment. Hive-provisioned clusters only.

### `acmlab lifecycle power-state <name>` (planned)

Shows current power state of a ClusterDeployment.

### `acmlab monitor list` (planned)

Lists cluster resource summaries from ManagedClusterInfo (nodes, CPU, memory).

### `acmlab monitor status <name>` (planned)

Shows detailed resource info for a specific cluster from ManagedClusterInfo.

### `acmlab import cluster <name>` (planned)

Imports an external cluster by creating ManagedCluster + KlusterletAddonConfig + auto-import Secret.

Options:
- `--kubeconfig` — path to the external cluster's kubeconfig
- `--labels` — labels to apply to the ManagedCluster

### `acmlab import detach <name>` (planned)

Detaches an imported cluster by removing its ManagedCluster resource.

### `acmlab import status <name>` (planned)

Shows import progress: ManagedCluster conditions, klusterlet status.

---

## MCP Tools

### Implemented

| Tool | Description |
|------|-------------|
| `acm_fleet_status` | Fleet summary: total clusters, healthy count, degraded list |
| `acm_list_managed_clusters` | Lists all ManagedClusters with availability, version, labels |
| `acm_get_managed_cluster` | Detailed info for one cluster: conditions, labels, version |
| `acm_hub_health` | Checks hub connectivity by listing ManagedCluster, ClusterDeployment, ManifestWork CRDs |

### Planned

| Tool | UC | Description |
|------|-----|-------------|
| `acm_provision_cluster` | UC-01 | Creates ClusterDeployment + install-config Secret |
| `acm_delete_cluster` | UC-01 | Deletes a ClusterDeployment |
| `acm_provision_status` | UC-01 | Polls ClusterDeployment conditions |
| `acm_create_image_registry_policy` | UC-02 | Creates Policy + PlacementRule + PlacementBinding for allowed registries |
| `acm_delete_policy` | UC-02 | Removes policy and bindings |
| `acm_policy_compliance` | UC-02 | Shows compliance status across clusters |
| `acm_deploy_tenant_isolation` | UC-03 | Creates ManifestWork with Namespace/RoleBinding/NetworkPolicy/ResourceQuota |
| `acm_undeploy_workload` | UC-03 | Removes ManifestWork |
| `acm_workload_status` | UC-03 | Shows ManifestWork applied status |
| `acm_hibernate_cluster` | UC-05 | Sets powerState to Hibernating (Hive-only) |
| `acm_resume_cluster` | UC-05 | Sets powerState to Running (Hive-only) |
| `acm_cluster_power_state` | UC-05 | Shows current power state |
| `acm_cluster_resources` | UC-06 | Node/CPU/memory from ManagedClusterInfo |
| `acm_list_cluster_resources` | UC-06 | Resource summary for all clusters |
| `acm_import_cluster` | UC-07 | Creates ManagedCluster + KlusterletAddonConfig + auto-import Secret |
| `acm_detach_cluster` | UC-07 | Removes imported cluster |
| `acm_import_status` | UC-07 | Shows import progress |
