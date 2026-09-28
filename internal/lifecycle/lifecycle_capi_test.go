package lifecycle

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

func capiMachineDeployment(namespace, name string, replicas int64, annotations map[string]interface{}) *unstructured.Unstructured {
	md := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta2",
			"kind":       "MachineDeployment",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"replicas": replicas,
			},
		},
	}
	if annotations != nil {
		md.Object["metadata"].(map[string]interface{})["annotations"] = annotations
	}
	return md
}

func fakeCAPIClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRCAPIMachineDeployment: "MachineDeploymentList",
			client.GVRClusterDeployment:     "ClusterDeploymentList",
		},
		objs...,
	)
	return &client.Client{Dynamic: fakeDynamic}
}

func TestHibernateCAPI(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 3, nil)
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.HibernateCAPI(context.Background(), "k8s-cluster")
	if err != nil {
		t.Fatalf("HibernateCAPI() error = %v", err)
	}

	got, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-cluster", "k8s-cluster-workers")
	if err != nil {
		t.Fatalf("Get MachineDeployment error = %v", err)
	}

	replicas, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if replicas != 0 {
		t.Errorf("expected replicas=0, got %d", replicas)
	}

	ann := got.GetAnnotations()
	if ann == nil || ann[preHibernateAnnotation] != "3" {
		t.Errorf("expected pre-hibernate annotation=3, got %v", ann)
	}
}

func TestHibernateCAPIIdempotent(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 0, map[string]interface{}{
		preHibernateAnnotation: "3",
	})
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.HibernateCAPI(context.Background(), "k8s-cluster")
	if err != nil {
		t.Fatalf("HibernateCAPI() on already hibernated error = %v", err)
	}
}

func TestHibernateCAPINoMachineDeployments(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.HibernateCAPI(context.Background(), "k8s-cluster")
	if err == nil {
		t.Fatal("expected error for no MachineDeployments")
	}
}

func TestResumeCAPI(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 0, map[string]interface{}{
		preHibernateAnnotation: "3",
	})
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.ResumeCAPI(context.Background(), "k8s-cluster")
	if err != nil {
		t.Fatalf("ResumeCAPI() error = %v", err)
	}

	got, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-cluster", "k8s-cluster-workers")
	if err != nil {
		t.Fatalf("Get MachineDeployment error = %v", err)
	}

	replicas, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if replicas != 3 {
		t.Errorf("expected replicas=3, got %d", replicas)
	}
}

func TestResumeCAPIDefaultReplicas(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 0, nil)
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.ResumeCAPI(context.Background(), "k8s-cluster")
	if err != nil {
		t.Fatalf("ResumeCAPI() error = %v", err)
	}

	got, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-cluster", "k8s-cluster-workers")
	if err != nil {
		t.Fatalf("Get MachineDeployment error = %v", err)
	}

	replicas, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if replicas != int64(defaultCAPIReplicas) {
		t.Errorf("expected replicas=%d (default), got %d", defaultCAPIReplicas, replicas)
	}
}

func TestResumeCAPINoMachineDeployments(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.ResumeCAPI(context.Background(), "k8s-cluster")
	if err == nil {
		t.Fatal("expected error for no MachineDeployments")
	}
}

func TestHibernateFallbackToCAPI(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 3, nil)
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.Hibernate(context.Background(), "k8s-cluster", "k8s-cluster")
	if err != nil {
		t.Fatalf("Hibernate() with CAPI fallback error = %v", err)
	}

	got, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-cluster", "k8s-cluster-workers")
	if err != nil {
		t.Fatalf("Get MachineDeployment error = %v", err)
	}

	replicas, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if replicas != 0 {
		t.Errorf("expected replicas=0 after CAPI hibernate, got %d", replicas)
	}
}

func TestResumeFallbackToCAPI(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 0, map[string]interface{}{
		preHibernateAnnotation: "5",
	})
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.Resume(context.Background(), "k8s-cluster", "k8s-cluster")
	if err != nil {
		t.Fatalf("Resume() with CAPI fallback error = %v", err)
	}

	got, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-cluster", "k8s-cluster-workers")
	if err != nil {
		t.Fatalf("Get MachineDeployment error = %v", err)
	}

	replicas, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if replicas != 5 {
		t.Errorf("expected replicas=5, got %d", replicas)
	}
}

func TestGetPowerStateCAPIHibernating(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 0, nil)
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	state, err := m.GetPowerState(context.Background(), "k8s-cluster", "k8s-cluster")
	if err != nil {
		t.Fatalf("GetPowerState() error = %v", err)
	}
	if state != PowerStateHibernating {
		t.Errorf("expected Hibernating, got %s", state)
	}
}

func TestGetPowerStateCAPIRunning(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 3, nil)
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	state, err := m.GetPowerState(context.Background(), "k8s-cluster", "k8s-cluster")
	if err != nil {
		t.Fatalf("GetPowerState() error = %v", err)
	}
	if state != PowerStateRunning {
		t.Errorf("expected Running, got %s", state)
	}
}

func TestGetPowerStateCAPINone(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	_, err := m.GetPowerState(context.Background(), "k8s-cluster", "k8s-cluster")
	if err == nil {
		t.Fatal("expected error when no ClusterDeployment or MachineDeployment exists")
	}
}

func TestCheckLifecycleSupportCAPIKubernetes(t *testing.T) {
	mci := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "internal.open-cluster-management.io/v1beta1",
			"kind":       "ManagedClusterInfo",
			"metadata": map[string]interface{}{
				"name":      "k8s-cluster",
				"namespace": "k8s-cluster",
			},
			"status": map[string]interface{}{
				"distributionInfo": map[string]interface{}{
					"type": "kubernetes",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme, mci)
	c := &client.Client{Dynamic: fakeDynamic}
	m := New(c, config.Config{}, discardLogger)

	reason, err := m.CheckLifecycleSupport(context.Background(), "k8s-cluster", "k8s-cluster")
	if err != nil {
		t.Fatalf("CheckLifecycleSupport() error = %v", err)
	}
	if reason.Support != LifecycleFull {
		t.Errorf("expected LifecycleFull for Kubernetes, got %v", reason.Support)
	}
}

func TestResumeCAPIInvalidAnnotation(t *testing.T) {
	md := capiMachineDeployment("k8s-cluster", "k8s-cluster-workers", 0, map[string]interface{}{
		preHibernateAnnotation: "not-a-number",
	})
	c := fakeCAPIClient(md)
	m := New(c, config.Config{}, discardLogger)

	err := m.ResumeCAPI(context.Background(), "k8s-cluster")
	if err != nil {
		t.Fatalf("ResumeCAPI() error = %v", err)
	}

	got, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-cluster", "k8s-cluster-workers")
	if err != nil {
		t.Fatalf("Get MachineDeployment error = %v", err)
	}

	replicas, _, _ := unstructured.NestedInt64(got.Object, "spec", "replicas")
	if replicas != int64(defaultCAPIReplicas) {
		t.Errorf("expected default replicas=%d for invalid annotation, got %d", defaultCAPIReplicas, replicas)
	}
}
