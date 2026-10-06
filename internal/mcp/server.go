package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/clusterset"
	"github.com/pablofelix/acm-caas-poc/internal/config"
	"github.com/pablofelix/acm-caas-poc/internal/idp"
	"github.com/pablofelix/acm-caas-poc/internal/decommission"
	"github.com/pablofelix/acm-caas-poc/internal/fleet"
	"github.com/pablofelix/acm-caas-poc/internal/importing"
	"github.com/pablofelix/acm-caas-poc/internal/lifecycle"
	"github.com/pablofelix/acm-caas-poc/internal/monitoring"
	"github.com/pablofelix/acm-caas-poc/internal/policy"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
	"github.com/pablofelix/acm-caas-poc/internal/registry"
	"github.com/pablofelix/acm-caas-poc/internal/scaling"
	"github.com/pablofelix/acm-caas-poc/internal/tenant"
	"github.com/pablofelix/acm-caas-poc/internal/access"
	"github.com/pablofelix/acm-caas-poc/internal/automation"
	"github.com/pablofelix/acm-caas-poc/internal/backup"
	"github.com/pablofelix/acm-caas-poc/internal/gitops"
	"github.com/pablofelix/acm-caas-poc/internal/pool"
	"github.com/pablofelix/acm-caas-poc/internal/rollout"
	"github.com/pablofelix/acm-caas-poc/internal/security"
	"github.com/pablofelix/acm-caas-poc/internal/upgrade"
	"github.com/pablofelix/acm-caas-poc/internal/virtualization"
)

