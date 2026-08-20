package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
	"github.com/pablofelix/acm-caas-poc/internal/fleet"
	"github.com/pablofelix/acm-caas-poc/internal/monitoring"
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
