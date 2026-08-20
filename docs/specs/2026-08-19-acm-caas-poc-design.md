# ACM CaaS PoC — Design Spec

> **AIPCC-29876** — Epic: AIPCC-29817
>
> A Go project to validate ACM capabilities for CaaS via executable
> Gherkin use cases, with an MCP server for interactive testing from Claude.
> The code is structured to graduate into the ComputeRequest controller.

---

## Goals

1. Validate 7 ACM capabilities against a live hub using the Go API
2. Each capability is specified as a Gherkin `.feature` file and executed via godog
3. An MCP server (stdio) exposes the same operations as tools for Claude
4. Code is reusable as the foundation for the ComputeRequest controller
5. No hardcoded credentials or configuration — everything via `.env` + `internal/config/`

## Non-Goals

- Building the ComputeRequest CRD or controller (that's the next project)
- Typed ACM Go clients (we use `k8s.io/client-go/dynamic` for now)
- Multi-provider support (IBM Cloud first, others later)
- Production-grade error handling or retry logic

---

## Project Structure

```
~/work/lab/acm/
  cmd/acmlab/
    main.go                  # Cobra CLI entry point
  internal/
    client/                  # Dynamic client wrapper + GVR constants
      client.go
      gvr.go
    config/                  # Configuration from env vars
      config.go
      config_test.go
    provisioning/            # UC-01: ClusterDeployment lifecycle
      provisioning.go
      builder.go             # Builds unstructured resources
    policy/                  # UC-02: Policy distribution
      policy.go
      builder.go
    workload/                # UC-03: ManifestWork deployment
      workload.go
      builder.go
    fleet/                   # UC-04: ManagedCluster queries
      fleet.go
    lifecycle/               # UC-05: Hibernate/resume
      lifecycle.go
    monitoring/              # UC-06: Cluster resource monitoring
      monitoring.go
      monitoring_test.go
    importing/               # UC-07: External cluster import
      importing.go
      builder.go
      importing_test.go
    mcp/                     # MCP server (stdio, tool registration)
      server.go
      tools_crud.go          # Low-level CRUD tools
      tools_uc.go            # High-level UC tools
  manifests/                 # Reference YAMLs (documentation, not rendered)
    clusterdeployment/
      clusterdeployment.yaml
      install-config.yaml
    policy/
      image-registry-restriction.yaml
      placement-binding.yaml
    manifestwork/
      tenant-isolation.yaml
    importing/
      managed-cluster.yaml
      klusterlet-addon-config.yaml
      auto-import-secret.yaml
  features/                  # Gherkin .feature files
    provisioning.feature
    policy.feature
    workload.feature
    fleet.feature
    lifecycle.feature
    monitoring.feature
    importing.feature
  integration/               # godog step definitions
    provisioning_test.go
    policy_test.go
    workload_test.go
    fleet_test.go
    lifecycle_test.go
    monitoring_test.go
    importing_test.go
    suite_test.go
  docs/
    acm-use-cases-caas-poc.md
    specs/
      2026-08-19-acm-caas-poc-design.md  # This file
  .env.example               # Template for required env vars
  .gitignore                 # Includes .env, style.md
  CLAUDE.md                  # Project conventions + style reference
  go.mod
```

---

## Configuration

### Environment Variables (`.env`)

All configuration comes from environment variables. No hardcoded values in Go code.

```bash
# .env.example

# Cluster connection
KUBECONFIG=~/.kube/config
ACM_HUB_CONTEXT=                     # kubectl context for the hub (empty = current)

# Cloud provider credentials (IBM Cloud)
IBMCLOUD_API_KEY=                     # IBM Cloud API key
IBMCLOUD_REGION=us-south              # Default region

# Cluster defaults
ACM_BASE_DOMAIN=infraops1.ibm.rh-ods.com
ACM_CLUSTER_IMAGE_SET=img4.22.9-multi-appsub
ACM_DEFAULT_WORKER_TYPE=bx2-4x16
ACM_DEFAULT_MASTER_TYPE=bx2-8x32
ACM_DEFAULT_WORKER_REPLICAS=2
ACM_DEFAULT_MASTER_REPLICAS=3

# Timeouts
ACM_PROVISION_TIMEOUT=45m
ACM_OPERATION_TIMEOUT=5m

# MCP
ACM_MCP_LOG_LEVEL=info
```

`.env` is in `.gitignore`. `.env.example` is committed.

### `internal/config/config.go`

```go
type Config struct {
    Kubeconfig          string
    HubContext          string

    IBMCloudAPIKey      string
    IBMCloudRegion      string

    BaseDomain          string
    ClusterImageSet     string
    DefaultWorkerType   string
    DefaultMasterType   string
    DefaultWorkerReplicas int
    DefaultMasterReplicas int

    ProvisionTimeout    time.Duration
    OperationTimeout    time.Duration

    MCPLogLevel         string
}
```

`LoadFromEnv()` reads from environment. No flags for credentials — env only.
CLI flags can override non-sensitive values (region, timeout, image set).

---

## Client Layer (`internal/client/`)

### `client.go`

Thin wrapper over `k8s.io/client-go/dynamic`. No ACM-specific logic.

```go
type Client struct { Dynamic dynamic.Interface }

func NewFromKubeconfig(path string) (*Client, error)
func NewFromDefault() (*Client, error)
func NewFromContext(kubeconfig, context string) (*Client, error)

func (c *Client) Create(ctx, gvr, namespace string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
func (c *Client) Get(ctx, gvr, namespace, name string) (*unstructured.Unstructured, error)
func (c *Client) List(ctx, gvr, namespace string, labelSelector string) (*unstructured.UnstructuredList, error)
func (c *Client) Delete(ctx, gvr, namespace, name string) error
func (c *Client) Patch(ctx, gvr, namespace, name string, pt types.PatchType, data []byte) (*unstructured.Unstructured, error)
func (c *Client) Watch(ctx, gvr, namespace string, opts metav1.ListOptions) (watch.Interface, error)
```

Namespace-aware: empty namespace = cluster-scoped.
Unit testable with `k8s.io/client-go/dynamic/fake`.

### `gvr.go`

GVR constants shared across UC packages:

```go
var (
    GVRClusterDeployment = schema.GroupVersionResource{Group: "hive.openshift.io", Version: "v1", Resource: "clusterdeployments"}
    GVRClusterImageSet   = schema.GroupVersionResource{Group: "hive.openshift.io", Version: "v1", Resource: "clusterimagesets"}
    GVRManagedCluster    = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters"}
    GVRManifestWork      = schema.GroupVersionResource{Group: "work.open-cluster-management.io", Version: "v1", Resource: "manifestworks"}
    GVRPolicy            = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "policies"}
    GVRPlacementBinding  = schema.GroupVersionResource{Group: "policy.open-cluster-management.io", Version: "v1", Resource: "placementbindings"}
    GVRPlacementRule     = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "placementrules"}
    GVRPlacement         = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1beta1", Resource: "placements"}
    GVRManagedClusterInfo = schema.GroupVersionResource{Group: "internal.open-cluster-management.io", Version: "v1beta1", Resource: "managedclusterinfos"}
    GVRKlusterletAddonConfig = schema.GroupVersionResource{Group: "agent.open-cluster-management.io", Version: "v1", Resource: "klusterletaddonconfigs"}
)
```

---

## UC Packages (`internal/<uc>/`)

### Pattern

Each UC package exposes:
- A struct with a `*client.Client` dependency
- Methods for the operations in its use case
- A `builder.go` that constructs `*unstructured.Unstructured` objects from parameters

Resource building uses Go maps, not YAML templates. The `manifests/` directory contains reference YAMLs for humans (what the Go code creates, documented as YAML).

### UC-01: Provisioning (`internal/provisioning/`)

```go
type Provisioner struct {
    client *client.Client
    cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Provisioner

type CreateOpts struct {
    Name              string
    Namespace         string
    Region            string
    BaseDomain        string
    Cloud             string
    CredentialsSecret string
    ImageSet          string
    WorkerType        string
    WorkerReplicas    int
    MasterType        string
    MasterReplicas    int
}

func (p *Provisioner) EnsureNamespace(ctx, name string) error
func (p *Provisioner) EnsureCredentials(ctx, namespace, apiKey string) error
func (p *Provisioner) CreateCluster(ctx, opts CreateOpts) error
func (p *Provisioner) DeleteCluster(ctx, namespace, name string) error
func (p *Provisioner) GetStatus(ctx, namespace, name string) (*ClusterStatus, error)
func (p *Provisioner) WaitForProvisioned(ctx, namespace, name string, timeout time.Duration) error

type ClusterStatus struct {
    Phase       string            // Pending, Provisioning, Provisioned, Failed
    Conditions  []Condition
    PowerState  string
}
```

`builder.go` builds the ClusterDeployment and install-config as unstructured maps.

### UC-02: Policy (`internal/policy/`) — Image Registry Restriction

```go
type Manager struct {
    client *client.Client
    cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Manager

type PolicyOpts struct {
    Name              string
    Namespace         string
    TargetLabels      map[string]string
    RemediationAction string   // enforce | inform
    AllowedRegistries []string // e.g. ["registry.redhat.io", "quay.io/myorg", "us.icr.io/myns"]
}

func (m *Manager) CreatePolicy(ctx, opts PolicyOpts) error
func (m *Manager) DeletePolicy(ctx, namespace, name string) error
func (m *Manager) GetCompliance(ctx, namespace, name string) (*ComplianceStatus, error)
```

`builder.go` builds a `ConfigurationPolicy` with `AllowedContainerImagesPolicy`
that restricts container images to the approved registries list.

### UC-03: Workload (`internal/workload/`) — Tenant RBAC Isolation

```go
type Deployer struct {
    client *client.Client
    cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Deployer

type TenantOpts struct {
    TeamName    string // e.g. "team-alpha"
    Group       string // OIDC/LDAP group, e.g. "cn=team-alpha,ou=groups,dc=company"
    CPUQuota    string // e.g. "8"
    MemoryQuota string // e.g. "16Gi"
}

type ManifestWorkOpts struct {
    Name       string
    Namespace  string             // spoke namespace on hub
    Manifests  []map[string]interface{}
}

func BuildTenantIsolationManifests(opts TenantOpts) []map[string]interface{}

func (d *Deployer) Deploy(ctx, opts ManifestWorkOpts) error
func (d *Deployer) Undeploy(ctx, namespace, name string) error
func (d *Deployer) GetStatus(ctx, namespace, name string) (*WorkStatus, error)
func (d *Deployer) WaitForApplied(ctx, namespace, name string, timeout time.Duration) error
```

`builder.go` builds tenant isolation manifests: Namespace, RoleBinding (edit role
to team group), NetworkPolicy (deny cross-namespace ingress), ResourceQuota
(CPU/memory limits). The `manifests/manifestwork/tenant-isolation.yaml` shows the
reference YAML for documentation.

### UC-04: Fleet (`internal/fleet/`)

```go
type Inspector struct {
    client *client.Client
}

func New(c *client.Client) *Inspector

type ClusterInfo struct {
    Name       string
    Labels     map[string]string
    Available  bool
    Joined     bool
    Accepted   bool
    Version    string
    Conditions []Condition
}

func (i *Inspector) ListClusters(ctx) ([]ClusterInfo, error)
func (i *Inspector) GetCluster(ctx, name string) (*ClusterInfo, error)
func (i *Inspector) WatchClusters(ctx) (<-chan ClusterEvent, error)
```

### UC-05: Lifecycle (`internal/lifecycle/`)

**Important:** Hibernate/resume relies on Hive's `ClusterDeployment.spec.powerState`,
which only exists for ACM-provisioned clusters. Imported/registered clusters (UC-07)
do not have a Hive-managed ClusterDeployment, so lifecycle operations are not available.
The ComputeRequest controller must check for ClusterDeployment existence before
attempting lifecycle ops and set `LifecycleNotSupported` condition on imported clusters.

```go
type Manager struct {
    client *client.Client
    cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Manager

func (m *Manager) Hibernate(ctx, namespace, name string) error
func (m *Manager) Resume(ctx, namespace, name string) error
func (m *Manager) GetPowerState(ctx, namespace, name string) (string, error)
func (m *Manager) WaitForPowerState(ctx, namespace, name string, target string, timeout time.Duration) error
```

### UC-06: Monitoring (`internal/monitoring/`)

Reads `ManagedClusterInfo` resources to surface node count, CPU/memory
capacity, allocatable resources, and node conditions from the hub.

```go
type Monitor struct {
    client *client.Client
    cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Monitor

type NodeInfo struct {
    Name        string
    Role        string            // master, worker
    Capacity    ResourceInfo
    Allocatable ResourceInfo
    Conditions  []Condition
}

type ResourceInfo struct {
    CPU    string
    Memory string
    Pods   string
}

type ClusterResources struct {
    Name              string
    Nodes             []NodeInfo
    TotalCPU          string
    AllocatableCPU    string
    TotalMemory       string
    AllocatableMemory string
}

func (m *Monitor) GetClusterResources(ctx, name string) (*ClusterResources, error)
func (m *Monitor) ListClusterResources(ctx) ([]ClusterResources, error)
```

### UC-07: Import (`internal/importing/`)

Imports existing clusters not provisioned by ACM. Creates `ManagedCluster`,
`KlusterletAddonConfig`, and an auto-import `Secret` with the external
cluster's kubeconfig.

```go
type Importer struct {
    client *client.Client
    cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Importer

type ImportOpts struct {
    Name       string
    Labels     map[string]string  // e.g. {"cloud": "on-prem", "region": "dc-1"}
    Kubeconfig string             // external cluster kubeconfig content
    Addons     []string           // e.g. ["policyController", "searchCollector"]
}

type ImportStatus struct {
    Joined    bool
    Available bool
    Conditions []Condition
}

func (i *Importer) ImportCluster(ctx, opts ImportOpts) error
func (i *Importer) DetachCluster(ctx, name string) error
func (i *Importer) GetImportStatus(ctx, name string) (*ImportStatus, error)
```

`builder.go` builds the `ManagedCluster`, `KlusterletAddonConfig` (with
selectable addons: applicationManager, policyController, searchCollector,
certPolicyController, iamPolicyController), and the auto-import `Secret`.

---

## MCP Server (`internal/mcp/`)

stdio transport. Registered as a Cobra subcommand: `acmlab mcp serve`.

### Low-Level Tools (CRUD)

| Tool | Description | Returns |
|------|------------|---------|
| `acm_create_cluster` | Create a ClusterDeployment | namespace, name, status |
| `acm_delete_cluster` | Delete a ClusterDeployment | confirmation |
| `acm_get_cluster_status` | Get ClusterDeployment conditions | phase, conditions, powerState |
| `acm_list_managed_clusters` | List ManagedClusters | array of ClusterInfo |
| `acm_create_policy` | Create Policy + PlacementBinding | name, compliance |
| `acm_delete_policy` | Delete a Policy | confirmation |
| `acm_create_manifestwork` | Create ManifestWork | name, applied status |
| `acm_delete_manifestwork` | Delete a ManifestWork | confirmation |
| `acm_patch_power_state` | Patch ClusterDeployment powerState | new state |
| `acm_get_cluster_resources` | Get node count, CPU/memory for a cluster | ClusterResources |
| `acm_list_cluster_resources` | Get resource summary for all clusters | array of ClusterResources |
| `acm_import_cluster` | Import an external cluster into ACM | name, import status |
| `acm_detach_cluster` | Remove an imported cluster from ACM | confirmation |
| `acm_get_import_status` | Check import progress | joined, available, conditions |

### High-Level Tools (UC flows)

| Tool | Description | Behavior |
|------|------------|----------|
| `acm_provision_spoke` | Full UC-01 | Creates ns + credentials + ClusterDeployment. Returns immediately with status=provisioning. Use `acm_get_cluster_status` to poll. |
| `acm_enforce_security` | Full UC-02 | Creates Policy + PlacementRule + PlacementBinding. Returns compliance state. |
| `acm_deploy_workload` | Full UC-03 | Creates ManifestWork. Returns immediately. Use `acm_get_cluster_status` to check. |
| `acm_fleet_status` | Full UC-04 | Returns all ManagedClusters with health summary. |
| `acm_hibernate_cluster` | UC-05 hibernate | Patches powerState. Returns immediately. |
| `acm_resume_cluster` | UC-05 resume | Patches powerState. Returns immediately. |
| `acm_cluster_resources` | Full UC-06 | Returns all clusters with node count, CPU/memory capacity, utilization ranking. |
| `acm_import_spoke` | Full UC-07 | Creates ManagedCluster + KlusterletAddonConfig + auto-import Secret. Returns immediately; poll with `acm_get_import_status`. |

**Long-running operations return immediately.** The tool response includes a status and tells Claude to use `acm_get_cluster_status` to poll. This matches the reconcile-loop pattern of the real controller.

### MCP Registration

Uses `github.com/mark3labs/mcp-go` (MIT, Go MCP SDK). Each tool is a function with JSON schema input/output defined via struct tags.

---

## Godog Integration

### Build Tags

- `//go:build integration` — tests that call the ACM API but don't wait for provisioning (quick: API acceptance, resource creation). ~5 minutes.
- `//go:build slow` — tests that wait for full provisioning/hibernation. ~60 minutes.

`go test ./...` runs neither. Explicit invocation:

```bash
# Quick integration tests (API calls, no long waits)
go test -tags integration ./integration/... -timeout 10m

# Full end-to-end (provisioning, hibernation, etc.)
go test -tags slow ./integration/... -timeout 90m
```

### `integration/suite_test.go`

```go
//go:build integration || slow

func TestFeatures(t *testing.T) {
    suite := godog.TestSuite{
        ScenarioInitializer: InitializeScenarios,
        Options: &godog.Options{
            Format:   "pretty",
            Paths:    []string{"../features"},
            TestingT: t,
        },
    }
    suite.Run()
}
```

### Step Definition Pattern

Each `_test.go` file registers its steps:

```go
//go:build integration || slow

func InitializeProvisioningScenario(ctx *godog.ScenarioContext) {
    p := &provisioningContext{}
    ctx.Given(`^the ACM hub is healthy$`, p.hubIsHealthy)
    ctx.Given(`^cloud credentials exist in namespace "([^"]*)"$`, p.credentialsExist)
    ctx.When(`^I create a ClusterDeployment "([^"]*)" in region "([^"]*)"$`, p.createCluster)
    ctx.Then(`^the ClusterDeployment status shows Provisioned = True`, p.waitProvisioned)
    ctx.Then(`^a ManagedCluster "([^"]*)" exists on the hub$`, p.managedClusterExists)
}
```

Step implementations call the `internal/<uc>/` packages — same code path as CLI and MCP.

---

## CLI (`cmd/acmlab/`)

Cobra commands mirror the UC structure:

```
acmlab
  provision create <name>    # UC-01
  provision delete <name>
  provision status <name>
  policy create <name>       # UC-02
  policy compliance <name>
  workload deploy <name>     # UC-03
  workload status <name>
  fleet status               # UC-04
  fleet list
  lifecycle hibernate <name> # UC-05
  lifecycle resume <name>
  monitor list               # UC-06
  monitor status <name>
  import cluster <name>      # UC-07
  import detach <name>
  import status <name>
  mcp serve                  # Start MCP server (stdio)
