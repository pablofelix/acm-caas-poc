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
			client.GVRManagedCluster:     "ManagedClusterList",
			client.GVRClusterDeployment:  "ClusterDeploymentList",
			client.GVRManifestWork:       "ManifestWorkList",
			client.GVRManagedClusterInfo: "ManagedClusterInfoList",
			client.GVRPolicy:             "PolicyList",
			client.GVRPlacement:          "PlacementList",
			client.GVRPlacementBinding:   "PlacementBindingList",
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

func newClusterInfoObj(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "internal.open-cluster-management.io", Version: "v1beta1", Kind: "ManagedClusterInfo",
	})
	obj.SetName(name)
	obj.SetNamespace(name)
	obj.Object["status"] = map[string]interface{}{
		"version":    "v1.30.0",
		"consoleURL": "https://console.example.com",
		"distributionInfo": map[string]interface{}{
			"ocp": map[string]interface{}{
				"channel": "stable-4.21",
				"version": "4.21.29",
			},
		},
		"nodeList": []interface{}{
			map[string]interface{}{
				"name": name + "-master-0",
				"capacity": map[string]interface{}{
					"cpu":    "8",
					"memory": "32Gi",
					"socket": "2",
				},
				"labels": map[string]interface{}{
					"node.kubernetes.io/instance-type": "bx2-8x32",
					"topology.kubernetes.io/region":    "us-south",
					"topology.kubernetes.io/zone":      "us-south-1",
				},
				"conditions": []interface{}{
					map[string]interface{}{"type": "Ready", "status": "True"},
				},
			},
		},
	}
	return obj
}

func TestListClusterResourcesReturnsData(t *testing.T) {
	c := fakeClientWithClusters(newClusterInfoObj("cluster-1"))
	resp := callTool(t, c, "acm_list_cluster_resources", nil)
	text := extractToolText(t, resp)

	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

func TestClusterResourcesReturnsNodeDetails(t *testing.T) {
	c := fakeClientWithClusters(newClusterInfoObj("my-cluster"))
	resp := callTool(t, c, "acm_cluster_resources", map[string]interface{}{"name": "my-cluster"})
	text := extractToolText(t, resp)

	var cr map[string]interface{}
	if err := json.Unmarshal([]byte(text), &cr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cr["TotalCPU"].(float64) != 8 {
		t.Errorf("TotalCPU = %v, want 8", cr["TotalCPU"])
	}
	if cr["OCPVersion"] != "4.21.29" {
		t.Errorf("OCPVersion = %v, want 4.21.29", cr["OCPVersion"])
	}
}

func TestClusterResourcesNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	s := NewServer(c, config.Config{})
	msg, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "acm_cluster_resources",
			"arguments": map[string]interface{}{"name": "nonexistent"},
		},
	})
	resp := s.HandleMessage(context.Background(), msg)
	rpcResp := resp.(mcplib.JSONRPCResponse)
	data, _ := json.Marshal(rpcResp.Result)
	var result struct {
		IsError bool `json:"isError"`
	}
	json.Unmarshal(data, &result)
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

func TestListPoliciesEmpty(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_list_policies", nil)
	text := extractToolText(t, resp)

	var policies []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &policies); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("got %d policies, want 0", len(policies))
	}
}

func TestListPoliciesWithExisting(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("test-policy")
	pol.SetNamespace("open-cluster-management-global-set")
	pol.Object["spec"] = map[string]interface{}{
		"remediationAction": "enforce",
		"disabled":          false,
	}

	c := fakeClientWithClusters(pol)
	resp := callTool(t, c, "acm_list_policies", nil)
	text := extractToolText(t, resp)

	var policies []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &policies); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("got %d policies, want 1", len(policies))
	}
	if policies[0]["Name"] != "test-policy" {
		t.Errorf("Name = %v, want test-policy", policies[0]["Name"])
	}
}

func TestApplyPolicyViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_apply_policy", map[string]interface{}{
		"name":        "my-policy",
		"remediation": "inform",
	})
	text := extractToolText(t, resp)
	if text != "Policy my-policy applied successfully" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestRemovePolicyViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_remove_policy", map[string]interface{}{
		"name": "nonexistent",
	})
	text := extractToolText(t, resp)
	if text != "Policy nonexistent removed successfully" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestSetPolicyRemediationViaMCP(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("my-policy")
	pol.SetNamespace("open-cluster-management-global-set")
	pol.Object["spec"] = map[string]interface{}{
		"remediationAction": "inform",
	}

	c := fakeClientWithClusters(pol)
	resp := callTool(t, c, "acm_set_policy_remediation", map[string]interface{}{
		"name":   "my-policy",
		"action": "enforce",
	})
	text := extractToolText(t, resp)
	if text != "Policy my-policy remediation set to enforce" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestGetPolicyViaMCP(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("my-policy")
	pol.SetNamespace("open-cluster-management-global-set")
	pol.Object["spec"] = map[string]interface{}{
		"remediationAction": "enforce",
		"disabled":          false,
	}
	pol.Object["status"] = map[string]interface{}{
		"compliant": "Compliant",
	}

	c := fakeClientWithClusters(pol)
	resp := callTool(t, c, "acm_get_policy", map[string]interface{}{
		"name": "my-policy",
	})
	text := extractToolText(t, resp)

	var info map[string]interface{}
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info["Compliant"] != "Compliant" {
		t.Errorf("Compliant = %v, want Compliant", info["Compliant"])
	}
}

func TestDeployTenantViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_deploy_tenant", map[string]interface{}{
		"name":    "team-alpha",
		"cluster": "infraops1",
	})
	text := extractToolText(t, resp)
	if text != "Tenant team-alpha deployed to infraops1" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestRemoveTenantViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_remove_tenant", map[string]interface{}{
		"name":    "nonexistent",
		"cluster": "infraops1",
	})
	text := extractToolText(t, resp)
	if text != "Tenant nonexistent removed from infraops1" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestListTenantsViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_list_tenants", map[string]interface{}{
		"cluster": "infraops1",
	})
	text := extractToolText(t, resp)

	var tenants []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &tenants); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tenants) != 0 {
		t.Errorf("got %d tenants, want 0", len(tenants))
	}
}

func TestTenantStatusViaMCP(t *testing.T) {
	mw := &unstructured.Unstructured{}
	mw.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "work.open-cluster-management.io", Version: "v1", Kind: "ManifestWork",
	})
	mw.SetName("tenant-team-alpha")
	mw.SetNamespace("infraops1")
	mw.SetLabels(map[string]string{"acmlab.redhat.com/tenant": "team-alpha"})
	mw.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Applied", "status": "True"},
		},
	}

	c := fakeClientWithClusters(mw)
	resp := callTool(t, c, "acm_tenant_status", map[string]interface{}{
		"name":    "team-alpha",
		"cluster": "infraops1",
	})
	text := extractToolText(t, resp)

	var ms map[string]interface{}
	if err := json.Unmarshal([]byte(text), &ms); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ms["Applied"] != true {
		t.Errorf("Applied = %v, want true", ms["Applied"])
	}
}
