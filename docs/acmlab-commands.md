# acmlab Command Reference

## CLI Commands

### Fleet

#### `acmlab fleet list`

Lists all ManagedCluster resources on the hub. Supports label filtering.

Options:
- `--label-selector`, `-l`: filter clusters by label (e.g. `vendor=OpenShift`)
- `--json`: output as JSON

```
$ acmlab fleet list
NAME              AVAILABLE  JOINED  ACCEPTED  VERSION
spoke1            True       True    True      4.22.9
spoke2            True       True    True      4.22.9
local-cluster     True       True    True      4.21.29

$ acmlab fleet list -l vendor=OpenShift
NAME              AVAILABLE  JOINED  ACCEPTED  VERSION
spoke1            True       True    True      4.22.9
```

#### `acmlab fleet status <name>`

Shows detailed status for a specific cluster: labels, conditions, version. Accepts multiple names or `--from-file` for batch.

Options:
- `--from-file`: YAML file with cluster list (see Batch Operations section)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output as JSON array of results

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

Provisions a spoke cluster via Hive ClusterDeployment and auto-imports it as a ManagedCluster in ACM. For IBM Cloud, IAM credentials (Service IDs + API keys) are auto-generated via the IBM Cloud IAM API: no external `ccoctl` tooling needed. Idempotent.

Options:
- `--platform`: cloud platform: ibmcloud, aws, gcp, azure (default: from `ACM_PLATFORM` env)
- `--region`: cloud region (default: from `IBMCLOUD_REGION` env)
- `--image-set`: ClusterImageSet name (default: from `ACM_CLUSTER_IMAGE_SET` env)
- `--worker-type`: worker instance type (default: bx2-4x16)
- `--master-type`: master instance type (default: bx2-8x32)
- `--workers`: number of worker nodes (default: 2)
- `--masters`: number of master nodes (default: 3)
- `--pull-secret`: path to pull secret file (required)
- `--ssh-key`: path to SSH public key file
- `--ssh-private-key`: path to SSH private key file
- `--manifests-dir`: path to ccoctl-generated manifests (optional for IBM Cloud: auto-generated if omitted)

```
$ acmlab provision create spoke1 --pull-secret ~/pull-secret.json --region us-south
Creating cluster spoke1 in us-south...
ClusterDeployment created. Hive will now provision the cluster.
Use 'acmlab provision status' to monitor progress.
```

#### `acmlab provision create <name>` (continued)

Options (continued):
- `--base-domain`: base DNS domain for the cluster (default: from `ACM_BASE_DOMAIN` env)

#### `acmlab provision destroy <name>`

Deletes a spoke cluster by removing its ClusterDeployment and ManagedCluster. Hive deprovisions infrastructure. For IBM Cloud, auto-cleans IAM Service IDs. Returns an error if the cluster does not exist. Accepts multiple names or `--from-file`.

Options:
- `--from-file`: YAML file with cluster list (see Batch Operations section)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

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
- `--registries`: comma-separated list of allowed registries
- `--remediation`: inform or enforce (default: inform)
- `--labels`: cluster label selector (key=value,key2=value2)
- `--operator`: operator name for OperatorPolicy
- `--operator-version`: pin operator to this version
- `--operator-channel`: pin operator to this channel
- `--cert-expiry`: certificate expiry threshold in days for CertificatePolicy
- `--cert-namespaces`: namespaces to monitor (default: openshift-config,openshift-ingress)
- `--cluster-set`: scope policy to a ClusterSet (UC-24)

#### `acmlab policy status <name>`

Shows policy compliance status across targeted clusters.

#### `acmlab policy report`

Shows per-ClusterSet compliance report across all policies.

Options:
- `--json`: output as JSON

#### `acmlab policy remove <name>`

Removes a policy and its associated Placement and PlacementBinding. Guards against concurrent deletion.

#### `acmlab policy automate <name>`

Creates a PolicyAutomation linking a governance policy to an Ansible job template. When the policy becomes NonCompliant, the automation triggers the Ansible playbook.

*Source: `cmd/acmlab/policy.go`: `policyAutomateCmd()`*

Options:
- `--policy`: referenced policy name (required)
- `--tower-url`: Ansible Tower URL
- `--tower-secret`: secret name with Tower credentials (required)
- `--job-template`: Ansible job template name (required)
- `--mode`: automation mode: scan, once, disabled (default: scan)
- `--namespace`: namespace (default: open-cluster-management-policies)
- `--extra-var`: extra variables (key=value, repeatable)

```
$ acmlab policy automate cert-remediation --policy cert-expiry-policy \
    --tower-secret aap-credentials --job-template renew-cert --mode scan
Creating PolicyAutomation cert-remediation for policy cert-expiry-policy...
PolicyAutomation created. Ansible jobs will trigger on policy violations.
```

#### `acmlab policy automation-status <name>`

Shows PolicyAutomation details: referenced policy, mode, status, last run.

*Source: `cmd/acmlab/policy.go`: `policyAutomationStatusCmd()`*

Options:
- `--namespace`: namespace (default: open-cluster-management-policies)
- `--json`: output as JSON

#### `acmlab policy list-automations`

Lists all PolicyAutomations with their policy reference, mode, and status.

*Source: `cmd/acmlab/policy.go`: `policyListAutomationsCmd()`*

Options:
- `--namespace`: namespace (default: open-cluster-management-policies)
- `--json`: output as JSON

```
$ acmlab policy list-automations
NAME                      POLICY                    MODE       STATUS
cert-remediation          cert-expiry-policy        scan       active
```

#### `acmlab policy remove-automation <name>`

Removes a PolicyAutomation.

*Source: `cmd/acmlab/policy.go`: `policyRemoveAutomationCmd()`*

Options:
- `--namespace`: namespace (default: open-cluster-management-policies)

#### `acmlab policy set-automation-mode <name> <mode>`

Updates the automation mode: scan, once, disabled.

*Source: `cmd/acmlab/policy.go`: `policySetAutomationModeCmd()`*

Options:
- `--namespace`: namespace (default: open-cluster-management-policies)

```
$ acmlab policy set-automation-mode cert-remediation disabled
PolicyAutomation cert-remediation mode set to disabled
```

### ClusterSets

#### `acmlab clusterset create <name>`

Creates a ManagedClusterSet with a ManagedClusterSetBinding in the specified namespace.

Options:
- `--namespace`: team namespace for the binding (required)

#### `acmlab clusterset list`

Lists all ManagedClusterSets with member cluster counts.

Options:
- `--json`: output as JSON

#### `acmlab clusterset assign <cluster>`

Assigns a managed cluster to a ClusterSet by updating its label.

Options:
- `--to`: target ClusterSet name (required)

#### `acmlab clusterset remove <name>`

Removes a ManagedClusterSet and its binding.

Options:
- `--namespace`: namespace of the binding (required)

### Identity Providers

#### `acmlab idp configure <name>`

Configures an Identity Provider on a cluster via ManifestWork. Creates an OAuth CR and Secret in openshift-config. Supports five IdP types: GitHub, Google, htpasswd, LDAP, OIDC (Keycloak).

Options:
- `--cluster`: target cluster (required)
- `--type`: IdP type: github, google, htpasswd, ldap, oidc (required)
- `--client-id`: OAuth client ID (github, google, oidc)
- `--client-secret`: OAuth client secret (github, google, oidc)
- `--organizations`: GitHub organizations (comma-separated)
- `--issuer-url`: OIDC issuer URL (oidc)
- `--users`: htpasswd users (user1:pass1,user2:pass2)
- `--ldap-url`: LDAP server URL
- `--bind-dn`: LDAP bind DN
- `--bind-password`: LDAP bind password
- `--insecure`: LDAP insecure connection

