package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
	"github.com/pablofelix/acm-caas-poc/internal/fleet"
	"github.com/pablofelix/acm-caas-poc/internal/monitoring"
	"github.com/pablofelix/acm-caas-poc/internal/policy"
	"github.com/pablofelix/acm-caas-poc/internal/tenant"
)

func NewServer(c *client.Client, cfg config.Config) *server.MCPServer {
	s := server.NewMCPServer(
		"acmlab",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	fleetInsp := fleet.New(c, cfg)
	registerFleetTools(s, fleetInsp)
	registerHealthTool(s, c)

	mon := monitoring.New(c, cfg)
	registerMonitoringTools(s, mon)

	pol := policy.New(c, cfg)
	registerPolicyTools(s, pol)

	ten := tenant.New(c, cfg)
	registerTenantTools(s, ten)

	return s
}

func registerFleetTools(s *server.MCPServer, fi *fleet.Inspector) {
	s.AddTool(
		mcp.NewTool("acm_fleet_status",
			mcp.WithDescription("Get complete fleet status — all managed clusters with health, labels, version, and conditions. Returns a summary with total/healthy/degraded counts."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			clusters, err := fi.ListClusters(ctx)
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
			clusters, err := fi.ListClusters(ctx)
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
			mcp.WithDescription("Create a governance policy with placement targeting clusters by label. Idempotent."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Policy name")),
			mcp.WithString("namespace", mcp.Description("Policy namespace (default: global-set)")),
			mcp.WithString("remediation", mcp.Description("inform or enforce (default: inform)")),
			mcp.WithString("registries", mcp.Description("Comma-separated list of allowed container registries")),
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
			if err := pol.Remove(ctx, name, ns); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Policy %s removed successfully", name)), nil
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
			if err := ten.Remove(ctx, name, cluster); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
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
