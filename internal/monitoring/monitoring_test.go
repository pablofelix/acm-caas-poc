package monitoring

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func fakeClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRManagedClusterInfo: "ManagedClusterInfoList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func newClusterInfo(name string, nodes []map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "internal.open-cluster-management.io", Version: "v1beta1", Kind: "ManagedClusterInfo",
	})
	obj.SetName(name)
	obj.SetNamespace(name)

	status := map[string]interface{}{
		"version":    "v1.30.0",
		"consoleURL": "https://console.example.com",
		"distributionInfo": map[string]interface{}{
			"ocp": map[string]interface{}{
				"channel": "stable-4.21",
				"version": "4.21.29",
			},
		},
	}

	if nodes != nil {
		nodeList := make([]interface{}, len(nodes))
		for i, n := range nodes {
			nodeList[i] = n
		}
		status["nodeList"] = nodeList
	}

	obj.Object["status"] = status
	return obj
}

func makeNode(name, cpu, memory, instanceType, region, zone string, ready bool) map[string]interface{} {
	readyStatus := "False"
	if ready {
		readyStatus = "True"
	}
	return map[string]interface{}{
		"name": name,
		"capacity": map[string]interface{}{
			"cpu":    cpu,
			"memory": memory,
			"socket": "2",
		},
		"labels": map[string]interface{}{
			"node.kubernetes.io/instance-type": instanceType,
			"topology.kubernetes.io/region":    region,
			"topology.kubernetes.io/zone":      zone,
		},
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Ready",
				"status": readyStatus,
			},
		},
	}
}

func TestGetClusterResourcesReturnsNodeInfo(t *testing.T) {
	nodes := []map[string]interface{}{
		makeNode("master-0", "8", "32911244Ki", "bx2-8x32", "us-south", "us-south-1", true),
		makeNode("master-1", "8", "32911248Ki", "bx2-8x32", "us-south", "us-south-2", true),
		makeNode("worker-0", "4", "16455624Ki", "bx2-4x16", "us-south", "us-south-1", true),
	}
	c := fakeClient(newClusterInfo("infraops1", nodes))
	mon := New(c, config.Config{})

	cr, err := mon.GetClusterResources(context.Background(), "infraops1")
	if err != nil {
		t.Fatalf("GetClusterResources failed: %v", err)
	}
	if cr.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", cr.TotalNodes)
	}
	if cr.ReadyNodes != 3 {
		t.Errorf("ReadyNodes = %d, want 3", cr.ReadyNodes)
	}
	if cr.TotalCPU != 20 {
		t.Errorf("TotalCPU = %d, want 20", cr.TotalCPU)
	}
	if cr.OCPVersion != "4.21.29" {
		t.Errorf("OCPVersion = %q, want 4.21.29", cr.OCPVersion)
	}
	if cr.Channel != "stable-4.21" {
		t.Errorf("Channel = %q, want stable-4.21", cr.Channel)
	}
}

func TestGetClusterResourcesCountsReadyNodes(t *testing.T) {
	nodes := []map[string]interface{}{
		makeNode("master-0", "8", "32Mi", "bx2-8x32", "us-south", "us-south-1", true),
		makeNode("worker-0", "4", "16Mi", "bx2-4x16", "us-south", "us-south-1", false),
	}
	c := fakeClient(newClusterInfo("test", nodes))
	mon := New(c, config.Config{})

	cr, err := mon.GetClusterResources(context.Background(), "test")
	if err != nil {
		t.Fatalf("GetClusterResources failed: %v", err)
	}
	if cr.ReadyNodes != 1 {
		t.Errorf("ReadyNodes = %d, want 1", cr.ReadyNodes)
	}
	if cr.TotalNodes != 2 {
		t.Errorf("TotalNodes = %d, want 2", cr.TotalNodes)
	}
}