```
$ acmlab idp configure corp-github --cluster spoke1 --type github \
    --client-id placeholder-id --client-secret placeholder-secret \
    --organizations example-org
IdP corp-github (github) configured on cluster spoke1
```

#### `acmlab idp list`

Lists Identity Providers configured on a cluster.

Options:
- `--cluster`: target cluster (required)
- `--json`: output as JSON

```
$ acmlab idp list --cluster spoke1
NAME                 TYPE       STATUS
corp-github          github     Applied
corp-ldap            ldap       Applied
keycloak             oidc       Pending
```

#### `acmlab idp rotate <name>`

Rotates credentials for an Identity Provider by updating the ManifestWork Secret.

Options:
- `--cluster`: target cluster (required)
- `--client-id`: new OAuth client ID
- `--client-secret`: new OAuth client secret
- `--users`: new htpasswd users
- `--bind-password`: new LDAP bind password

#### `acmlab idp remove <name>`

Removes an Identity Provider from a cluster.

Options:
- `--cluster`: target cluster (required)

#### `acmlab idp configure-unique`

Deploys unique emergency htpasswd credentials to a cluster. Generates a cryptographically random password, creates an htpasswd IdP named `emergency-<cluster>`, and stores a rotation timestamp.

Options:
- `--cluster`: target cluster (required)
- `--admin-user`: admin username (default: cluster-admin)

```
$ acmlab idp configure-unique --cluster spoke1 --admin-user cluster-admin
Unique IdP configured on spoke1
Admin user: cluster-admin
Password: <generated-password>
Save this password: it will not be shown again.
```

#### `acmlab idp enforce-sso`

Creates a fleet-wide compliance policy that marks clusters without an OpenID (SSO) IdP as NonCompliant.

Options:
- `--namespace`: policy namespace (default: global-set)

```
$ acmlab idp enforce-sso
SSO enforcement policy created
```

### Resource Quota

#### `acmlab policy apply-quota`

Stamps quota labels on a ManagedCluster and creates a ConfigurationPolicy to monitor compliance.

Options:
- `--cluster`: target cluster (required)
- `--max-workers`: maximum worker nodes allowed
- `--max-gpus`: maximum GPU nodes allowed

```
$ acmlab policy apply-quota --cluster spoke1 --max-workers 5 --max-gpus 1
Quota policy applied to spoke1 (max-workers=5, max-gpus=1)
```

#### `acmlab policy quota-status <cluster>`

Shows quota labels and compliance status for a cluster.

```
$ acmlab policy quota-status spoke1
Cluster:     spoke1
Max Workers: 5
Max GPUs:    1
Compliant:   true
```

### ClusterPool

#### `acmlab pool create <name>`

Creates a Hive ClusterPool for pre-warmed cluster access. Clusters are provisioned and hibernated, ready for instant claiming.

Options:
- `--size`: number of ready clusters (default: 3)
- `--platform`: cloud platform: ibmcloud, aws, gcp, azure
- `--region`: cloud region
- `--image-set`: ClusterImageSet name
- `--base-domain`: base domain for clusters
- `--namespace`: pool namespace (default: pool name)

```
$ acmlab pool create amd64-419 --size 3 --image-set img4.19-multi --platform ibmcloud --region us-south --base-domain example.com
ClusterPool amd64-419 created (size=3)
```

#### `acmlab pool list`

Lists all ClusterPools with size, ready count, and claimed count.

```
$ acmlab pool list
NAME             SIZE  READY  CLAIMED  STANDBY
amd64-419        3     2      1        0
```

#### `acmlab pool get <name>`

Shows detailed ClusterPool info.

Options:
- `--namespace`: pool namespace (default: pool name)
- `--json`: output as JSON

#### `acmlab pool delete <name>`

Deletes a ClusterPool and all its clusters.

Options:
- `--namespace`: pool namespace (default: pool name)

### ClusterClaim

#### `acmlab claim create <pool-name>`

Claims a pre-warmed cluster from a pool for instant access.

Options:
- `--name`: claim name (auto-generated if not provided)
- `--namespace`: pool namespace (default: pool name)
- `--ttl`: time-to-live for the claim (e.g., 48h)

```
$ acmlab claim create amd64-419 --name my-test --ttl 48h
ClusterClaim my-test created from pool amd64-419
```

#### `acmlab claim list`

Lists all ClusterClaims with their status and bound cluster.

Options:
- `--namespace`: filter by namespace
- `--json`: output as JSON

#### `acmlab claim release <name>`

Releases a claimed cluster back to the pool.

Options:
- `--namespace`: claim namespace (default: claim name)

### Scaling

#### `acmlab scaling get <cluster>`

Shows MachinePool info: replicas, autoscaling bounds, platform.

Options:
- `--json`: output as JSON

#### `acmlab scaling set <cluster>`

Sets a fixed worker replica count.

Options:
- `--replicas`: number of worker nodes (default: 2)
- `--json`: output as JSON

#### `acmlab scaling auto <cluster>`

Enables autoscaling with min/max bounds.

Options:
- `--min`: minimum worker nodes (default: 1)
- `--max`: maximum worker nodes (default: 5)
- `--json`: output as JSON

#### `acmlab scaling set-flavor <cluster>`

Changes worker node instance type via MachinePool platform update. Hive performs a rolling replacement: new workers are created with the new type, old workers are drained and removed.

Options:
- `--worker-type`: new worker instance type (required)

```
$ acmlab scaling set-flavor spoke1 --worker-type cx2-8x16
Cluster spoke1 worker flavor changed to cx2-8x16
```

#### `acmlab scaling list`

Lists all MachinePools across the fleet.

Options:
- `--json`: output as JSON

#### `acmlab scaling init <cluster>`

Creates a MachinePool for a Hive cluster that does not have one. Auto-detects current workers.

Options:
- `--worker-type`: worker instance type (auto-detected if omitted)
- `--replicas`: worker count (auto-detected if omitted)
- `--json`: output as JSON

### Tenants

#### `acmlab tenant deploy <name>`

Deploys tenant isolation resources (Namespace, RoleBinding, NetworkPolicy, ResourceQuota) via ManifestWork to a target cluster.

Options:
- `--cluster`: target cluster name
- `--team`: team/group name for RBAC
- `--cpu`: ResourceQuota CPU limit
- `--memory`: ResourceQuota memory limit

#### `acmlab tenant status <name>`

Shows ManifestWork applied status on the target cluster.

#### `acmlab tenant list`

Lists tenant deployments.

Options:
- `--cluster`: filter by cluster

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

Hibernates a Hive-provisioned cluster by setting `spec.powerState` to `Hibernating`. Idempotent: if already hibernating, does nothing.

Options:
- `--namespace`, `-n`: cluster namespace (defaults to cluster name)
- `--wait`: wait for hibernation to complete
- `--timeout`: timeout for wait operation (default: 10m)
- `--from-file`: YAML file with cluster list (see Batch Operations section)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

```
$ acmlab lifecycle hibernate spoke2
Cluster spoke2/spoke2 is hibernating
```

#### `acmlab lifecycle resume <cluster-name>`

Resumes a hibernated cluster by setting `spec.powerState` to `Running`. Idempotent.

