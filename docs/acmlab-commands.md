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

Shows detailed status for a specific cluster: labels, conditions, version. Accepts multiple names or `--from-file` for batch.

Options:
- `--from-file` — YAML file with cluster list (see Batch Operations section)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output as JSON array of results

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

Deletes a spoke cluster by removing its ClusterDeployment. Hive deprovisions infrastructure. For IBM Cloud, auto-cleans IAM Service IDs. Idempotent. Accepts multiple names or `--from-file`.

Options:
- `--from-file` — YAML file with cluster list (see Batch Operations section)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

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

Creates a governance policy with Placement and PlacementBinding. Supports three policy types:

- **ConfigurationPolicy** (default): enforce object state (namespaces, registry restrictions)
- **OperatorPolicy** (UC-27): pin operator versions and channels
- **CertificatePolicy** (UC-28): detect expiring certificates

Options:
- `--registries` — comma-separated list of allowed registries
- `--remediation` — inform or enforce (default: inform)
- `--labels` — cluster label selector (key=value,key2=value2)
- `--operator` — operator name for OperatorPolicy
- `--operator-version` — pin operator to this version
- `--operator-channel` — pin operator to this channel
- `--cert-expiry` — certificate expiry threshold in days for CertificatePolicy
- `--cert-namespaces` — namespaces to monitor (default: openshift-config,openshift-ingress)
- `--cluster-set` — scope policy to a ClusterSet (UC-24)

#### `acmlab policy status <name>`

Shows policy compliance status across targeted clusters.

#### `acmlab policy report`

Shows per-ClusterSet compliance report across all policies.

Options:
- `--json` — output as JSON

#### `acmlab policy remove <name>`

Removes a policy and its associated Placement and PlacementBinding.

### ClusterSets

#### `acmlab clusterset create <name>`

Creates a ManagedClusterSet with a ManagedClusterSetBinding in the specified namespace.

Options:
- `--namespace` — team namespace for the binding (required)

#### `acmlab clusterset list`

Lists all ManagedClusterSets with member cluster counts.

Options:
- `--json` — output as JSON

#### `acmlab clusterset assign <cluster>`

Assigns a managed cluster to a ClusterSet by updating its label.

Options:
- `--to` — target ClusterSet name (required)

#### `acmlab clusterset remove <name>`

Removes a ManagedClusterSet and its binding.

Options:
- `--namespace` — namespace of the binding (required)

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

#### `acmlab monitor setup`

Deploys the observability stack (MinIO + MultiClusterObservability CR) on the hub.

#### `acmlab monitor teardown`

Removes the observability stack and all associated resources.

#### `acmlab monitor obs-status`

Shows the MultiClusterObservability CR status and spoke collection state.

### Lifecycle

#### `acmlab lifecycle hibernate <cluster-name>`

Hibernates a Hive-provisioned cluster by setting `spec.powerState` to `Hibernating`. Idempotent — if already hibernating, does nothing.

Options:
- `--namespace`, `-n` — cluster namespace (defaults to cluster name)
- `--wait` — wait for hibernation to complete
- `--timeout` — timeout for wait operation (default: 10m)
- `--from-file` — YAML file with cluster list (see Batch Operations section)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

```
$ acmlab lifecycle hibernate spoke2
Cluster spoke2/spoke2 is hibernating
```

#### `acmlab lifecycle resume <cluster-name>`

Resumes a hibernated cluster by setting `spec.powerState` to `Running`. Idempotent.

When used with `--wait`, after the cluster reaches Running state, automatically connects to the spoke cluster and approves any expired kubelet certificates. OpenShift kubelet client certs rotate every ~24h — if the cluster was hibernated during a rotation window, the certs expire and nodes cannot start pods until the CSRs are approved. This recovery step handles that automatically.

Options:
- `--namespace`, `-n` — cluster namespace (defaults to cluster name)
- `--wait` — wait for resume to complete, then recover expired certificates
- `--timeout` — timeout for wait operation (default: 15m)
- `--from-file` — YAML file with cluster list (see Batch Operations section)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

