package fleet

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

func fakeClusterClient(clusters ...*unstructured.Unstructured) *client.Client {
	scheme := runtime.NewScheme()
	objs := make([]runtime.Object, len(clusters))
	for i, c := range clusters {
		objs[i] = c
	}
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRManagedCluster: "ManagedClusterList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func managedCluster(name string, available bool, labels map[string]string) *unstructured.Unstructured {
	availStatus := "True"
	if !available {
		availStatus = "False"
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	obj.SetName(name)
	if labels != nil {
		obj.SetLabels(labels)
	}
	obj.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":    "ManagedClusterConditionAvailable",
				"status":  availStatus,
				"message": "cluster is reachable",
			},
			map[string]interface{}{
				"type":   "HubAcceptedManagedCluster",
				"status": "True",
			},
			map[string]interface{}{
				"type":   "ManagedClusterJoined",
				"status": "True",
			},
		},
		"version": map[string]interface{}{
			"kubernetes": "v1.29.0",
		},
	}
	return obj
}

func TestListClustersReturnsParsedInfo(t *testing.T) {
	c := fakeClusterClient(
		managedCluster("spoke-1", true, map[string]string{"cloud": "IBMCloud"}),
		managedCluster("spoke-2", false, map[string]string{"cloud": "AWS"}),
	)
	insp := New(c, config.Config{})

	clusters, err := insp.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(clusters))
	}
	if clusters[0].Name != "spoke-1" {
		t.Errorf("first cluster name = %q, want %q", clusters[0].Name, "spoke-1")
	}
	if !clusters[0].Available {
		t.Error("spoke-1 should be available")
	}
	if clusters[1].Available {
		t.Error("spoke-2 should not be available")
	}
}

func TestListClustersReturnsEmptyForNoResources(t *testing.T) {
	c := fakeClusterClient()
	insp := New(c, config.Config{})

	clusters, err := insp.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}
	if len(clusters) != 0 {
		t.Errorf("got %d clusters, want 0", len(clusters))
	}
}

func TestGetClusterReturnsLabelsAndConditions(t *testing.T) {
	labels := map[string]string{"cloud": "IBMCloud", "region": "us-south"}
	c := fakeClusterClient(managedCluster("test-cluster", true, labels))
	insp := New(c, config.Config{})

	info, err := insp.GetCluster(context.Background(), "test-cluster")
	if err != nil {
		t.Fatalf("GetCluster failed: %v", err)
	}
	if info.Labels["cloud"] != "IBMCloud" {
		t.Errorf("label cloud = %q, want %q", info.Labels["cloud"], "IBMCloud")
	}
	if info.Labels["region"] != "us-south" {
		t.Errorf("label region = %q, want %q", info.Labels["region"], "us-south")
	}
	if !info.Accepted {
		t.Error("expected Accepted = true")
	}
	if !info.Joined {
		t.Error("expected Joined = true")
	}
	if info.Version != "v1.29.0" {
		t.Errorf("Version = %q, want %q", info.Version, "v1.29.0")
	}
	if len(info.Conditions) != 3 {
		t.Errorf("got %d conditions, want 3", len(info.Conditions))
	}
}

func TestGetClusterReturnsErrorForMissing(t *testing.T) {
	c := fakeClusterClient()
	insp := New(c, config.Config{})

	_, err := insp.GetCluster(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing cluster")
	}
}

func TestParseClusterInfoHandlesNilStatus(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	obj.SetName("no-status")

	info := parseClusterInfo(obj.Object)
	if info.Name != "no-status" {
		t.Errorf("Name = %q, want %q", info.Name, "no-status")
	}
	if info.Available {
		t.Error("expected Available = false for nil status")
	}
	if len(info.Conditions) != 0 {
		t.Errorf("got %d conditions, want 0", len(info.Conditions))
	}
}

func TestParseClusterInfoHandlesNilLabels(t *testing.T) {
	obj := managedCluster("no-labels", true, nil)
	info := parseClusterInfo(obj.Object)
	if info.Labels != nil {
		t.Errorf("expected nil labels, got %v", info.Labels)
	}
}

func TestParseClusterInfoReadsConditionMessage(t *testing.T) {
	obj := managedCluster("with-msg", true, nil)
	info := parseClusterInfo(obj.Object)

	found := false
	for _, c := range info.Conditions {
		if c.Type == "ManagedClusterConditionAvailable" && c.Message == "cluster is reachable" {
			found = true
		}
	}
	if !found {
		t.Error("expected condition with message 'cluster is reachable'")
	}
}
