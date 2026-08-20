package mcp

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

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

func TestNewServerReturnsNonNil(t *testing.T) {
	c := fakeClientWithClusters()
	s := NewServer(c, config.Config{})
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestNewServerWithClusters(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	obj.SetName("test-cluster")

	c := fakeClientWithClusters(obj)
	s := NewServer(c, config.Config{})
	if s == nil {
		t.Fatal("NewServer returned nil with clusters")
	}
}