```
$ acmlab lifecycle resume spoke2 --wait
Cluster spoke2/spoke2 is resuming
Waiting for cluster to resume (timeout: 15m0s)...
Cluster successfully resumed
Checking for expired kubelet certificates...
Approved 20 expired kubelet certificate(s)
  - csr-2k2ct
  - csr-5ft9t
  ...
```

#### `acmlab lifecycle status <cluster-name>`

Shows cluster power state — both desired (spec) and actual (status). Indicates when a transition is in progress.

```
$ acmlab lifecycle status spoke2
Cluster: spoke2/spoke2
Desired State (spec):  Hibernating
Actual State (status): WaitingForMachinesToStop

Note: Power state transition in progress
```

#### `acmlab lifecycle diagnose <cluster-name>`

Runs diagnostic checks that cross-reference Hive ClusterDeployment state with ACM ManagedCluster conditions. Detects inconsistencies like a cluster that Hive reports as Running but ACM shows as unavailable (klusterlet issue). Outputs actionable suggestions when problems are found.

Options:
- `--namespace`, `-n` — cluster namespace (defaults to cluster name)
- `--json` — output report as JSON

```
$ acmlab lifecycle diagnose spoke2
Cluster: spoke2/spoke2
Platform: ibmcloud
Hive Power (spec):   Running
Hive Power (status): Running
ACM Available: Unknown
ACM Joined:    True

  [OK]      Power state consistent: Running
  [ERROR]   Hive power=Running but ACM Available=Unknown
            Registration agent stopped updating its lease — klusterlet may need restart

Suggestions:
  - Restart klusterlet agent pods: kubectl delete pods -n open-cluster-management-agent -l app=klusterlet-agent --context <spoke>
  - Check klusterlet logs: kubectl logs -n open-cluster-management-agent -l app=klusterlet-agent --tail=50 --context <spoke>

Issues detected. Review suggestions above.
```

#### `acmlab lifecycle list`

Lists all clusters that support lifecycle operations (Hive-provisioned). Imported clusters are excluded.

```
$ acmlab lifecycle list
Clusters with lifecycle support (2):
  - spoke1/spoke1
  - spoke2/spoke2
```

### Import

#### `acmlab import cluster <name>`

Imports an external cluster into ACM by creating a ManagedCluster, namespace, and KlusterletAddonConfig. If `--kubeconfig-path` is provided, creates an auto-import secret so ACM installs the klusterlet automatically.

Options:
- `--kubeconfig-path` — path to spoke cluster kubeconfig for auto-import
- `--kubeconfig-context` — context name in default kubeconfig to use for auto-import (embeds file-based certs automatically)
- `--label`, `-l` — labels for the ManagedCluster (key=value, repeatable)
- `--cluster-set` — ManagedClusterSet to assign (default: "default")
- `--wait` — wait for import to complete (only with auto-import)
- `--timeout` — timeout for wait operation (default: 10m)
- `--from-file` — YAML file with cluster list (see Batch Operations section)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

```
$ acmlab import cluster import-test --kubeconfig-path /tmp/import-test.kubeconfig --label cloud=IBM --label vendor=OpenShift --wait
Cluster import-test registered for import (auto-import enabled)
Waiting for cluster to become available (timeout: 10m0s)...
Cluster successfully imported and available
```

#### `acmlab import detach <name>`

Detaches a cluster from ACM management. Does NOT destroy the underlying cluster — only removes the ACM registration. ACM cleanup controllers remove the klusterlet from the spoke. Accepts multiple names or `--from-file`.

Options:
- `--from-file` — YAML file with cluster list (see Batch Operations section)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

```
$ acmlab import detach import-test
Cluster import-test detached from ACM
```

#### `acmlab import status <name>`

Shows import status: availability, join state, creation method, and auto-import status.

Options:
- `--json` — output as JSON

```
$ acmlab import status import-test
Cluster: import-test
Available: True
Joined: True
Created via: other
Auto-import: true
```

#### `acmlab import list`

Lists all imported (non-Hive) clusters with availability and join status.