When used with `--wait`, after the cluster reaches Running state, automatically connects to the spoke cluster and approves any expired kubelet certificates. OpenShift kubelet client certs rotate every ~24h: if the cluster was hibernated during a rotation window, the certs expire and nodes cannot start pods until the CSRs are approved. This recovery step handles that automatically.

Options:
- `--namespace`, `-n`: cluster namespace (defaults to cluster name)
- `--wait`: wait for resume to complete, then recover expired certificates
- `--timeout`: timeout for wait operation (default: 15m)
- `--from-file`: YAML file with cluster list (see Batch Operations section)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

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

Shows cluster power state: both desired (spec) and actual (status). Indicates when a transition is in progress.

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
- `--namespace`, `-n`: cluster namespace (defaults to cluster name)
- `--json`: output report as JSON

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
            Registration agent stopped updating its lease: klusterlet may need restart

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
- `--kubeconfig-path`: path to spoke cluster kubeconfig for auto-import
- `--kubeconfig-context`: context name in default kubeconfig to use for auto-import (embeds file-based certs automatically)
- `--label`, `-l`: labels for the ManagedCluster (key=value, repeatable)
- `--cluster-set`: ManagedClusterSet to assign (default: "default")
- `--wait`: wait for import to complete (only with auto-import)
- `--timeout`: timeout for wait operation (default: 10m)
- `--from-file`: YAML file with cluster list (see Batch Operations section)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

```
$ acmlab import cluster import-test --kubeconfig-path /tmp/import-test.kubeconfig --label cloud=IBM --label vendor=OpenShift --wait
Cluster import-test registered for import (auto-import enabled)
Waiting for cluster to become available (timeout: 10m0s)...
Cluster successfully imported and available
```

#### `acmlab import detach <name>`

Detaches a cluster from ACM management. Does NOT destroy the underlying cluster: only removes the ACM registration. ACM cleanup controllers remove the klusterlet from the spoke. Accepts multiple names or `--from-file`.

Options:
- `--from-file`: YAML file with cluster list (see Batch Operations section)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

```
$ acmlab import detach import-test
Cluster import-test detached from ACM
```

#### `acmlab import status <name>`

Shows import status: availability, join state, creation method, and auto-import status.

Options:
- `--json`: output as JSON

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
- `--target`: target mirror registry (required, e.g., `your-registry.example.com/acm-mirror`)

```
$ acmlab registry mirror-script import-test --target your-registry.example.com/acm-mirror > mirror.sh
# Generates: skopeo copy --all docker://registry.redhat.io/... docker://your-registry.example.com/acm-mirror/...
```

#### `acmlab registry configure <cluster>`

Configures a `ManagedClusterImageRegistry` on the hub so ACM rewrites klusterlet image references before applying them to the spoke. Also creates the required `ManagedClusterSetBinding` and `Placement` (with tolerations for unavailable clusters).

Options:
- `--mirror`: mirror registry base path (e.g., `your-registry.example.com/acm-mirror`)
- `--pull-secret`: path to pull secret JSON for the mirror registry
- `--registry`: explicit source=mirror mapping (repeatable, overrides `--mirror`)
- `--from-file`: YAML file with cluster list (per-item `mirror` and `pullSecretPath` override global flags)
- `--concurrency int`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

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
- `--json`: output as JSON

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
- `--json`: output as JSON

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
- `--json`: output as JSON

```
$ acmlab upgrade list
CLUSTER              VERSION      CHANNEL          METHOD       AVAILABLE
hub-cluster          4.21.29      stable-4.21      manifestwork 4.21.30, 4.21.31
spoke2               4.22.9       stable-4.22      hive         4.22.10, 4.22.11, 4.22.12
```

#### `acmlab upgrade set-channel <cluster> <channel>`

Sets the OCP update channel for a cluster by creating a ManifestWork that patches the spoke ClusterVersion via ServerSideApply. Idempotent: updates existing ManifestWork if present.

```
$ acmlab upgrade set-channel spoke2 fast-4.22
Channel set to fast-4.22 for cluster spoke2
```

#### `acmlab upgrade start <cluster> <version>`

Triggers a cluster upgrade by creating a ManifestWork that patches the spoke ClusterVersion desiredUpdate. Idempotent: updates existing ManifestWork if present.

```
$ acmlab upgrade start spoke2 4.22.10
Upgrade to 4.22.10 started for cluster spoke2
```

#### `acmlab upgrade history <cluster>`

Shows version upgrade history for a cluster: past versions with state, start time, and completion time.

Options:
- `--json`: output as JSON

```
$ acmlab upgrade history spoke2
VERSION      STATE        STARTED                  COMPLETED
4.22.9       Completed    2026-09-01T10:00:00Z     2026-09-01T11:30:00Z
4.22.8       Completed    2026-08-15T08:00:00Z     2026-08-15T09:45:00Z
```

### Decommission

#### `acmlab decommission start <cluster>`

Starts a decommission workflow: creates a tracking ConfigMap in the cluster namespace and runs an automatic audit. The audit collects node count, CPU/memory capacity, owner, and platform from ManagedCluster and ManagedClusterInfo. Idempotent: if a workflow already exists, returns the existing state.

Options:
- `--owner`: cluster owner email (overrides label detection)
- `--deadline`: reclaim deadline ISO 8601 (default: 14 days from now)
- `--kubeconfig-path`: spoke kubeconfig for imported clusters

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
  [2026-09-14T15:10:17Z] imported: Decommission workflow started
  [2026-09-14T15:10:18Z] audited: 5 nodes, 32 CPU, 131527516Ki memory
```

#### `acmlab decommission list`

Lists all active decommission workflows across the fleet.

Options:
- `--json`: output as JSON

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

Runs a standalone audit without starting a decommission workflow. Read-only: no state change.

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

### Security Baseline

#### `acmlab security apply <level>`

Deploys Gatekeeper/OPA security constraints to a cluster via ManifestWork. Creates ConfigurationPolicy to verify Gatekeeper health.

Options:
- `--cluster`: target cluster name
- `--cluster-set`: apply to all clusters in a ClusterSet

```
$ acmlab security apply cis-level1 --cluster spoke2
Security baseline cis-level1 applied to spoke2
Constraints: K8sPSPPrivilegedContainer, K8sPSPHostNamespace, K8sRequiredResources, K8sAllowedRepos
```

#### `acmlab security status <cluster>`

Shows OPA audit results and Gatekeeper health for a cluster.

```
$ acmlab security status spoke2
Cluster:    spoke2
Level:      cis-level1
Gatekeeper: Healthy
Status:     Applied
```

#### `acmlab security list`

Lists all security baselines deployed across the fleet.

#### `acmlab security remove <cluster>`

Removes the security baseline ManifestWork from a cluster.

#### `acmlab security apply-policy`

Deploys a custom admission policy to a spoke cluster via ManifestWork. Supports both Gatekeeper (Rego) and Kyverno (YAML) engines. The engine is auto-detected from file extension (`.rego` = Gatekeeper, `.yaml`/`.yml` = Kyverno) or set explicitly with `--engine`.

*Source: `cmd/acmlab/security.go`: `securityApplyPolicyCmd()`*

Options:
- `--cluster`: target cluster name (required)
- `--policy-file`: path to policy file — `.rego` or `.yaml` (required)
- `--name`: policy name (auto-detected from file if omitted)
- `--engine`: policy engine: gatekeeper, kyverno (auto-detected from extension)
- `--match`: Kubernetes kinds to match (Gatekeeper only, default: Pod)

```
$ acmlab security apply-policy --cluster spoke1 --policy-file deny-latest.rego
Applying Gatekeeper policy from deny-latest.rego to spoke1...
Gatekeeper policy deployed via ManifestWork.

