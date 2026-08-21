package tenant

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
			client.GVRManifestWork: "ManifestWorkList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestDeployCreatesTenantManifestWork(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := TenantOpts{
		Name:    "team-alpha",
		Cluster: "infraops1",
		Team:    "alpha-devs",
	}
	if err := mgr.Deploy(context.Background(), opts); err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	obj, err := c.Get(context.Background(), client.GVRManifestWork, "infraops1", "tenant-team-alpha")
	if err != nil {
		t.Fatalf("ManifestWork not created: %v", err)
	}

	labels := obj.GetLabels()
	if labels["acmlab.redhat.com/tenant"] != "team-alpha" {
		t.Errorf("tenant label = %q, want team-alpha", labels["acmlab.redhat.com/tenant"])
	}

	spec, _ := obj.Object["spec"].(map[string]interface{})
	workload, _ := spec["workload"].(map[string]interface{})
	manifests, _ := workload["manifests"].([]interface{})
	if len(manifests) != 4 {
		t.Errorf("manifests count = %d, want 4 (Namespace, RoleBinding, NetworkPolicy, ResourceQuota)", len(manifests))
	}
}

func TestDeployIsIdempotent(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := TenantOpts{Name: "team-alpha", Cluster: "infraops1"}
	if err := mgr.Deploy(context.Background(), opts); err != nil {
		t.Fatalf("first Deploy failed: %v", err)
	}
	if err := mgr.Deploy(context.Background(), opts); err != nil {
		t.Fatalf("second Deploy failed (not idempotent): %v", err)
	}
}

func TestDeployWithCustomLimits(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := TenantOpts{
		Name:        "team-beta",
		Cluster:     "infraops1",
		CPULimit:    "8",
		MemoryLimit: "16Gi",
		PodLimit:    50,
	}
	if err := mgr.Deploy(context.Background(), opts); err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	obj, err := c.Get(context.Background(), client.GVRManifestWork, "infraops1", "tenant-team-beta")
	if err != nil {
		t.Fatalf("ManifestWork not created: %v", err)
	}

	spec, _ := obj.Object["spec"].(map[string]interface{})
	workload, _ := spec["workload"].(map[string]interface{})
	manifests, _ := workload["manifests"].([]interface{})

	quota, ok := manifests[3].(map[string]interface{})
	if !ok {
		t.Fatal("quota manifest not a map")
	}
	qSpec, _ := quota["spec"].(map[string]interface{})
	hard, _ := qSpec["hard"].(map[string]interface{})
	if hard["requests.cpu"] != "8" {
		t.Errorf("cpu = %v, want 8", hard["requests.cpu"])
	}
	if hard["requests.memory"] != "16Gi" {
		t.Errorf("memory = %v, want 16Gi", hard["requests.memory"])
	}
	if hard["pods"] != "50" {
		t.Errorf("pods = %v, want 50", hard["pods"])
	}
}

func TestRemoveDeletesManifestWork(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := TenantOpts{Name: "team-alpha", Cluster: "infraops1"}
	if err := mgr.Deploy(context.Background(), opts); err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if err := mgr.Remove(context.Background(), "team-alpha", "infraops1"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRManifestWork, "infraops1", "tenant-team-alpha"); err == nil {
		t.Error("ManifestWork still exists after remove")
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	if err := mgr.Remove(context.Background(), "nonexistent", "infraops1"); err != nil {
		t.Fatalf("Remove on empty cluster failed (not idempotent): %v", err)
	}
}

func TestListTenants(t *testing.T) {
	mw := &unstructured.Unstructured{}
	mw.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "work.open-cluster-management.io", Version: "v1", Kind: "ManifestWork",
	})
	mw.SetName("tenant-team-alpha")
	mw.SetNamespace("infraops1")
	mw.SetLabels(map[string]string{"acmlab.redhat.com/tenant": "team-alpha"})

	c := fakeClient(mw)
	mgr := New(c, config.Config{})

	tenants, err := mgr.List(context.Background(), "infraops1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("got %d tenants, want 1", len(tenants))
	}
	if tenants[0].Name != "team-alpha" {
		t.Errorf("name = %q, want team-alpha", tenants[0].Name)
	}
	if tenants[0].Cluster != "infraops1" {
		t.Errorf("cluster = %q, want infraops1", tenants[0].Cluster)
	}
	if tenants[0].Status != "Pending" {
		t.Errorf("status = %q, want Pending", tenants[0].Status)
	}
}