```
$ acmlab import list
Imported clusters (1):
  - import-test  Available=True  Joined=True
```

### Registry

#### `acmlab registry list-images <cluster>`

Lists all container images required by ACM on a spoke cluster, extracted from ManifestWorks. Use this to identify which images must be mirrored for restricted-registry clusters.

```
$ acmlab registry list-images import-test
Images required by ACM on import-test (6):

  registry.redhat.io/multicluster-engine/registration-operator-rhel9@sha256:7f8a4fb1...
    ManifestWork: import-test-klusterlet

  registry.redhat.io/multicluster-engine/managedcluster-import-controller-rhel9@sha256:4d71...
    ManifestWork: import-test-klusterlet

  registry.redhat.io/multicluster-engine/cluster-proxy-rhel9@sha256:2ccb3...
    ManifestWork: addon-cluster-proxy-deploy-0

  registry.redhat.io/rhacm2/search-collector-rhel9@sha256:6255...
    ManifestWork: addon-search-collector-deploy-0
  ...
```

#### `acmlab registry mirror-script <cluster>`

Generates a bash script with `skopeo copy` commands to mirror all required images from `registry.redhat.io` to a target registry.

Options:
- `--target` — target mirror registry (required, e.g., `your-registry.example.com/acm-mirror`)

```
$ acmlab registry mirror-script import-test --target your-registry.example.com/acm-mirror > mirror.sh
# Generates: skopeo copy --all docker://registry.redhat.io/... docker://your-registry.example.com/acm-mirror/...
```

#### `acmlab registry configure <cluster>`

Configures a `ManagedClusterImageRegistry` on the hub so ACM rewrites klusterlet image references before applying them to the spoke. Also creates the required `ManagedClusterSetBinding` and `Placement` (with tolerations for unavailable clusters).

Options:
- `--mirror` — mirror registry base path (e.g., `your-registry.example.com/acm-mirror`)
- `--pull-secret` — path to pull secret JSON for the mirror registry
- `--registry` — explicit source=mirror mapping (repeatable, overrides `--mirror`)
- `--from-file` — YAML file with cluster list (per-item `mirror` and `pullSecretPath` override global flags)
- `--concurrency int` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

```
$ acmlab registry configure import-test \
    --mirror your-registry.example.com/acm-mirror \
    --pull-secret ~/pull-secret.json
Image registry mirror configured for cluster import-test
Mirror registry: your-registry.example.com/acm-mirror
```

#### `acmlab registry status <cluster>`

Shows whether a `ManagedClusterImageRegistry` is configured and lists the source→mirror mappings.

Options:
- `--json` — output as JSON

```
$ acmlab registry status import-test
Cluster: import-test
Mirror configured: true
Registry mappings:
  registry.redhat.io/multicluster-engine → your-registry.example.com/acm-mirror/multicluster-engine
  registry.redhat.io/rhacm2 → your-registry.example.com/acm-mirror/rhacm2
```

#### `acmlab registry remove <cluster>`

Removes the `ManagedClusterImageRegistry`, `Placement`, `ManagedClusterSetBinding`, and pull secret created by `configure`.

```
$ acmlab registry remove import-test
Image registry mirror removed for cluster import-test
```

### Upgrade

#### `acmlab upgrade status <cluster>`

Shows upgrade status for a cluster: current version, desired version, channel, available updates, upgrade method, and whether an upgrade is in progress.

Options:
- `--json` — output as JSON

```
$ acmlab upgrade status spoke2
Cluster:         spoke2
Type:            OCP
Upgrade method:  hive
Current version: 4.22.9
Desired version: 4.22.9
Channel:         stable-4.22
Available:       4.22.10, 4.22.11, 4.22.12
```

#### `acmlab upgrade list`

Lists clusters with available OCP upgrades. Excludes vanilla Kubernetes clusters (report-only) and clusters already at the latest version.

Options:
- `--json` — output as JSON

```
$ acmlab upgrade list
CLUSTER              VERSION      CHANNEL          METHOD       AVAILABLE
hub-cluster          4.21.29      stable-4.21      manifestwork 4.21.30, 4.21.31
spoke2               4.22.9       stable-4.22      hive         4.22.10, 4.22.11, 4.22.12
```