$ acmlab security apply-policy --cluster spoke1 --policy-file gpu-limits.yaml
Applying Kyverno policy from gpu-limits.yaml to spoke1...
Kyverno ClusterPolicy deployed via ManifestWork.
```

#### `acmlab security list-policies`

Lists all custom policies (Gatekeeper and Kyverno) deployed across the fleet.

*Source: `cmd/acmlab/security.go`: `securityListPoliciesCmd()`*

Options:
- `--engine`: filter by engine: gatekeeper, kyverno (default: all)
- `--json`: output as JSON

```
$ acmlab security list-policies
ENGINE       CLUSTER                   POLICY               STATUS
gatekeeper   spoke1                    deny-latest          Applied
kyverno      spoke1                    disallow-latest      Applied
kyverno      spoke2                    gpu-resource-limits  Pending
```

#### `acmlab security remove-policy <name>`

Removes a custom policy from a cluster.

*Source: `cmd/acmlab/security.go`: `securityRemovePolicyCmd()`*

Options:
- `--cluster`: target cluster name (required)
- `--engine`: policy engine: gatekeeper, kyverno (required)

```
$ acmlab security remove-policy deny_latest --cluster spoke1 --engine gatekeeper
Policy deny_latest removed from spoke1
```

### GitOps

#### `acmlab gitops create <name>`

Creates an ApplicationSet for fleet-wide GitOps deployment via Argo CD. Uses clusterDecisionResource generator with ACM Placement for cluster selection.

*Source: `cmd/acmlab/gitops.go`: `gitopsCreateCmd()`*

Options:
- `--repo`: Git repository URL (required)
- `--path`: path in repository to deploy (required)
- `--revision`: Git revision (default: main)
- `--generator`: generator type: placement, cluster (default: placement)
- `--namespace`: namespace (default: openshift-gitops)
- `--label`: label selector (key=value, repeatable)

```
$ acmlab gitops create training-stack --repo https://github.com/org/cluster-configs --path teams/training
Creating ApplicationSet training-stack...
ApplicationSet created. Argo CD will generate Application resources per matching cluster.
```

#### `acmlab gitops get <name>`

Shows ApplicationSet details: repo, path, generator type, status, generated app count.

*Source: `cmd/acmlab/gitops.go`: `gitopsGetCmd()`*

Options:
- `--namespace`: namespace (default: openshift-gitops)
- `--json`: output as JSON

#### `acmlab gitops list`

Lists all ApplicationSets with generator type, app count, and status.

*Source: `cmd/acmlab/gitops.go`: `gitopsListCmd()`*

Options:
- `--namespace`: namespace (default: openshift-gitops)
- `--json`: output as JSON

```
$ acmlab gitops list
NAME                      GENERATOR    APPS     STATUS
training-stack            placement    3        Healthy
```

#### `acmlab gitops delete <name>`

Deletes an ApplicationSet and its generated Applications.

*Source: `cmd/acmlab/gitops.go`: `gitopsDeleteCmd()`*

Options:
- `--namespace`: namespace (default: openshift-gitops)

#### `acmlab gitops sync <name>`

Triggers a sync refresh on an ApplicationSet by annotating it with a timestamp.

*Source: `cmd/acmlab/gitops.go`: `gitopsSyncCmd()`*

Options:
- `--namespace`: namespace (default: openshift-gitops)

### Hub Backup

#### `acmlab backup enable`

Enables scheduled hub backup via BackupSchedule CR. Deploys OADP/Velero integration for ACM hub state.

*Source: `cmd/acmlab/backup.go`: `backupEnableCmd()`*

Options:
- `--schedule`: cron schedule (default: every 6 hours)
- `--ttl`: backup TTL (default: 720h)
- `--storage-location`: Velero BSL name (default: default)
- `--namespace`: backup namespace

```
$ acmlab backup enable --schedule "0 */6 * * *" --ttl 720h
Enabling hub backup schedule...
Hub backup schedule enabled.
```

#### `acmlab backup disable`

Disables hub backup schedule by removing the BackupSchedule CR.

*Source: `cmd/acmlab/backup.go`: `backupDisableCmd()`*

Options:
- `--namespace`: backup namespace

#### `acmlab backup status`

Shows hub backup schedule status: enabled, schedule, phase, last backup.

*Source: `cmd/acmlab/backup.go`: `backupStatusCmd()`*

Options:
- `--namespace`: backup namespace
- `--json`: output as JSON

```
$ acmlab backup status
Enabled:          true
Schedule:         0 */6 * * *
Phase:            Enabled
Storage Location: default
Last Backup:      2026-09-17T06:00:00Z
Last Status:      Completed
```

#### `acmlab backup list`

Lists completed backup and restore operations with phase and start time.

*Source: `cmd/acmlab/backup.go`: `backupListCmd()`*

Options:
- `--namespace`: backup namespace
- `--json`: output as JSON

```
$ acmlab backup list
NAME                                PHASE           START TIME
acm-backup-2026-09-17-06-00-00     Completed       2026-09-17T06:00:00Z
acm-backup-2026-09-17-00-00-00     Completed       2026-09-17T00:00:00Z
```

#### `acmlab backup restore`

Triggers hub restore from a backup. Reconnects Hive-provisioned clusters automatically; imported clusters are flagged for re-import.

*Source: `cmd/acmlab/backup.go`: `backupRestoreCmd()`*

Options:
- `--namespace`: backup namespace
- `--backup-name`: specific backup to restore from
- `--sync-mode`: sync mode: latest, skip (default: latest)

```
$ acmlab backup restore --backup-name acm-backup-2026-09-17-06-00-00
Initiating hub restore...
Hub restore initiated. Monitor with 'acmlab backup list'.
```

### Progressive Rollout

#### `acmlab rollout create <name>`

Creates a ManifestWorkReplicaSet for fleet-wide updates with rollout strategy control.

Options:
- `--placement`: Placement name for cluster selection
- `--strategy`: rollout strategy: All, Progressive, ProgressivePerGroup (default: All)
- `--max-concurrency`: clusters updated simultaneously (default: 1)
- `--max-failures`: failure threshold, e.g. "10%" or "2" (default: 0)
- `--namespace`: namespace (default: open-cluster-management)

```
$ acmlab rollout create kueue-v12 --placement gpu-clusters --strategy progressive --max-concurrency 2 --max-failures 10%
Rollout kueue-v12 created (strategy=Progressive, maxConcurrency=2, maxFailures=10%)
```

#### `acmlab rollout get <name>`

Shows rollout progress: applied, failed, and total cluster counts.

#### `acmlab rollout list`

Lists all ManifestWorkReplicaSets with strategy and progress.

#### `acmlab rollout delete <name>`

Deletes a ManifestWorkReplicaSet.

#### `acmlab rollout update-strategy <name>`

Changes the rollout strategy on an existing ManifestWorkReplicaSet.

Options:
- `--strategy`: new strategy: All, Progressive, ProgressivePerGroup
- `--max-concurrency`: updated concurrency
- `--max-failures`: updated failure threshold
- `--namespace`: namespace

### Managed Access

#### `acmlab access enable <cluster>`

Creates ManagedServiceAccount and enables cluster-proxy addon for credential-free hub-to-spoke access with auto-rotated tokens.

Options:
- `--ttl`: token rotation interval (default: 720h)
- `--roles`: RBAC roles for the service account (default: cluster-admin)

```
$ acmlab access enable spoke2 --ttl 720h
Managed access enabled on spoke2 (token TTL: 720h)
```

#### `acmlab access disable <cluster>`

Removes ManagedServiceAccount and addons from a cluster.

#### `acmlab access status <cluster>`

Shows access status: token availability, addon health, rotation schedule.

```
$ acmlab access status spoke2
Cluster:        spoke2
Enabled:        true
Token:          available
Addon Healthy:  true
```

#### `acmlab access list`

Lists all clusters with managed access enabled.

### Agent-Mode GitOps (UC-44)

#### `acmlab gitops enable-agent <name>`

Enable agent-mode (pull-based) GitOps for disconnected or edge clusters. Creates an ApplicationSet with PullMode=true so spokes pull configurations instead of the hub pushing them.

Options:
- `--repo`: Git repository URL (required)
- `--path`: path in repository (required)
- `--revision`: Git revision (default: main)
- `--namespace`: namespace (default: openshift-gitops)
- `--cluster`: target cluster names (repeatable, required)

```
$ acmlab gitops enable-agent edge-apps --repo https://github.com/org/edge-configs --path manifests/edge --cluster edge-01 --cluster edge-02
Enabling agent-mode GitOps edge-apps for 2 cluster(s)...
Agent-mode ApplicationSet created with PullMode=true. Disconnected clusters will pull configurations.
```

#### `acmlab gitops agent-status <name>`

Show agent-mode GitOps status: repo, path, mode, and sync status.

Options:
- `--namespace`: namespace (default: openshift-gitops)
- `--json`: output as JSON

#### `acmlab gitops disable-agent <name>`

Disable agent-mode GitOps by removing the ApplicationSet.

Options:
- `--namespace`: namespace (default: openshift-gitops)

### Right-Sizing (UC-45)

#### `acmlab rightsizing enable <cluster>`

Enable right-sizing monitoring on a cluster. Deploys MCOA PrometheusRules for resource usage metrics collection.

```
$ acmlab rightsizing enable spoke1
Enabling right-sizing monitoring on spoke1...
Right-sizing monitoring enabled on spoke1
```

#### `acmlab rightsizing advise <cluster>`

Get right-sizing recommendations for workloads on a cluster. Read-only: no changes are applied.

Options:
- `--json`: output as JSON

```
$ acmlab rightsizing advise spoke1
WORKLOAD             CONTAINER       CUR CPU      REC CPU      CUR MEM      REC MEM      SAVINGS
nginx-deploy         nginx           500m         200m         512Mi        256Mi        ~50%
```

#### `acmlab rightsizing adjust <cluster>`

Apply right-sizing adjustments to workloads on a cluster. Mutates resource requests/limits based on recommendations.

Options:
- `--dry-run`: show proposed changes without applying
- `--json`: output as JSON

```
$ acmlab rightsizing adjust spoke1 --dry-run
[DRY-RUN]
WORKLOAD                  CONTAINER       ACTION     DETAIL
nginx-deploy              nginx           resize     cpu: 500m→200m, mem: 512Mi→256Mi
```

#### `acmlab rightsizing list`

List clusters with right-sizing monitoring enabled.

Options:
- `--json`: output as JSON

#### `acmlab rightsizing disable <cluster>`

Disable right-sizing monitoring on a cluster. Removes MCOA resources.

### Cluster Proxy (UC-46)

#### `acmlab access enable-proxy <cluster>`

Enable cluster proxy on a spoke cluster. Creates a ManagedClusterAddOn for reverse proxy tunnels from spoke to hub.

```
$ acmlab access enable-proxy spoke1
Cluster proxy enabled on spoke1
```

#### `acmlab access proxy-status <cluster>`

Show cluster proxy status: enabled, healthy, and endpoint.

Options:
- `--json`: output as JSON

```
$ acmlab access proxy-status spoke1
Cluster:   spoke1
Enabled:   true
Healthy:   true
Endpoint:  https://cluster-proxy-addon.open-cluster-management.svc:443
```

#### `acmlab access disable-proxy <cluster>`

Disable cluster proxy on a spoke cluster.

### Add-on Lifecycle (UC-47)

#### `acmlab addon list`

List registered ClusterManagementAddOns with display name and status.

Options:
- `--json`: output as JSON

```
$ acmlab addon list
NAME                           DISPLAY NAME                   STATUS
work-manager                   Work Manager                   Available
observability-controller       Observability Controller       Available
```

#### `acmlab addon get <name>`

Show add-on details: description, install namespace, linked configurations.

Options:
- `--json`: output as JSON

#### `acmlab addon configure <name>`

Create or update an AddOnDeploymentConfig with key-value configuration.

Options:
- `--namespace`: namespace (default: open-cluster-management)
- `--install-namespace`: agent install namespace on spoke clusters
- `--set`: configuration values (key=value, repeatable)

```
$ acmlab addon configure observability-config --set replica-count=3 --set log-level=debug
Configuring add-on observability-config...
AddOnDeploymentConfig observability-config created
```

#### `acmlab addon list-configs`

List AddOnDeploymentConfig resources.

Options:
- `--json`: output as JSON

#### `acmlab addon remove-config <name>`

Remove an AddOnDeploymentConfig.

Options:
- `--namespace`: namespace (default: open-cluster-management)

### Placement Scoring (UC-48)

#### `acmlab fleet scoring-configure <name>`

Configure placement scoring for resource-based cluster selection. Creates a Placement with ResourceAllocatableCPU/Memory prioritizers and an AddOnPlacementScore.

Options:
- `--namespace`: placement namespace (default: open-cluster-management)
- `--prioritizers`: scoring prioritizers (default: ResourceAllocatableCPU,ResourceAllocatableMemory)
- `--cluster-set`: scope to a ClusterSet
- `--labels`, `-l`: filter clusters by labels (key=value,key2=value2)

```
$ acmlab fleet scoring-configure gpu-scoring --cluster-set gpu-clusters --prioritizers ResourceAllocatableCPU,ResourceAllocatableMemory
Configuring placement scoring gpu-scoring...
Scoring placement configured. Clusters will be ranked by resource availability.
```

#### `acmlab fleet scoring-status <name>`

Show placement scoring decisions: clusters ranked by score.

Options:
- `--namespace`: placement namespace
- `--json`: output as JSON

```
$ acmlab fleet scoring-status gpu-scoring
Scoring: gpu-scoring
Namespace: open-cluster-management