```

Global flags: `--kubeconfig`, `--context`, `--namespace`.

---

## Go Dependencies

```go
require (
    github.com/spf13/cobra        v1.10.x
    github.com/cucumber/godog     v0.15.x
    github.com/mark3labs/mcp-go   v0.28.x
    github.com/joho/godotenv      v1.5.x
    k8s.io/client-go              v0.30.x
    k8s.io/apimachinery           v0.30.x
)
```

No ACM/Hive typed dependencies. Dynamic client only.

---

## .gitignore

```
.env
style.md
*.exe
vendor/
```

---

## CLAUDE.md

References `~/claude/lab/docs/style.md` for Go conventions. Adds:

- Dynamic client over typed — no Hive/OCM module imports
- All ACM resource construction in `builder.go` via unstructured maps
- No `os/exec` for kubectl/oc — everything through Go API
- Credentials via `.env` — never hardcoded
- Integration tests require live ACM hub — use build tags
- MCP long-running tools return immediately, caller polls for result

---

## Graduation Path to ComputeRequest Controller

The UC packages become the controller's reconcile logic:

| Lab package | Controller reconciler |
|-------------|----------------------|
| `provisioning.CreateCluster()` | `Reconcile()` when ComputeRequest is created |
| `policy.CreatePolicy()` | `Reconcile()` when spec.security changes |
| `workload.Deploy()` | `Reconcile()` when spec.features/addons change |
| `fleet.GetCluster()` | Status update loop |
| `lifecycle.Hibernate()` | `Reconcile()` when idle timeout triggers |
| `monitoring.GetClusterResources()` | Capacity-aware scheduling, resource-pressure detection |
| `importing.ImportCluster()` | `Reconcile()` when ComputeRequest.spec.import is set |

The `internal/client/` dynamic wrapper gets replaced by typed clients. The interfaces stay the same.