func TestGetClusterResourcesNotFound(t *testing.T) {
	c := fakeClient()
	mon := New(c, config.Config{})

	_, err := mon.GetClusterResources(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent cluster, got nil")
	}
}

func TestGetClusterResourcesHandlesEmptyNodeList(t *testing.T) {
	c := fakeClient(newClusterInfo("empty", nil))
	mon := New(c, config.Config{})

	cr, err := mon.GetClusterResources(context.Background(), "empty")
	if err != nil {
		t.Fatalf("GetClusterResources failed: %v", err)
	}
	if cr.TotalNodes != 0 {
		t.Errorf("TotalNodes = %d, want 0", cr.TotalNodes)
	}
	if cr.TotalCPU != 0 {
		t.Errorf("TotalCPU = %d, want 0", cr.TotalCPU)
	}
	if cr.ConsoleURL != "https://console.example.com" {
		t.Errorf("ConsoleURL = %q, want https://console.example.com", cr.ConsoleURL)
	}
}

func TestGetClusterResourcesHandlesNilStatus(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "internal.open-cluster-management.io", Version: "v1beta1", Kind: "ManagedClusterInfo",
	})
	obj.SetName("nostatus")
	obj.SetNamespace("nostatus")

	c := fakeClient(obj)
	mon := New(c, config.Config{})

	cr, err := mon.GetClusterResources(context.Background(), "nostatus")
	if err != nil {
		t.Fatalf("GetClusterResources failed: %v", err)
	}
	if cr.TotalNodes != 0 {
		t.Errorf("TotalNodes = %d, want 0", cr.TotalNodes)
	}
}

func TestListClusterResourcesReturnsAll(t *testing.T) {
	nodes1 := []map[string]interface{}{
		makeNode("m-0", "8", "32Mi", "bx2-8x32", "us-south", "us-south-1", true),
	}
	nodes2 := []map[string]interface{}{
		makeNode("m-0", "4", "16Mi", "bx2-4x16", "eu-de", "eu-de-1", true),
		makeNode("w-0", "4", "16Mi", "bx2-4x16", "eu-de", "eu-de-2", true),
	}
	c := fakeClient(newClusterInfo("c1", nodes1), newClusterInfo("c2", nodes2))
	mon := New(c, config.Config{})

	results, err := mon.ListClusterResources(context.Background())
	if err != nil {
		t.Fatalf("ListClusterResources failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d clusters, want 2", len(results))
	}
}

func TestListClusterResourcesEmpty(t *testing.T) {
	c := fakeClient()
	mon := New(c, config.Config{})

	results, err := mon.ListClusterResources(context.Background())
	if err != nil {
		t.Fatalf("ListClusterResources failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d clusters, want 0", len(results))
	}
}

func TestNodeInfoParsesLabels(t *testing.T) {
	nodes := []map[string]interface{}{
		makeNode("worker-0", "16", "64Gi", "mx2-16x128", "eu-de", "eu-de-1", true),
	}
	c := fakeClient(newClusterInfo("labeled", nodes))
	mon := New(c, config.Config{})

	cr, err := mon.GetClusterResources(context.Background(), "labeled")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if cr.Nodes[0].InstanceType != "mx2-16x128" {
		t.Errorf("InstanceType = %q, want mx2-16x128", cr.Nodes[0].InstanceType)
	}
	if cr.Nodes[0].Region != "eu-de" {
		t.Errorf("Region = %q, want eu-de", cr.Nodes[0].Region)
	}
	if cr.Nodes[0].Zone != "eu-de-1" {
		t.Errorf("Zone = %q, want eu-de-1", cr.Nodes[0].Zone)
	}
}

func TestIsNodeReadyWithNoConditions(t *testing.T) {
	node := map[string]interface{}{
		"name":     "no-conds",
		"capacity": map[string]interface{}{"cpu": "4"},
	}
	if isNodeReady(node) {
		t.Error("expected not ready for node without conditions")
	}
}
