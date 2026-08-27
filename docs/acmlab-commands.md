# acmlab Command Reference

## CLI Commands

### Fleet

#### `acmlab fleet list`

Lists all ManagedCluster resources on the hub.

```
$ acmlab fleet list
NAME              AVAILABLE  JOINED  ACCEPTED  VERSION
spoke1            True       True    True      4.22.9
spoke2            True       True    True      4.22.9
local-cluster     True       True    True      4.21.29
```

#### `acmlab fleet status <name>`

Shows detailed status for a specific cluster: labels, conditions, version.

```
$ acmlab fleet status spoke1
Name:       spoke1
Available:  true
Joined:     true
Accepted:   true
Version:    4.22.9
Labels:
  cloud=IBM
  vendor=OpenShift
Conditions:
  ManagedClusterConditionAvailable: True
  ManagedClusterJoined: True
```

### Provisioning

#### `acmlab provision create <name>`

Provisions a spoke cluster via Hive ClusterDeployment and auto-imports it as a ManagedCluster in ACM. For IBM Cloud, IAM credentials (Service IDs + API keys) are auto-generated via the IBM Cloud IAM API — no external `ccoctl` tooling needed. Idempotent.

Options:
- `--platform` — cloud platform: ibmcloud, aws, gcp, azure (default: from `ACM_PLATFORM` env)
- `--region` — cloud region (default: from `IBMCLOUD_REGION` env)
- `--image-set` — ClusterImageSet name (default: from `ACM_CLUSTER_IMAGE_SET` env)
- `--worker-type` — worker instance type (default: bx2-4x16)
- `--master-type` — master instance type (default: bx2-8x32)
- `--workers` — number of worker nodes (default: 2)
- `--masters` — number of master nodes (default: 3)
- `--pull-secret` — path to pull secret file (required)
- `--ssh-key` — path to SSH public key file
- `--ssh-private-key` — path to SSH private key file
- `--manifests-dir` — path to ccoctl-generated manifests (optional for IBM Cloud — auto-generated if omitted)

```
$ acmlab provision create spoke1 --pull-secret ~/pull-secret.json --region us-south
Creating cluster spoke1 in us-south...
ClusterDeployment created. Hive will now provision the cluster.
Use 'acmlab provision status' to monitor progress.
```

#### `acmlab provision destroy <name>`

Deletes a spoke cluster by removing its ClusterDeployment. Hive deprovisions infrastructure. For IBM Cloud, auto-cleans IAM Service IDs. Idempotent.

#### `acmlab provision status <name>`

Shows ClusterDeployment provisioning status with conditions, failure info.

```
$ acmlab provision status spoke1
Cluster:      spoke1
BaseDomain:   example.com
Region:       us-south
ImageSet:     img4.22.9-multi-appsub
Installed:    true
Provisioned:  true
```

#### `acmlab provision list`

Lists all clusters provisioned via acmlab with status.

```
$ acmlab provision list
NAME                 DOMAIN        REGION       INSTALLED  IMAGE SET
spoke1               example.com   us-south     true       img4.22.9-multi-appsub
spoke2               example.com   us-south     true       img4.22.9-multi-appsub
```

#### `acmlab provision image-sets`

Lists available ClusterImageSets for provisioning.

### Policies

#### `acmlab policy list`

Lists governance policies on the hub.

#### `acmlab policy apply <name>`

Creates or updates an image registry restriction policy with PlacementRule and PlacementBinding.

Options:
- `--registries` — comma-separated list of allowed registries
- `--remediation` — inform or enforce (default: inform)

#### `acmlab policy status <name>`

Shows policy compliance status across targeted clusters.

#### `acmlab policy remove <name>`

Removes a policy and its associated PlacementRule and PlacementBinding.

### Tenants

#### `acmlab tenant deploy <name>`

Deploys tenant isolation resources (Namespace, RoleBinding, NetworkPolicy, ResourceQuota) via ManifestWork to a target cluster.

Options:
- `--cluster` — target cluster name
- `--team` — team/group name for RBAC
- `--cpu` — ResourceQuota CPU limit
- `--memory` — ResourceQuota memory limit

#### `acmlab tenant status <name>`

Shows ManifestWork applied status on the target cluster.

#### `acmlab tenant list`

Lists tenant deployments.

Options:
- `--cluster` — filter by cluster

#### `acmlab tenant remove <name>`

Removes the ManifestWork from a target cluster.

### Monitoring

#### `acmlab monitor list`

Lists cluster resource summaries from ManagedClusterInfo (nodes, CPU, memory).

#### `acmlab monitor status <name>`

Shows detailed resource info for a specific cluster.

### MCP Server

#### `acmlab mcp serve`

Starts the MCP server on stdio. Register as `acmlab` in Claude Code's MCP config.

---

## MCP Tools

| Tool | UC | Description |
|------|-----|-------------|
| `acm_fleet_status` | UC-04 | Fleet summary: total clusters, healthy count, degraded list |
| `acm_list_managed_clusters` | UC-04 | Lists all ManagedClusters with availability, version, labels |
| `acm_get_managed_cluster` | UC-04 | Detailed info for one cluster: conditions, labels, version |
| `acm_hub_health` | UC-04 | Checks hub connectivity by listing CRDs |
| `acm_list_cluster_resources` | UC-06 | Resource summary for all clusters |
| `acm_cluster_resources` | UC-06 | Detailed node/CPU/memory for a specific cluster |
| `acm_list_policies` | UC-02 | Lists governance policies |
| `acm_get_policy` | UC-02 | Gets policy details and compliance |
| `acm_apply_policy` | UC-02 | Creates/updates image registry policy |
| `acm_remove_policy` | UC-02 | Removes a policy |
| `acm_set_policy_remediation` | UC-02 | Changes policy remediation mode |
| `acm_provision_create` | UC-01 | Creates ClusterDeployment + ManagedCluster (auto-generates IBM Cloud IAM creds) |
| `acm_provision_destroy` | UC-01 | Deletes ClusterDeployment, cleans up IAM |
| `acm_provision_status` | UC-01 | ClusterDeployment provisioning status |
| `acm_provision_list` | UC-01 | Lists clusters provisioned via acmlab |
| `acm_list_image_sets` | UC-01 | Lists available ClusterImageSets |

### Planned

| Tool | UC | Description |
|------|-----|-------------|
| `acm_hibernate_cluster` | UC-05 | Sets powerState to Hibernating (Hive-only) |
| `acm_resume_cluster` | UC-05 | Sets powerState to Running (Hive-only) |
| `acm_cluster_power_state` | UC-05 | Shows current power state |
| `acm_import_cluster` | UC-07 | Imports external cluster via kubeconfig |
| `acm_detach_cluster` | UC-07 | Detaches an imported cluster |