CLUSTER                        SCORE      REASON
spoke1                         85         ResourceAllocatableCPU: 85
spoke2                         62         ResourceAllocatableCPU: 62
```

#### `acmlab fleet scoring-remove <name>`

Remove a scoring placement.

Options:
- `--namespace`: placement namespace

#### `acmlab fleet scoring-list`

List all scoring placements.

Options:
- `--namespace`: placement namespace
- `--json`: output as JSON

### PolicySet (UC-49)

#### `acmlab policy apply-set <name>`

Create a PolicySet compliance profile grouping multiple policies with a single PlacementBinding.

Options:
- `--namespace`, `-n`: policy namespace (default: global-set)
- `--description`: PolicySet description
- `--policies`: policy names to include (required)
- `--cluster-set`: scope to a ClusterSet

```
$ acmlab policy apply-set cis-golden-config --policies cert-expiry,image-registry,operator-pin --description "CIS golden configuration"
Creating PolicySet cis-golden-config...
PolicySet created. Grouped policies will be evaluated as a compliance profile.
```

#### `acmlab policy get-set <name>`

Show PolicySet details: description, compliance status, and member policies.

Options:
- `--namespace`, `-n`: policy namespace (default: global-set)
- `--json`: output as JSON

#### `acmlab policy list-sets`

List all PolicySets with compliance status and policy count.

Options:
- `--namespace`, `-n`: policy namespace (default: global-set)
- `--json`: output as JSON

```
$ acmlab policy list-sets
NAME                      COMPLIANT       POLICIES   DESCRIPTION
cis-golden-config         Compliant       3          CIS golden configuration
```

#### `acmlab policy remove-set <name>`

Remove a PolicySet and its placement resources.

Options:
- `--namespace`, `-n`: policy namespace (default: global-set)

### Policy Troubleshooting

#### `acmlab policy violations <name>`

Show policy violations per cluster. Reads from Policy.status.status[].clusterConditions on the hub.

Options:
- `--namespace`, `-n`: policy namespace (default: global-set)
- `--json`: output as JSON

```
$ acmlab policy violations image-registry-policy
Policy:    image-registry-policy
Compliant: NonCompliant