#### `acmlab upgrade set-channel <cluster> <channel>`

Sets the OCP update channel for a cluster by creating a ManifestWork that patches the spoke ClusterVersion via ServerSideApply. Idempotent — updates existing ManifestWork if present.

```
$ acmlab upgrade set-channel spoke2 fast-4.22
Channel set to fast-4.22 for cluster spoke2
```

#### `acmlab upgrade start <cluster> <version>`

Triggers a cluster upgrade by creating a ManifestWork that patches the spoke ClusterVersion desiredUpdate. Idempotent — updates existing ManifestWork if present.

```
$ acmlab upgrade start spoke2 4.22.10
Upgrade to 4.22.10 started for cluster spoke2
```

#### `acmlab upgrade history <cluster>`

Shows version upgrade history for a cluster — past versions with state, start time, and completion time.

Options:
- `--json` — output as JSON

```
$ acmlab upgrade history spoke2
VERSION      STATE        STARTED                  COMPLETED
4.22.9       Completed    2026-09-01T10:00:00Z     2026-09-01T11:30:00Z
4.22.8       Completed    2026-08-15T08:00:00Z     2026-08-15T09:45:00Z
```

### Decommission

#### `acmlab decommission start <cluster>`

Starts a decommission workflow: creates a tracking ConfigMap in the cluster namespace and runs an automatic audit. The audit collects node count, CPU/memory capacity, owner, and platform from ManagedCluster and ManagedClusterInfo. Idempotent — if a workflow already exists, returns the existing state.

Options:
- `--owner` — cluster owner email (overrides label detection)
- `--deadline` — reclaim deadline ISO 8601 (default: 14 days from now)
- `--kubeconfig-path` — spoke kubeconfig for imported clusters

```
$ acmlab decommission start spoke2 --owner "admin@example.com"
{
  "clusterName": "spoke2",
  "phase": "audited",
  "owner": "admin@example.com",
  "deadline": "2026-09-28T15:10:16Z",
  "audit": {
    "nodeCount": 5,
    "cpuCapacity": "32",
    "memoryCapacity": "131527516Ki",
    "platform": "IBM",
    "clusterAge": "3d"
  },
  "history": [
    {"phase": "imported", "message": "Decommission workflow started"},
    {"phase": "audited", "message": "5 nodes, 32 CPU, 131527516Ki memory"}
  ]
}
```

#### `acmlab decommission advance <cluster>`

Advances the decommission to the next phase. Each phase executes its action before transitioning:
- **audited → notified**: records notification timestamp
- **notified → backed-up**: records backup path
- **backed-up → drained**: stubs cordon + evict (PoC)
- **drained → deleted**: deletes ClusterDeployment or ManagedCluster (**DESTRUCTIVE**)
- **deleted → cleaned**: removes ManifestWorks, namespace from hub (**DESTRUCTIVE**)

```
$ acmlab decommission advance spoke2
{
  "clusterName": "spoke2",
  "phase": "notified",
  ...
}
```

#### `acmlab decommission status <cluster>`

Shows current phase, owner, deadline, audit data, and full history.

```
$ acmlab decommission status spoke2
Cluster:  spoke2
Phase:    audited
Owner:    admin@example.com
Deadline: 2026-09-28T15:10:16Z
Nodes:    5
CPU:      32
Memory:   131527516Ki
Platform: IBM

History:
  [2026-09-14T15:10:17Z] imported — Decommission workflow started
  [2026-09-14T15:10:18Z] audited — 5 nodes, 32 CPU, 131527516Ki memory
```

#### `acmlab decommission list`

Lists all active decommission workflows across the fleet.

Options:
- `--json` — output as JSON

```
$ acmlab decommission list
CLUSTER              PHASE        OWNER                          DEADLINE
spoke2               audited      admin@example.com            2026-09-28T15:10:16Z
```

#### `acmlab decommission cancel <cluster>`