func TestListTenantsWithAppliedStatus(t *testing.T) {
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

	c := fakeClient(mw)
	mgr := New(c, config.Config{})

	tenants, err := mgr.List(context.Background(), "infraops1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if tenants[0].Status != "Applied" {
		t.Errorf("status = %q, want Applied", tenants[0].Status)
	}
}

func TestStatusParsesResourceStatus(t *testing.T) {
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
			map[string]interface{}{"type": "Available", "status": "True"},
		},
		"resourceStatus": map[string]interface{}{
			"manifests": []interface{}{
				map[string]interface{}{
					"resourceMeta": map[string]interface{}{
						"kind":      "Namespace",
						"name":      "team-alpha",
						"namespace": "",
					},
					"conditions": []interface{}{
						map[string]interface{}{"type": "Applied", "status": "True"},
					},
				},
				map[string]interface{}{
					"resourceMeta": map[string]interface{}{
						"kind":      "RoleBinding",
						"name":      "team-alpha-admin",
						"namespace": "team-alpha",
					},
					"conditions": []interface{}{
						map[string]interface{}{"type": "Applied", "status": "True"},
					},
				},
			},
		},
	}

	c := fakeClient(mw)
	mgr := New(c, config.Config{})

	ms, err := mgr.Status(context.Background(), "team-alpha", "infraops1")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !ms.Applied {
		t.Error("expected Applied=true")
	}
	if len(ms.Conditions) != 2 {
		t.Errorf("conditions count = %d, want 2", len(ms.Conditions))
	}
	if len(ms.Resources) != 2 {
		t.Fatalf("resources count = %d, want 2", len(ms.Resources))
	}
	if ms.Resources[0].Kind != "Namespace" {
		t.Errorf("resource[0].Kind = %q, want Namespace", ms.Resources[0].Kind)
	}
	if ms.Resources[1].Name != "team-alpha-admin" {
		t.Errorf("resource[1].Name = %q, want team-alpha-admin", ms.Resources[1].Name)
	}
}

func TestStatusNoStatus(t *testing.T) {
	mw := &unstructured.Unstructured{}
	mw.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "work.open-cluster-management.io", Version: "v1", Kind: "ManifestWork",
	})
	mw.SetName("tenant-team-alpha")
	mw.SetNamespace("infraops1")

	c := fakeClient(mw)
	mgr := New(c, config.Config{})

	ms, err := mgr.Status(context.Background(), "team-alpha", "infraops1")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if ms.Applied {
		t.Error("expected Applied=false when no status")
	}
	if len(ms.Resources) != 0 {
		t.Errorf("expected no resources, got %d", len(ms.Resources))
	}
}

func TestDeployDefaultsTeamToName(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := TenantOpts{Name: "team-gamma", Cluster: "infraops1"}
	if err := mgr.Deploy(context.Background(), opts); err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	obj, _ := c.Get(context.Background(), client.GVRManifestWork, "infraops1", "tenant-team-gamma")
	spec, _ := obj.Object["spec"].(map[string]interface{})
	workload, _ := spec["workload"].(map[string]interface{})
	manifests, _ := workload["manifests"].([]interface{})

	rb, _ := manifests[1].(map[string]interface{})
	subjects, _ := rb["subjects"].([]interface{})
	subj, _ := subjects[0].(map[string]interface{})
	if subj["name"] != "team-gamma" {
		t.Errorf("team group = %v, want team-gamma (defaulted from name)", subj["name"])
	}
}