CLUSTER                   MESSAGE
spoke1                    Container uses disallowed registry: docker.io
```

#### `acmlab policy troubleshoot <name>`

Troubleshoot a policy by combining violations, recent events in the policy namespace, and propagation status into a single report.

Options:
- `--namespace`, `-n`: policy namespace (default: global-set)
- `--json`: output as JSON

```
$ acmlab policy troubleshoot image-registry-policy
Policy:    image-registry-policy
Namespace: global-set
Compliant: NonCompliant

Violations (1):
  - spoke1: Container uses disallowed registry: docker.io

Recent Events (2):
  [Warning] PolicyPropagation: Policy replicated to spoke1 (2026-09-17T10:00:00Z)
  [Normal] PolicyStatusSync: Policy status updated (2026-09-17T10:00:05Z)
```

### ClusterCurator (UC-50)

#### `acmlab lifecycle curator-apply <cluster>`

Apply ClusterCurator day-2 automation hooks. Pre/post hooks run Kubernetes Jobs or Ansible Jobs during lifecycle operations (upgrade, hibernate, resume).

Options:
- `--namespace`, `-n`: cluster namespace (defaults to cluster name)
- `--pre-hook`: pre-hook job name
- `--pre-hook-type`: pre-hook type: Job or AnsibleJob (default: Job)
- `--post-hook`: post-hook job name
- `--post-hook-type`: post-hook type: Job or AnsibleJob (default: Job)

```
$ acmlab lifecycle curator-apply spoke1 --pre-hook validate-health --post-hook notify-team --post-hook-type AnsibleJob
Applying ClusterCurator to spoke1...
ClusterCurator applied. Day-2 hooks will run during lifecycle operations.
```

#### `acmlab lifecycle curator-status <cluster>`

Show ClusterCurator status: pre/post hooks, execution status.

Options:
- `--namespace`, `-n`: cluster namespace
- `--json`: output as JSON

#### `acmlab lifecycle curator-remove <cluster>`

Remove ClusterCurator from a cluster.

Options:
- `--namespace`, `-n`: cluster namespace

#### `acmlab lifecycle curator-list`

List all ClusterCurators with hook configuration and status.

Options:
- `--namespace`, `-n`: filter by namespace
- `--json`: output as JSON

```
$ acmlab lifecycle curator-list
CLUSTER                   PRE-HOOK        POST-HOOK       STATUS
spoke1                    validate-health notify-team     active
spoke2                    -               cleanup-job     active
```

### Virtual Machines (UC-51)

#### `acmlab vm deploy`

Deploy a virtual machine to a managed cluster via ManifestWork wrapping a KubeVirt VirtualMachine CRD.

**Flags:**
- `--name <name>`: VM name (required)
- `--cluster <cluster>`: target cluster (required)
- `--cpu <n>`: CPU cores (default: 2)
- `--memory <size>`: memory (default: 4Gi)
- `--image <image>`: container disk image (default: RHEL 9 guest image)
- `--disk-size <size>`: disk size (default: 20Gi)

#### `acmlab vm start <name>`

Start a stopped virtual machine.

**Flags:**
- `--cluster <cluster>`: target cluster (required)

#### `acmlab vm stop <name>`

Stop a running virtual machine.

**Flags:**
- `--cluster <cluster>`: target cluster (required)

#### `acmlab vm migrate <name>`

Live-migrate a virtual machine to another node by adding a VirtualMachineInstanceMigration to the ManifestWork.

**Flags:**
- `--cluster <cluster>`: target cluster (required)

#### `acmlab vm status <name>`

Show detailed VM status including CPU, memory, image, disk, and node assignment.

**Flags:**
- `--cluster <cluster>`: target cluster (required)
- `--json`: output as JSON

#### `acmlab vm list`

List all virtual machines deployed across the fleet.

**Flags:**
- `--json`: output as JSON

#### `acmlab vm remove <name>`

Remove a virtual machine from a managed cluster.

**Flags:**
- `--cluster <cluster>`: target cluster (required)

### Observability (UC-52)

#### `acmlab observability setup`

Deploy the full observability stack (namespace, MinIO, Thanos secret, MCO).

#### `acmlab observability teardown`

Remove the observability stack and all associated resources.

#### `acmlab observability status`

Show observability stack status (NotInstalled, Pending, Progressing, Ready).

#### `acmlab observability configure-pull-secret`

Copy the pull secret from `openshift-config` namespace to the observability namespace. Required for image access.

#### `acmlab observability configure-storage`

Configure ObjectBucketClaim for production Thanos storage (replaces MinIO for production use).

**Flags:**
- `--storage-class <class>`: storage class for the OBC
- `--bucket <name>`: bucket name prefix

#### `acmlab observability deploy-rules`

Deploy custom Prometheus recording and alerting rules via ConfigMap.

**Flags:**
- `--rules-file <path>`: path to YAML file with Prometheus rules (required)

#### `acmlab observability remove-rules`

Remove custom Prometheus rules.

#### `acmlab observability deploy-dashboard`

Deploy a custom Grafana dashboard via ConfigMap with the `grafana-custom-dashboard=true` label.

**Flags:**
- `--name <name>`: dashboard name (required)
- `--dashboard-file <path>`: path to Grafana dashboard JSON file (required)

#### `acmlab observability remove-dashboard <name>`

Remove a custom Grafana dashboard.

#### `acmlab observability configure-metrics`

Configure custom metrics allowlist for collection from managed clusters.

**Flags:**
- `--metric <name>`: metric name to allowlist (repeatable)

#### `acmlab observability addon-health`

Show observability addon health status across all managed clusters.

**Flags:**
- `--json`: output as JSON

#### `acmlab observability configure-retention`

Configure Thanos retention settings on the MCO.

**Flags:**
- `--retention <duration>`: local retention duration (e.g. 24h)
- `--block-duration <duration>`: TSDB block duration (e.g. 2h)
- `--delete-delay <duration>`: deletion delay for blocks (e.g. 48h)

### Placement Tolerations (UC-34)

#### `acmlab fleet add-taint <cluster>`

Add a taint to a managed cluster. Taints prevent workloads from scheduling unless the Placement tolerates them.

Options:
- `--key`: taint key (required)
- `--value`: taint value
- `--effect`: taint effect — NoSchedule, PreferNoSchedule, NoExecute (default: NoSchedule)

```
$ acmlab fleet add-taint gpu-spoke1 --key gpu-workloads --value reserved --effect NoSchedule
Taint gpu-workloads=reserved:NoSchedule added to gpu-spoke1
```

#### `acmlab fleet remove-taint <cluster>`

Remove a taint from a managed cluster.

Options:
- `--key`: taint key (required)

#### `acmlab fleet list-taints <cluster>`

List all taints on a managed cluster.

Options:
- `--json`: output as JSON

```
$ acmlab fleet list-taints gpu-spoke1
KEY                            VALUE                EFFECT
gpu-workloads                  reserved             NoSchedule
```

#### `acmlab fleet create-tolerant-placement <name>`

Create a Placement that tolerates specific taints, allowing scheduling on tainted clusters.

Options:
- `--tolerate`: tolerations in key=value format (required, repeatable)
- `--namespace`: namespace for the Placement
- `--cluster-set`: ClusterSets to scope the Placement (repeatable)

```
$ acmlab fleet create-tolerant-placement ml-pipeline --tolerate gpu-workloads=reserved --namespace default --cluster-set gpu-set
Tolerant placement ml-pipeline created
```

### Cluster Templating (UC-53)

#### `acmlab provision template-create <name>`

Create a ClusterDeploymentCustomization with installConfigPatches for standardised cluster profiles.

Options:
- `--patch`: JSON patch entries (required, repeatable)

```
$ acmlab provision template-create small-profile --patch '{"op":"replace","path":"/compute/0/replicas","value":"2"}'
Template small-profile created
```

#### `acmlab provision template-get <name>`

Get a cluster template's details including patches.

#### `acmlab provision template-list`

List all cluster templates.

Options:
- `--json`: output as JSON

#### `acmlab provision template-remove <name>`

Remove a cluster template.

#### `acmlab provision template-apply <cluster>`

Apply a template to a ClusterDeployment.

Options:
- `--template`: template name (required)

### Global ManagedClusterSet (UC-54)

#### `acmlab clusterset global-enable`

Create a global ManagedClusterSet with selectorType=LabelSelector and empty matchLabels, matching all clusters. Labels clusters with acmlab.redhat.com/global=true.

```
$ acmlab clusterset global-enable
Global ClusterSet "global" enabled (matches all clusters)
```

#### `acmlab clusterset global-bind <namespace>`

Bind the global ClusterSet to a namespace, enabling Placements in that namespace to use it.

#### `acmlab clusterset global-unbind <namespace>`

Remove the global ClusterSet binding from a namespace.

#### `acmlab clusterset global-status`

Show the global ClusterSet status: bound namespaces and matched cluster count.

Options:
- `--json`: output as JSON

### ManifestWork Ordering (UC-55)

#### `acmlab workorder create-ordered <name>`

Create a ManifestWork with ordinal-based manifest sequencing. Lower ordinal values are applied first, enabling dependency-aware deployment (e.g. namespace before deployment).

Options:
- `--cluster`: target cluster (required)
- `--manifests`: path to ordered manifests JSON file (required)

The JSON file contains an array of objects with `ordinal` (int) and `object` (Kubernetes manifest) fields.

```
$ acmlab workorder create-ordered app-stack --cluster spoke1 --manifests /tmp/ordered-manifests.json
Ordered ManifestWork app-stack created on spoke1 with 3 manifests
```

#### `acmlab workorder get-ordered <name>`

Get status of an ordered ManifestWork.

Options:
- `--cluster`: target cluster (required)

#### `acmlab workorder list-ordered`

List ordered ManifestWorks on a cluster.

Options:
- `--cluster`: target cluster (required)
- `--json`: output as JSON

#### `acmlab workorder remove-ordered <name>`

Remove an ordered ManifestWork from a cluster.

Options:
- `--cluster`: target cluster (required)

### Cluster Discovery — OCM (UC-56)

#### `acmlab discovery enable`

Enable cluster discovery by creating a DiscoveryConfig (discovery.open-cluster-management.io/v1) and a Secret with the OCM API token. Discovers OpenShift clusters registered with Red Hat.

Options:
- `--namespace`: namespace for discovery resources (default: open-cluster-management)
- `--token`: OpenShift Cluster Manager API token (required — obtain via `ocm login --use-device-code` then `ocm token`)
- `--last-active`: discover clusters active within N days (default: 7)
- `--versions`: filter by OpenShift versions (e.g. 4.14,4.15)

```
$ acmlab discovery enable --namespace open-cluster-management --token $OCM_TOKEN
Enabling cluster discovery in open-cluster-management...
Discovery enabled. Clusters will appear as DiscoveredCluster resources.
```

#### `acmlab discovery disable`

Disable cluster discovery and remove the DiscoveryConfig and Secret.

Options:
- `--namespace`: namespace for discovery resources (default: open-cluster-management)

#### `acmlab discovery list`

List discovered clusters (DiscoveredCluster CRDs populated by the discovery operator).

Options:
- `--namespace`: namespace for discovery resources (default: open-cluster-management)
- `--json`: output as JSON

```
$ acmlab discovery list --namespace open-cluster-management
NAME                      CLOUD      VERSION      REGION          STATUS
my-rosa-cluster           AWS        4.15.2       us-east-1       Active
```

#### `acmlab discovery import <name>`

Import a discovered cluster into ACM by creating a ManagedCluster and KlusterletAddonConfig.

Options:
- `--namespace`: namespace where cluster was discovered (default: open-cluster-management)

#### `acmlab discovery status`

Show discovery configuration status: namespace, credential, last active filter, cluster count.

Options:
- `--namespace`: namespace for discovery resources (default: open-cluster-management)
- `--json`: output as JSON

### Cloud-Native Discovery (UC-57)

#### `acmlab discovery scan`

Scan cloud providers for clusters not yet managed by ACM. Supports AWS (EKS + ROSA via aws/rosa CLIs) and IBM Cloud (IKS + ROKS via ibmcloud CLI). Cross-references results with ACM ManagedClusters.

Options:
- `--provider`: cloud provider — aws, ibmcloud (default: scan all)
- `--region`: filter by region
- `--json`: output as JSON

```
$ acmlab discovery scan --provider aws --region us-east-1
NAME                      PROVIDER   TYPE     REGION          VERSION      STATUS     MANAGED
my-eks-cluster            aws        EKS      us-east-1       1.29         ACTIVE
my-rosa-cluster           aws        ROSA     us-east-1       4.15.2       ready      yes
```

#### `acmlab discovery auto-import <name>`

Import a cloud-discovered cluster into ACM.

Options:
- `--provider`: cloud provider — aws, ibmcloud (required)

#### `acmlab discovery scan-kubeconfigs`

Scan a directory of kubeconfig files to discover clusters not managed by ACM. Works for any cluster type (OpenShift, EKS, GKE, AKS, vanilla Kubernetes). Matches by name and server URL.

Options:
- `--dir`: directory to scan (default: ~/.kube/)
- `--json`: output as JSON

```
$ acmlab discovery scan-kubeconfigs --dir ~/.kube/
NAME                      SERVER                                    FILE                     MANAGED
my-cluster                https://api.my-cluster.example.com:6443   ~/.kube/my-cluster.yaml  no
```

#### `acmlab discovery auto-import-kubeconfig`

Import a cluster from a kubeconfig file into ACM.

Options:
- `--name`: cluster name (required)
- `--kubeconfig`: path to kubeconfig file (required)

### Batch Operations

All commands that take a single cluster name also accept multiple names and a `--from-file` flag.

**Flags available on all commands:**
- `--from-file <file.yaml>`: YAML file with cluster list
- `--concurrency <n>`: max parallel operations (default 5, max 20)
- `--json`: output results as JSON array

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
| `acm_scaling_get` | UC-10 | MachinePool info: replicas, autoscaling, platform |
| `acm_scaling_set` | UC-10 | Set fixed worker replica count |
| `acm_scaling_auto` | UC-10 | Enable autoscaling with min/max bounds |
| `acm_scaling_set_flavor` | UC-37 | Change worker node instance type via MachinePool |
| `acm_scaling_list` | UC-10 | List all MachinePools across the fleet |
| `acm_upgrade_status` | UC-09 | Upgrade status: version, channel, available updates, method |
| `acm_upgrade_list` | UC-09 | List clusters with available OCP upgrades |
| `acm_upgrade_set_channel` | UC-09 | Set OCP update channel via ManifestWork |
| `acm_upgrade_start` | UC-09 | Start OCP version upgrade via ManifestWork |
| `acm_upgrade_history` | UC-09 | Version upgrade history with state and timestamps |
| `acm_configure_idp` | UC-12 | Configure Identity Provider on a cluster via ManifestWork |
| `acm_remove_idp` | UC-12 | Remove Identity Provider from a cluster |
| `acm_list_idps` | UC-12 | List Identity Providers configured on a cluster |
| `acm_rotate_idp` | UC-12 | Rotate credentials for an Identity Provider |
| `acm_configure_unique_idp` | UC-16 | Deploy unique emergency credentials to a cluster |
| `acm_enforce_sso` | UC-16 | Create fleet-wide SSO compliance policy |
| `acm_apply_quota_policy` | UC-15 | Apply resource quota enforcement policy to a cluster |
| `acm_quota_status` | UC-15 | Check quota compliance for a cluster |
| `acm_create_pool` | UC-25 | Create a Hive ClusterPool for pre-warmed clusters |
| `acm_list_pools` | UC-25 | List all ClusterPools with size and status |
| `acm_get_pool` | UC-25 | Get detailed ClusterPool info |
| `acm_delete_pool` | UC-25 | Delete a ClusterPool |
| `acm_create_claim` | UC-25 | Claim a cluster from a pool for instant access |
| `acm_release_claim` | UC-25 | Release a claimed cluster back to the pool |
| `acm_list_claims` | UC-25 | List all ClusterClaims |
| `acm_apply_security_baseline` | UC-29 | Deploy Gatekeeper CIS constraints to a cluster |
| `acm_security_status` | UC-29 | OPA audit results and Gatekeeper health |
| `acm_list_security_baselines` | UC-29 | List all security baselines across the fleet |
| `acm_remove_security_baseline` | UC-29 | Remove security baseline from a cluster |
| `acm_create_rollout` | UC-33 | Create ManifestWorkReplicaSet with rollout strategy |
| `acm_get_rollout` | UC-33 | Get rollout progress and status |
| `acm_list_rollouts` | UC-33 | List all ManifestWorkReplicaSets |
| `acm_delete_rollout` | UC-33 | Delete a ManifestWorkReplicaSet |
| `acm_update_rollout_strategy` | UC-33 | Change rollout strategy on existing MWRS |
| `acm_enable_access` | UC-35 | Enable credential-free managed access to a cluster |
| `acm_disable_access` | UC-35 | Disable managed access and remove addons |
| `acm_access_status` | UC-35 | Check access status, token health, addon state |
| `acm_list_access` | UC-35 | List clusters with managed access enabled |
| `acm_list_clustersets` | UC-22 | List all ManagedClusterSets with member clusters and counts |
| `acm_create_clusterset` | UC-22 | Create a ManagedClusterSet with binding in a team namespace |
| `acm_remove_clusterset` | UC-22 | Remove a ManagedClusterSet and its binding |
| `acm_assign_cluster_to_set` | UC-22 | Assign a managed cluster to a ClusterSet by label |
| `acm_create_automation` | UC-30 | Create PolicyAutomation linking policy to Ansible job template |
| `acm_get_automation` | UC-30 | Get PolicyAutomation status: mode, linked policy, last run |
| `acm_list_automations` | UC-30 | List all PolicyAutomations with policies, modes, status |
| `acm_delete_automation` | UC-30 | Delete a PolicyAutomation |
| `acm_set_automation_mode` | UC-30 | Set PolicyAutomation mode: scan, once, or disabled |
| `acm_create_appset` | UC-32 | Create ApplicationSet for fleet-wide GitOps deployment |
| `acm_get_appset` | UC-32 | Get ApplicationSet details: repo, path, generator, apps |
| `acm_list_appsets` | UC-32 | List all ApplicationSets with generator type and status |
| `acm_delete_appset` | UC-32 | Delete an ApplicationSet and its generated Applications |
| `acm_sync_appset` | UC-32 | Trigger sync refresh on an ApplicationSet |
| `acm_enable_backup` | UC-36 | Enable scheduled hub backup via OADP/Velero |
| `acm_disable_backup` | UC-36 | Disable hub backup schedule |
| `acm_backup_status` | UC-36 | Get hub backup schedule status and last backup |
| `acm_list_backups` | UC-36 | List completed backup/restore operations |
| `acm_restore_backup` | UC-36 | Trigger hub restore from a backup |
| `acm_add_taint` | UC-34 | Add a taint to a managed cluster |
| `acm_remove_taint` | UC-34 | Remove a taint from a managed cluster |
| `acm_list_taints` | UC-34 | List taints on a managed cluster |
| `acm_create_tolerant_placement` | UC-34 | Create a Placement with tolerations for tainted clusters |
| `acm_template_create` | UC-53 | Create a ClusterDeploymentCustomization template |
| `acm_template_get` | UC-53 | Get template details |
| `acm_template_list` | UC-53 | List all cluster templates |
| `acm_template_remove` | UC-53 | Remove a cluster template |
| `acm_template_apply` | UC-53 | Apply a template to a ClusterDeployment |
| `acm_global_enable` | UC-54 | Enable the global ManagedClusterSet |
| `acm_global_bind` | UC-54 | Bind global set to a namespace |
| `acm_global_unbind` | UC-54 | Unbind global set from a namespace |
| `acm_global_status` | UC-54 | Show global ClusterSet status |
| `acm_create_ordered_work` | UC-55 | Create ManifestWork with ordinal-based sequencing |
| `acm_get_ordered_work` | UC-55 | Get ordered ManifestWork status |
| `acm_list_ordered_work` | UC-55 | List ordered ManifestWorks on a cluster |
| `acm_remove_ordered_work` | UC-55 | Remove an ordered ManifestWork |
| `acm_discovery_enable` | UC-56 | Enable OCM-based cluster discovery |
| `acm_discovery_disable` | UC-56 | Disable cluster discovery |
| `acm_discovery_list` | UC-56 | List discovered clusters |
| `acm_discovery_import` | UC-56 | Import a discovered cluster into ACM |
| `acm_discovery_status` | UC-56 | Show discovery configuration status |
| `acm_discovery_scan` | UC-57 | Scan cloud providers for unmanaged clusters |
| `acm_discovery_auto_import` | UC-57 | Import a cloud-discovered cluster |
| `acm_discovery_scan_kubeconfigs` | UC-57 | Scan kubeconfig files for unmanaged clusters |
| `acm_discovery_auto_import_kubeconfig` | UC-57 | Import a cluster from kubeconfig |