Cancels a decommission workflow by deleting the tracking ConfigMap. The cluster is not affected. Safe at any phase before `deleted`.

```
$ acmlab decommission cancel spoke2
Decommission cancelled for spoke2
```

#### `acmlab decommission audit <cluster>`

Runs a standalone audit without starting a decommission workflow. Read-only — no state change.

```
$ acmlab decommission audit spoke2
{
  "nodeCount": 5,
  "cpuCapacity": "32",
  "memoryCapacity": "131527516Ki",
  "owner": "",
  "platform": "IBM",
  "clusterAge": "3d"
}
```

### Batch Operations

All commands that take a single cluster name also accept multiple names and a `--from-file` flag.

**Flags available on all commands:**
- `--from-file <file.yaml>` — YAML file with cluster list
- `--concurrency <n>` — max parallel operations (default 5, max 20)
- `--json` — output results as JSON array

**YAML file format:**
```yaml
clusters:
  - spoke1          # plain name
  - name: spoke2    # or object with per-cluster options
    kubeconfigPath: /tmp/spoke2.kubeconfig
    labels:
      cloud: IBM
```

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
| `acm_deploy_tenant` | UC-03 | Deploys tenant isolation (namespace, RBAC, network policy, quota) to a spoke via ManifestWork |
| `acm_remove_tenant` | UC-03 | Removes tenant isolation from a spoke cluster |
| `acm_list_tenants` | UC-03 | Lists tenants deployed to a spoke cluster with sync status |
| `acm_tenant_status` | UC-03 | Detailed tenant ManifestWork sync status with per-resource results |
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
| `acm_hibernate_cluster` | UC-05 | Sets powerState to Hibernating (Hive-only) |
| `acm_resume_cluster` | UC-05 | Sets powerState to Running (Hive-only) |
| `acm_lifecycle_status` | UC-05 | Shows spec vs status power state and transition flag |
| `acm_lifecycle_diagnose` | UC-05 | Cross-references Hive + ACM state, detects inconsistencies, suggests fixes |
| `acm_lifecycle_recover_certs` | UC-05 | Approves expired kubelet CSRs on spoke after resume from hibernation |
| `acm_list_lifecycle_clusters` | UC-05 | Lists all Hive-provisioned clusters |
| `acm_import_cluster` | UC-07 | Imports external cluster, optional auto-import via kubeconfig |
| `acm_detach_cluster` | UC-07 | Detaches a cluster from ACM (does not destroy it) |
| `acm_import_status` | UC-07 | Import status: availability, join state, auto-import |
| `acm_list_imported_clusters` | UC-07 | Lists all imported (non-Hive) clusters |
| `acm_registry_list_images` | UC-13 | Lists images required by ACM on a spoke, extracted from ManifestWorks |
| `acm_registry_configure_mirror` | UC-13 | Creates ManagedClusterImageRegistry + Placement + pull secret on hub |
| `acm_registry_mirror_status` | UC-13 | Checks if registry mirror is configured for a cluster |
| `acm_registry_generate_mirror_script` | UC-13 | Generates bash script with skopeo commands to mirror images |
| `acm_decommission_start` | UC-08 | Start decommission workflow (creates ConfigMap, runs audit) |
| `acm_decommission_advance` | UC-08 | Advance to next decommission phase |
| `acm_decommission_status` | UC-08 | Get current decommission state for a cluster |
| `acm_decommission_list` | UC-08 | List all active decommission workflows |
| `acm_decommission_cancel` | UC-08 | Cancel workflow (keeps cluster intact) |
| `acm_decommission_audit` | UC-08 | Standalone audit without starting decommission |
| `acm_upgrade_status` | UC-09 | Upgrade status: version, channel, available updates, method |
| `acm_upgrade_list` | UC-09 | List clusters with available OCP upgrades |
| `acm_upgrade_set_channel` | UC-09 | Set OCP update channel via ManifestWork |
| `acm_upgrade_start` | UC-09 | Start OCP version upgrade via ManifestWork |
| `acm_upgrade_history` | UC-09 | Version upgrade history with state and timestamps |
