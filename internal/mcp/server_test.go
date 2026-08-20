package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func fakeClientWithClusters(clusters ...*unstructured.Unstructured) *client.Client {
	scheme := runtime.NewScheme()
	objs := make([]runtime.Object, len(clusters))
	for i, c := range clusters {
		objs[i] = c
	}
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRManagedCluster:    "ManagedClusterList",
			client.GVRClusterDeployment: "ClusterDeploymentList",
			client.GVRManifestWork:      "ManifestWorkList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func newManagedCluster(name string, available bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	obj.SetName(name)
	obj.SetLabels(map[string]string{"vendor": "OpenShift"})

	status := "True"
	if !available {
		status = "False"
	}
	obj.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":    "ManagedClusterConditionAvailable",
				"status":  status,
				"message": "cluster is reachable",
			},
		},
		"version": map[string]interface{}{
			"kubernetes": "v1.30.0",
		},
	}
	return obj
}

func callTool(t *testing.T, c *client.Client, toolName string, args map[string]interface{}) mcplib.JSONRPCMessage {
	t.Helper()
	s := NewServer(c, config.Config{})

	params := map[string]interface{}{
		"name": toolName,
	}
	if args != nil {
		params["arguments"] = args
	}
	msg, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  params,
	})
	return s.HandleMessage(context.Background(), msg)
}

func extractToolText(t *testing.T, resp mcplib.JSONRPCMessage) string {
	t.Helper()
	rpcResp, ok := resp.(mcplib.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	data, _ := json.Marshal(rpcResp.Result)
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	return result.Content[0].Text
}

func TestNewServerReturnsNonNil(t *testing.T) {
	c := fakeClientWithClusters()
	s := NewServer(c, config.Config{})
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestFleetStatusReturnsClusterCounts(t *testing.T) {
	c := fakeClientWithClusters(
		newManagedCluster("cluster-1", true),
		newManagedCluster("cluster-2", false),
	)
	resp := callTool(t, c, "acm_fleet_status", nil)
	text := extractToolText(t, resp)

	var summary struct {
		Total    int `json:"total"`
		Healthy  int `json:"healthy"`
		Degraded int `json:"degraded"`
	}
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatalf("unmarshal fleet status: %v", err)
	}
	if summary.Total != 2 {
		t.Errorf("total = %d, want 2", summary.Total)
	}
	if summary.Healthy != 1 {
		t.Errorf("healthy = %d, want 1", summary.Healthy)
	}
	if summary.Degraded != 1 {
		t.Errorf("degraded = %d, want 1", summary.Degraded)
	}
}

func TestFleetStatusEmptyCluster(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_fleet_status", nil)
	text := extractToolText(t, resp)

	var summary struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if summary.Total != 0 {
		t.Errorf("total = %d, want 0", summary.Total)
	}
}

func TestListManagedClustersReturnsAll(t *testing.T) {
	c := fakeClientWithClusters(
		newManagedCluster("c1", true),
		newManagedCluster("c2", true),
	)
	resp := callTool(t, c, "acm_list_managed_clusters", nil)
	text := extractToolText(t, resp)

	var clusters []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &clusters); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(clusters) != 2 {
		t.Errorf("clusters count = %d, want 2", len(clusters))
	}
}

func TestGetManagedClusterReturnsDetails(t *testing.T) {
	c := fakeClientWithClusters(newManagedCluster("my-cluster", true))
	resp := callTool(t, c, "acm_get_managed_cluster", map[string]interface{}{"name": "my-cluster"})
	text := extractToolText(t, resp)

	var info map[string]interface{}
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info["Name"] != "my-cluster" {
		t.Errorf("Name = %v, want my-cluster", info["Name"])
	}
	if info["Available"] != true {
		t.Errorf("Available = %v, want true", info["Available"])
	}
}

func TestGetManagedClusterNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	s := NewServer(c, config.Config{})

	msg, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "acm_get_managed_cluster",
			"arguments": map[string]interface{}{"name": "nonexistent"},
		},
	})
	resp := s.HandleMessage(context.Background(), msg)
	rpcResp, ok := resp.(mcplib.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	data, _ := json.Marshal(rpcResp.Result)
	var result struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Error("expected isError=true for nonexistent cluster")
	}
}

func TestHubHealthAllOK(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_hub_health", nil)
	text := extractToolText(t, resp)

	var health struct {
		Healthy bool              `json:"healthy"`
		Checks  map[string]string `json:"checks"`
	}
	if err := json.Unmarshal([]byte(text), &health); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !health.Healthy {
		t.Error("expected healthy=true")
	}
	for resource, status := range health.Checks {
		if status != "OK" {
			t.Errorf("check %s = %s, want OK", resource, status)
		}
	}
}