func NewServer(c *client.Client, cfg config.Config, log *slog.Logger) *server.MCPServer {
	s := server.NewMCPServer(
		"acmlab",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	fleetInsp := fleet.New(c, cfg, log)
	registerFleetTools(s, fleetInsp)
	registerHealthTool(s, c)

	mon := monitoring.New(c, cfg, log)
	registerMonitoringTools(s, mon)

	pol := policy.New(c, cfg, log)
	registerPolicyTools(s, pol)

	ten := tenant.New(c, cfg, log)
	registerTenantTools(s, ten)

	prov := provisioning.New(c, cfg, log)
	registerProvisioningTools(s, prov, cfg)

	lc := lifecycle.New(c, cfg, log)
	registerLifecycleTools(s, lc)

	imp := importing.New(c, cfg, log)
	registerImportTools(s, imp)

	reg := registry.New(c, cfg, log)
	registerRegistryTools(s, reg)

	sc := scaling.New(c, cfg, log)
	registerScalingTools(s, sc)

	decomm := decommission.New(c, cfg, log)
	registerDecommissionTools(s, decomm)

	upg := upgrade.New(c, cfg, log)
	registerUpgradeTools(s, upg)

	cs := clusterset.New(c, cfg, log)
	registerClusterSetTools(s, cs)

	idpMgr := idp.New(c, cfg, log)
	registerIdPTools(s, idpMgr)

	poolMgr := pool.New(c, cfg, log)
	registerPoolTools(s, poolMgr)

	secMgr := security.New(c, cfg, log)
	registerSecurityTools(s, secMgr)

	rolloutMgr := rollout.New(c, cfg, log)
	registerRolloutTools(s, rolloutMgr)

	accessMgr := access.New(c, cfg, log)
	registerAccessTools(s, accessMgr)

	backupMgr := backup.New(c, cfg, log)
	registerBackupTools(s, backupMgr)

	autoMgr := automation.New(c, cfg, log)
	registerAutomationTools(s, autoMgr)

	gitopsMgr := gitops.New(c, cfg, log)
	registerGitOpsTools(s, gitopsMgr)

	vmMgr := virtualization.New(c, cfg, log)
	registerVirtualizationTools(s, vmMgr)

	return s
}

func registerFleetTools(s *server.MCPServer, fi *fleet.Inspector) {
	s.AddTool(
		mcp.NewTool("acm_fleet_status",
			mcp.WithDescription("Get complete fleet status — all managed clusters with health, labels, version, and conditions. Returns a summary with total/healthy/degraded counts."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			clusters, err := fi.ListClusters(ctx, "")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			healthy := 0
			for _, c := range clusters {
				if c.Available {
					healthy++
				}
			}
			summary := map[string]interface{}{
				"total":    len(clusters),
				"healthy":  healthy,
				"degraded": len(clusters) - healthy,
				"clusters": clusters,
			}
			data, _ := json.MarshalIndent(summary, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_list_managed_clusters",
			mcp.WithDescription("List all ManagedCluster resources on the ACM hub with status, labels, and conditions."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			clusters, err := fi.ListClusters(ctx, "")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(clusters, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_get_managed_cluster",
			mcp.WithDescription("Get detailed info for a specific ManagedCluster including labels, conditions, version, and health."),
			mcp.WithString("name", mcp.Required(), mcp.Description("ManagedCluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			info, err := fi.GetCluster(ctx, name)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerMonitoringTools(s *server.MCPServer, mon *monitoring.Monitor) {
	s.AddTool(
		mcp.NewTool("acm_list_cluster_resources",
			mcp.WithDescription("List resource summaries for all clusters — node count, CPU capacity, OCP version, and channel."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			results, err := mon.ListClusterResources(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(results, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_cluster_resources",
			mcp.WithDescription("Get detailed resource info for a specific cluster — per-node CPU, memory, instance type, region, zone, and readiness."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			cr, err := mon.GetClusterResources(ctx, name)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(cr, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerPolicyTools(s *server.MCPServer, pol *policy.Manager) {
	s.AddTool(
		mcp.NewTool("acm_list_policies",
			mcp.WithDescription("List governance policies with compliance status."),
			mcp.WithString("namespace", mcp.Description("Policy namespace (default: global-set)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ns, _ := req.GetArguments()["namespace"].(string)
			policies, err := pol.List(ctx, ns)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(policies, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_get_policy",
			mcp.WithDescription("Get detailed policy info with per-cluster compliance status."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Policy name")),
			mcp.WithString("namespace", mcp.Description("Policy namespace (default: global-set)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			ns, _ := req.GetArguments()["namespace"].(string)
			info, err := pol.Get(ctx, name, ns)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_apply_policy",
			mcp.WithDescription("Create a governance policy with placement targeting clusters by label. Supports ConfigurationPolicy (default), OperatorPolicy (--operator), and CertificatePolicy (--cert-expiry). Idempotent."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Policy name")),
			mcp.WithString("namespace", mcp.Description("Policy namespace (default: global-set)")),
			mcp.WithString("remediation", mcp.Description("inform or enforce (default: inform)")),
			mcp.WithString("registries", mcp.Description("Comma-separated list of allowed container registries")),
			mcp.WithString("operator", mcp.Description("Operator name for OperatorPolicy")),
			mcp.WithString("operator_version", mcp.Description("Pin operator to this version")),
			mcp.WithString("operator_channel", mcp.Description("Pin operator to this channel")),
			mcp.WithNumber("cert_expiry_days", mcp.Description("Certificate expiry threshold in days for CertificatePolicy")),
			mcp.WithString("cert_namespaces", mcp.Description("Comma-separated namespaces to monitor for cert expiry")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			ns, _ := req.GetArguments()["namespace"].(string)
			remediation, _ := req.GetArguments()["remediation"].(string)
			registriesStr, _ := req.GetArguments()["registries"].(string)

			opts := policy.PolicyOpts{
				Name:              name,
				Namespace:         ns,
				RemediationAction: remediation,
			}
			if registriesStr != "" {
				for _, r := range splitTrim(registriesStr) {
					opts.AllowedRegistries = append(opts.AllowedRegistries, r)
				}
			}
			if op, ok := req.GetArguments()["operator"].(string); ok && op != "" {
				opts.OperatorName = op
			}
			if v, ok := req.GetArguments()["operator_version"].(string); ok {
				opts.OperatorVersion = v
			}
			if ch, ok := req.GetArguments()["operator_channel"].(string); ok {
				opts.OperatorChannel = ch
			}
			if days, ok := req.GetArguments()["cert_expiry_days"].(float64); ok && days > 0 {
				opts.CertExpiryDays = int(days)
			}
			if ns, ok := req.GetArguments()["cert_namespaces"].(string); ok && ns != "" {
				opts.CertNamespaces = splitTrim(ns)
			}
			if err := pol.Apply(ctx, opts); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Policy %s applied successfully", name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_remove_policy",
			mcp.WithDescription("Remove a policy and its placement resources. Idempotent, no leftovers."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Policy name")),
			mcp.WithString("namespace", mcp.Description("Policy namespace (default: global-set)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			ns, _ := req.GetArguments()["namespace"].(string)
			removed, err := pol.Remove(ctx, name, ns)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if !removed {
				return mcp.NewToolResultText(fmt.Sprintf("Policy %s not found (nothing to remove)", name)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Policy %s removed successfully", name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_apply_quota_policy",
			mcp.WithDescription("Apply resource quota enforcement policy to a cluster. Stamps quota labels on the ManagedCluster and creates a ConfigurationPolicy to monitor compliance. Educate-first model: inform, not auto-enforce."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster name")),
			mcp.WithNumber("max_workers", mcp.Description("Maximum worker nodes allowed (0 = skip)")),
			mcp.WithNumber("max_gpus", mcp.Description("Maximum GPU nodes allowed (0 = skip)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			maxWorkersF, _ := req.GetArguments()["max_workers"].(float64)
			maxGPUsF, _ := req.GetArguments()["max_gpus"].(float64)
			maxWorkers, maxGPUs := int(maxWorkersF), int(maxGPUsF)
			if maxWorkers <= 0 && maxGPUs <= 0 {
				return mcp.NewToolResultError("at least one of max_workers or max_gpus must be positive"), nil
			}
			if err := pol.ApplyQuotaPolicy(ctx, cluster, maxWorkers, maxGPUs); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Quota policy applied to %s (max-workers=%d, max-gpus=%d)", cluster, maxWorkers, maxGPUs)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_quota_status",
			mcp.WithDescription("Check resource quota compliance for a cluster. Shows quota labels and policy compliance status."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			status, err := pol.GetQuotaStatus(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(status, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_set_policy_remediation",
			mcp.WithDescription("Change a policy's remediation action between inform and enforce."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Policy name")),
			mcp.WithString("action", mcp.Required(), mcp.Description("inform or enforce")),
			mcp.WithString("namespace", mcp.Description("Policy namespace (default: global-set)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			action, _ := req.RequireString("action")
			ns, _ := req.GetArguments()["namespace"].(string)
			if err := pol.SetRemediation(ctx, name, ns, action); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Policy %s remediation set to %s", name, action)), nil
		},
	)
}

func splitTrim(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func registerTenantTools(s *server.MCPServer, ten *tenant.Manager) {
	s.AddTool(
		mcp.NewTool("acm_deploy_tenant",
			mcp.WithDescription("Deploy tenant isolation (namespace, RBAC, network policy, quota) to a spoke cluster via ManifestWork. Idempotent."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Tenant name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target spoke cluster")),
			mcp.WithString("team", mcp.Description("Team/group for RBAC (default: tenant name)")),
			mcp.WithString("cpu", mcp.Description("CPU request limit (default: 4)")),
			mcp.WithString("memory", mcp.Description("Memory request limit (default: 8Gi)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			cluster, _ := req.RequireString("cluster")
			team, _ := req.GetArguments()["team"].(string)
			cpu, _ := req.GetArguments()["cpu"].(string)
			mem, _ := req.GetArguments()["memory"].(string)
			opts := tenant.TenantOpts{
				Name:        name,
				Cluster:     cluster,
				Team:        team,
				CPULimit:    cpu,
				MemoryLimit: mem,
			}
			if err := ten.Deploy(ctx, opts); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Tenant %s deployed to %s", name, cluster)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_remove_tenant",
			mcp.WithDescription("Remove tenant isolation from a spoke cluster. Idempotent, no leftovers."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Tenant name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target spoke cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			cluster, _ := req.RequireString("cluster")
			removed, err := ten.Remove(ctx, name, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if !removed {
				return mcp.NewToolResultText(fmt.Sprintf("Tenant %s not found on %s (nothing to remove)", name, cluster)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Tenant %s removed from %s", name, cluster)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_list_tenants",
			mcp.WithDescription("List tenants deployed to a spoke cluster with sync status."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Spoke cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			tenants, err := ten.List(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(tenants, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_tenant_status",
			mcp.WithDescription("Get detailed tenant ManifestWork sync status with per-resource results."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Tenant name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Spoke cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			cluster, _ := req.RequireString("cluster")
			ms, err := ten.Status(ctx, name, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(ms, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerProvisioningTools(s *server.MCPServer, prov *provisioning.Manager, cfg config.Config) {
	s.AddTool(
		mcp.NewTool("acm_provision_preflight",
			mcp.WithDescription("Run preflight checks before provisioning — validates credentials, quota, image sets, and name conflicts. Run this before acm_provision_create to catch problems in seconds instead of failing after 40 minutes."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name to check")),
			mcp.WithString("platform", mcp.Description("Cloud platform: ibmcloud, aws")),
			mcp.WithString("pull_secret", mcp.Description("Pull secret JSON")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			platform, _ := req.GetArguments()["platform"].(string)
			pullSecret, _ := req.GetArguments()["pull_secret"].(string)

			if platform == "" {
				platform = cfg.Platform
			}
			opts := provisioning.ClusterOpts{
				Name:       name,
				Platform:   platform,
				PullSecret: pullSecret,
			}
			if platform == "aws" {
				awsCreds, err := provisioning.LoadAWSCredentials("")
				if err == nil {
					opts.AWSAccessKeyID = awsCreds.AccessKeyID
					opts.AWSSecretAccessKey = awsCreds.SecretAccessKey
				}
			}
			if platform == "ibmcloud" {
				opts.IBMCloudAPIKey = cfg.IBMCloudAPIKey
			}

			results, err := prov.Preflight(ctx, opts)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("preflight error: %v", err)), nil
			}
			return mcp.NewToolResultText(provisioning.FormatPreflightResults(results)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_provision_create",
			mcp.WithDescription("Create a spoke cluster via Hive ClusterDeployment. Requires pull secret. Idempotent. For IBM Cloud, IAM credentials are auto-generated."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("platform", mcp.Description("Cloud platform: ibmcloud, aws, gcp, azure (default: from config)")),
			mcp.WithString("region", mcp.Description("Cloud region (default: from config)")),
			mcp.WithString("image_set", mcp.Description("ClusterImageSet name (default: from config)")),
			mcp.WithString("worker_type", mcp.Description("Worker instance type (default: from config)")),
			mcp.WithString("workers", mcp.Description("Number of worker nodes (default: 2)")),
			mcp.WithString("pull_secret", mcp.Required(), mcp.Description("Pull secret JSON content")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			pullSecret, _ := req.RequireString("pull_secret")
			platform, _ := req.GetArguments()["platform"].(string)
			region, _ := req.GetArguments()["region"].(string)
			imageSet, _ := req.GetArguments()["image_set"].(string)
			workerType, _ := req.GetArguments()["worker_type"].(string)

			if platform == "" {
				platform = cfg.Platform
			}
			opts := provisioning.ClusterOpts{
				Name:       name,
				Platform:   platform,
				Region:     region,
				ImageSet:   imageSet,
				WorkerType: workerType,
				PullSecret: pullSecret,
			}
			if platform == "aws" {
				awsCreds, err := provisioning.LoadAWSCredentials("")
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("loading AWS credentials: %v", err)), nil
				}
				opts.AWSAccessKeyID = awsCreds.AccessKeyID
				opts.AWSSecretAccessKey = awsCreds.SecretAccessKey
			}
			if err := prov.Create(ctx, opts); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Cluster %s creation initiated. Hive will provision on IBM Cloud.", name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_provision_destroy",
			mcp.WithDescription("Destroy a spoke cluster — deletes ClusterDeployment, Hive deprovisions infrastructure. Idempotent."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			if err := prov.Destroy(ctx, name); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Cluster %s destruction initiated", name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_provision_status",
			mcp.WithDescription("Get ClusterDeployment provisioning status with conditions and failure info."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			info, err := prov.Status(ctx, name)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_provision_list",
			mcp.WithDescription("List clusters provisioned via acmlab with status."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			clusters, err := prov.List(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(clusters, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_list_image_sets",
			mcp.WithDescription("List available ClusterImageSets for provisioning."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			sets, err := prov.ListImageSets(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(sets, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerHealthTool(s *server.MCPServer, c *client.Client) {
	s.AddTool(
		mcp.NewTool("acm_hub_health",
			mcp.WithDescription("Check ACM hub connectivity and verify that ACM CRDs are installed. Use this before any other tool to confirm the hub is reachable."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			checks := map[string]string{}

			_, err := c.List(ctx, client.GVRManagedCluster, "", "")
			if err != nil {
				checks["ManagedCluster"] = fmt.Sprintf("FAIL: %v", err)
			} else {
				checks["ManagedCluster"] = "OK"
			}

			_, err = c.List(ctx, client.GVRClusterDeployment, "", "")
			if err != nil {
				checks["ClusterDeployment"] = fmt.Sprintf("FAIL: %v", err)
			} else {
				checks["ClusterDeployment"] = "OK"
			}

			_, err = c.List(ctx, client.GVRManifestWork, "", "")
			if err != nil {
				checks["ManifestWork"] = fmt.Sprintf("FAIL: %v", err)
			} else {
				checks["ManifestWork"] = "OK"
			}

			allOK := true
			for _, v := range checks {
				if v != "OK" {
					allOK = false
					break
				}
			}

			result := map[string]interface{}{
				"healthy": allOK,
				"checks":  checks,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerLifecycleTools(s *server.MCPServer, lc *lifecycle.Manager) {
	s.AddTool(
		mcp.NewTool("acm_hibernate_cluster",
			mcp.WithDescription("Hibernate a Hive-provisioned cluster to save costs. Sets ClusterDeployment powerState to Hibernating. Returns immediately — poll with acm_lifecycle_status to track progress. Only works with Hive-provisioned clusters, not imported ones."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("namespace", mcp.Description("Cluster namespace (defaults to cluster name)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			namespace := name
			if ns, err := req.RequireString("namespace"); err == nil && ns != "" {
				namespace = ns
			}

			supported, err := lc.ClusterSupportsLifecycle(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("checking lifecycle support: %v", err)), nil
			}
			if !supported {
				return mcp.NewToolResultError(fmt.Sprintf("cluster %s/%s does not support lifecycle operations (no ClusterDeployment found — may be imported)", namespace, name)), nil
			}

			if err := lc.Hibernate(ctx, namespace, name); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("hibernating cluster: %v", err)), nil
			}

			state, _ := lc.GetPowerState(ctx, namespace, name)
			result := map[string]interface{}{
				"cluster":   fmt.Sprintf("%s/%s", namespace, name),
				"action":    "hibernate",
				"status":    "initiated",
				"powerState": string(state),
				"next":      "Use acm_lifecycle_status to track progress",
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_resume_cluster",
			mcp.WithDescription("Resume a hibernated Hive-provisioned cluster. Sets ClusterDeployment powerState to Running. Returns immediately — poll with acm_lifecycle_status to track progress. After resume completes, use acm_lifecycle_recover_certs to approve any expired kubelet certificates."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("namespace", mcp.Description("Cluster namespace (defaults to cluster name)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			namespace := name
			if ns, err := req.RequireString("namespace"); err == nil && ns != "" {
				namespace = ns
			}

			supported, err := lc.ClusterSupportsLifecycle(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("checking lifecycle support: %v", err)), nil
			}
			if !supported {
				return mcp.NewToolResultError(fmt.Sprintf("cluster %s/%s does not support lifecycle operations (no ClusterDeployment found — may be imported)", namespace, name)), nil
			}

			if err := lc.Resume(ctx, namespace, name); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("resuming cluster: %v", err)), nil
			}

			state, _ := lc.GetPowerState(ctx, namespace, name)
			result := map[string]interface{}{
				"cluster":   fmt.Sprintf("%s/%s", namespace, name),
				"action":    "resume",
				"status":    "initiated",
				"powerState": string(state),
				"next":      "Use acm_lifecycle_status to track progress",
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_lifecycle_status",
			mcp.WithDescription("Get the power state of a cluster (spec and status). Shows desired state vs actual state to track transitions."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("namespace", mcp.Description("Cluster namespace (defaults to cluster name)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			namespace := name
			if ns, err := req.RequireString("namespace"); err == nil && ns != "" {
				namespace = ns
			}

			supported, err := lc.ClusterSupportsLifecycle(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("checking lifecycle support: %v", err)), nil
			}
			if !supported {
				return mcp.NewToolResultError(fmt.Sprintf("cluster %s/%s does not support lifecycle operations (no ClusterDeployment found — may be imported)", namespace, name)), nil
			}

			specState, err := lc.GetPowerState(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting power state: %v", err)), nil
			}

			statusState, err := lc.GetPowerStateStatus(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting power state status: %v", err)), nil
			}

			transitioning := specState != statusState
			result := map[string]interface{}{
				"cluster":       fmt.Sprintf("%s/%s", namespace, name),
				"desiredState":  string(specState),
				"actualState":   string(statusState),
				"transitioning": transitioning,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_lifecycle_diagnose",
			mcp.WithDescription("Diagnose cluster health by cross-referencing Hive ClusterDeployment state with ACM ManagedCluster conditions. Detects inconsistencies (e.g., Hive says Running but ACM reports unavailable), checks for problem conditions, and returns actionable suggestions."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("namespace", mcp.Description("Cluster namespace (defaults to cluster name)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			namespace := name
			if ns, err := req.RequireString("namespace"); err == nil && ns != "" {
				namespace = ns
			}

			report, err := lc.Diagnose(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("diagnosing cluster: %v", err)), nil
			}

			data, _ := json.MarshalIndent(report, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_lifecycle_recover_certs",
			mcp.WithDescription("Approve expired kubelet certificates on a spoke cluster after resume from hibernation. Kubelet certs rotate every ~24h in OpenShift — if the cluster was hibernated during rotation, certs expire and nodes cannot start pods until CSRs are approved. This tool connects to the spoke via its admin kubeconfig and approves pending kubelet CSRs automatically."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("namespace", mcp.Description("Cluster namespace (defaults to cluster name)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.RequireString("name")
			namespace := name
			if ns, err := req.RequireString("namespace"); err == nil && ns != "" {
				namespace = ns
			}

			result, err := lc.PostResumeRecovery(ctx, namespace, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("recovering certificates: %v", err)), nil
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_list_lifecycle_clusters",
			mcp.WithDescription("List all Hive-provisioned clusters that support lifecycle operations (hibernate/resume). Imported clusters are excluded."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			clusters, err := lc.ListClustersWithLifecycle(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("listing clusters: %v", err)), nil
			}

			resultMap := map[string]interface{}{
				"count":    len(clusters),
				"clusters": clusters,
			}
			data, _ := json.MarshalIndent(resultMap, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerImportTools(s *server.MCPServer, imp *importing.Manager) {
	s.AddTool(
		mcp.NewTool("acm_import_cluster",
			mcp.WithDescription("Import an external cluster into ACM. Creates ManagedCluster, namespace, and KlusterletAddonConfig. If kubeconfig is provided (base64-encoded), creates an auto-import secret for automatic klusterlet installation."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Name for the imported cluster")),
			mcp.WithString("kubeconfig", mcp.Description("Base64-encoded kubeconfig of the spoke cluster for auto-import")),
			mcp.WithString("labels", mcp.Description("Comma-separated labels (key=value,key=value)")),
			mcp.WithString("cluster_set", mcp.Description("ManagedClusterSet to assign (default: 'default')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.GetArguments()["name"].(string)
			if name == "" {
				return mcp.NewToolResultError("name is required"), nil
			}

			opts := importing.ImportOptions{Name: name}

			if ks, ok := req.GetArguments()["kubeconfig"].(string); ok && ks != "" {
				decoded, err := base64.StdEncoding.DecodeString(ks)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("decoding kubeconfig: %v", err)), nil
				}
				opts.Kubeconfig = decoded
			}

			if ls, ok := req.GetArguments()["labels"].(string); ok && ls != "" {
				opts.Labels = make(map[string]string)
				for _, pair := range strings.Split(ls, ",") {
					parts := strings.SplitN(pair, "=", 2)
					if len(parts) == 2 {
						opts.Labels[parts[0]] = parts[1]
					}
				}
			}

			if cs, ok := req.GetArguments()["cluster_set"].(string); ok && cs != "" {
				opts.ClusterSet = cs
			}

			result, err := imp.Import(ctx, opts)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("importing cluster: %v", err)), nil
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_detach_cluster",
			mcp.WithDescription("Detach a cluster from ACM management. Does NOT destroy the cluster — only removes it from ACM. The klusterlet on the spoke is cleaned up automatically by ACM."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Name of the cluster to detach")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.GetArguments()["name"].(string)
			if name == "" {
				return mcp.NewToolResultError("name is required"), nil
			}

			if err := imp.Detach(ctx, name); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("detaching cluster: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Cluster %s detached from ACM", name)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_import_status",
			mcp.WithDescription("Get import status of a cluster — availability, join state, auto-import status, and labels."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.GetArguments()["name"].(string)
			if name == "" {
				return mcp.NewToolResultError("name is required"), nil
			}

			status, err := imp.GetImportStatus(ctx, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting status: %v", err)), nil
			}

			data, _ := json.MarshalIndent(status, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_list_imported_clusters",
			mcp.WithDescription("List all imported (non-Hive) clusters with their availability and join status."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			clusters, err := imp.ListImported(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("listing imported clusters: %v", err)), nil
			}

			importedResult := map[string]interface{}{
				"count":    len(clusters),
				"clusters": clusters,
			}
			data, _ := json.MarshalIndent(importedResult, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerRegistryTools(s *server.MCPServer, reg *registry.Manager) {
	s.AddTool(
		mcp.NewTool("acm_registry_list_images",
			mcp.WithDescription("List container images required by ACM on a spoke cluster. Extracts images from ManifestWorks — useful for identifying what to mirror for restricted-registry clusters."),
			mcp.WithString("cluster", mcp.Description("Name of the managed cluster"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)

			images, err := reg.ListRequiredImages(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("listing images: %v", err)), nil
			}

			data, _ := json.MarshalIndent(images, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_registry_configure_mirror",
			mcp.WithDescription("Configure image registry mirror for a cluster using ManagedClusterImageRegistry. Creates Placement + pull secret + registry config on the hub so ACM rewrites image references in klusterlet manifests."),
			mcp.WithString("cluster", mcp.Description("Name of the managed cluster"), mcp.Required()),
			mcp.WithString("mirror", mcp.Description("Mirror registry (e.g., quay.io/myorg)"), mcp.Required()),
			mcp.WithString("pull_secret_path", mcp.Description("Path to pull secret JSON for the mirror registry")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			cluster := args["cluster"].(string)
			mirror := args["mirror"].(string)

			opts := registry.MirrorConfig{
				ClusterName:    cluster,
				MirrorRegistry: mirror,
			}
			if ps, ok := args["pull_secret_path"].(string); ok && ps != "" {
				opts.PullSecretPath = ps
			}

			if err := reg.ConfigureMirror(ctx, opts); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("configuring mirror: %v", err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Image registry mirror configured for cluster %s (mirror: %s)", cluster, mirror)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_registry_mirror_status",
			mcp.WithDescription("Check if an image registry mirror is configured for a cluster."),
			mcp.WithString("cluster", mcp.Description("Name of the managed cluster"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)

			status, err := reg.GetMirrorStatus(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting mirror status: %v", err)), nil
			}

			data, _ := json.MarshalIndent(status, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_registry_generate_mirror_script",
			mcp.WithDescription("Generate a bash script with skopeo commands to mirror ACM images to a target registry."),
			mcp.WithString("cluster", mcp.Description("Name of the managed cluster"), mcp.Required()),
			mcp.WithString("target", mcp.Description("Target mirror registry (e.g., quay.io/myorg)"), mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			cluster := args["cluster"].(string)
			target := args["target"].(string)

			images, err := reg.ListRequiredImages(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("listing images: %v", err)), nil
			}

			script := registry.GenerateMirrorScript(images, target)
			return mcp.NewToolResultText(script), nil
		},
	)
}

func registerScalingTools(s *server.MCPServer, sc *scaling.Manager) {
	s.AddTool(
		mcp.NewTool("acm_scaling_get",
			mcp.WithDescription("Get MachinePool info for a cluster — replicas, autoscaling bounds, and platform."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			info, err := sc.GetMachinePool(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting MachinePool: %v", err)), nil
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_scaling_set",
			mcp.WithDescription("Set a fixed worker node replica count on a cluster's MachinePool. Disables autoscaling if active."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithNumber("replicas", mcp.Required(), mcp.Description("Number of worker nodes")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			replicasFloat, _ := req.GetArguments()["replicas"].(float64)
			replicas := int(replicasFloat)
			if err := sc.SetReplicasAuto(ctx, cluster, replicas); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("setting replicas: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Cluster %s worker replicas set to %d", cluster, replicas)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_scaling_auto",
			mcp.WithDescription("Enable autoscaling on a cluster's MachinePool with min/max worker node bounds."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithNumber("min", mcp.Required(), mcp.Description("Minimum worker nodes")),
			mcp.WithNumber("max", mcp.Required(), mcp.Description("Maximum worker nodes")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			minFloat, _ := req.GetArguments()["min"].(float64)
			maxFloat, _ := req.GetArguments()["max"].(float64)
			min, max := int(minFloat), int(maxFloat)
			if min >= max {
				return mcp.NewToolResultError(fmt.Sprintf("min (%d) must be less than max (%d)", min, max)), nil
			}
			if err := sc.EnableAutoscaling(ctx, cluster, min, max); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("enabling autoscaling: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Cluster %s MachinePool autoscaling enabled (min=%d max=%d)", cluster, min, max)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_scaling_list",
			mcp.WithDescription("List all MachinePools across the fleet with replicas and autoscaling info."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pools, err := sc.ListMachinePools(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("listing MachinePools: %v", err)), nil
			}
			result := map[string]interface{}{
				"count": len(pools),
				"pools": pools,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_scaling_set_flavor",
			mcp.WithDescription("Change worker node instance type on a cluster's MachinePool. Hive performs a rolling replacement."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("worker_type", mcp.Required(), mcp.Description("New worker instance type")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster, _ := req.RequireString("cluster")
			workerType, _ := req.RequireString("worker_type")
			if err := sc.SetFlavor(ctx, cluster, workerType); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("setting worker flavor: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Cluster %s worker flavor changed to %s", cluster, workerType)), nil
		},
	)
}

func registerDecommissionTools(s *server.MCPServer, m *decommission.Manager) {
	s.AddTool(
		mcp.NewTool("acm_decommission_start",
			mcp.WithDescription("Start decommission workflow for a cluster. Runs audit automatically."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
			mcp.WithString("owner", mcp.Description("Cluster owner email")),
			mcp.WithString("deadline", mcp.Description("Reclaim deadline ISO 8601")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			owner, _ := req.GetArguments()["owner"].(string)
			deadline, _ := req.GetArguments()["deadline"].(string)
			state, err := m.Start(ctx, cluster, decommission.StartOpts{Owner: owner, Deadline: deadline})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(state, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_decommission_advance",
			mcp.WithDescription("Advance decommission to the next phase."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			state, err := m.Advance(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(state, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_decommission_status",
			mcp.WithDescription("Get current decommission state for a cluster."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			state, err := m.GetState(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(state, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_decommission_list",
			mcp.WithDescription("List all active decommission workflows."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			states, err := m.List(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(states, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_decommission_cancel",
			mcp.WithDescription("Cancel a decommission workflow. Keeps the cluster."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			if err := m.Cancel(ctx, cluster); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Decommission cancelled for %s", cluster)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_decommission_audit",
			mcp.WithDescription("Run standalone audit for a cluster without starting decommission."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Cluster name")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			report, err := m.Audit(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(report, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)
}

func registerVirtualizationTools(s *server.MCPServer, m *virtualization.Manager) {
	s.AddTool(
		mcp.NewTool("acm_vm_deploy",
			mcp.WithDescription("Deploy a virtual machine to a managed cluster via ManifestWork. Requires OpenShift Virtualization on the target cluster — run ensure-cnv first."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VM name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
			mcp.WithString("cpu", mcp.Description("CPU cores (default: 2)")),
			mcp.WithString("memory", mcp.Description("Memory (default: 4Gi)")),
			mcp.WithString("image", mcp.Description("Container disk image")),
			mcp.WithString("diskSize", mcp.Description("Disk size (default: 20Gi)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			opts := virtualization.VMOpts{
				Name:    args["name"].(string),
				Cluster: args["cluster"].(string),
			}
			if v, ok := args["cpu"].(string); ok {
				opts.CPU = v
			}
			if v, ok := args["memory"].(string); ok {
				opts.Memory = v
			}
			if v, ok := args["image"].(string); ok {
				opts.Image = v
			}
			if v, ok := args["diskSize"].(string); ok {
				opts.DiskSize = v
			}
			if err := m.Deploy(ctx, opts); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("VM %s deployed to %s", opts.Name, opts.Cluster)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_remove",
			mcp.WithDescription("Remove a virtual machine from a managed cluster."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VM name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			if err := m.Remove(ctx, args["name"].(string), args["cluster"].(string)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("VM removed"), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_start",
			mcp.WithDescription("Start a virtual machine."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VM name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			if err := m.Start(ctx, args["name"].(string), args["cluster"].(string)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("VM started"), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_stop",
			mcp.WithDescription("Stop a virtual machine."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VM name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			if err := m.Stop(ctx, args["name"].(string), args["cluster"].(string)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("VM stopped"), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_migrate",
			mcp.WithDescription("Live-migrate a virtual machine to another node."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VM name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			if err := m.Migrate(ctx, args["name"].(string), args["cluster"].(string)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("Migration initiated"), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_status",
			mcp.WithDescription("Get detailed status of a virtual machine."),
			mcp.WithString("name", mcp.Required(), mcp.Description("VM name")),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			detail, err := m.Status(ctx, args["name"].(string), args["cluster"].(string))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(detail, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_list",
			mcp.WithDescription("List all virtual machines across the fleet."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			vms, err := m.List(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(vms, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_ensure_cnv",
			mcp.WithDescription("Install OpenShift Virtualization (CNV) on a managed cluster via ACM governance policy. Creates Namespace, OperatorGroup, Subscription, and HyperConverged CR. Idempotent."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			if err := m.EnsureCNVOperator(ctx, cluster); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("CNV operator policy applied to %s", cluster)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_cnv_status",
			mcp.WithDescription("Check OpenShift Virtualization installation status on a managed cluster."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			status, err := m.CNVStatus(ctx, cluster)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			data, _ := json.MarshalIndent(status, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("acm_vm_remove_cnv",
			mcp.WithDescription("Remove OpenShift Virtualization governance policy from a managed cluster."),
			mcp.WithString("cluster", mcp.Required(), mcp.Description("Target cluster")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			cluster := req.GetArguments()["cluster"].(string)
			if err := m.RemoveCNVOperator(ctx, cluster); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("CNV operator policy removed from %s", cluster)), nil
		},
	)
}
