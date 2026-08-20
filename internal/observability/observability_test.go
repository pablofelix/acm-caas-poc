package observability

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
			client.GVRNamespace:                     "NamespaceList",
			client.GVRPersistentVolumeClaim:         "PersistentVolumeClaimList",
			client.GVRDeployment:                    "DeploymentList",
			client.GVRService:                       "ServiceList",
			client.GVRSecret:                        "SecretList",
			client.GVRMultiClusterObservability:      "MultiClusterObservabilityList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestSetupCreatesAllResources(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	if err := mgr.Setup(context.Background()); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	ctx := context.Background()

	if _, err := c.Get(ctx, client.GVRNamespace, "", Namespace); err != nil {
		t.Errorf("namespace not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRPersistentVolumeClaim, Namespace, MinIOName); err != nil {
		t.Errorf("PVC not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRDeployment, Namespace, MinIOName); err != nil {
		t.Errorf("Deployment not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRService, Namespace, MinIOName); err != nil {
		t.Errorf("Service not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRSecret, Namespace, SecretName); err != nil {
		t.Errorf("Secret not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRMultiClusterObservability, "", MCOName); err != nil {
		t.Errorf("MCO not created: %v", err)
	}
}

func TestSetupIsIdempotent(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	if err := mgr.Setup(context.Background()); err != nil {
		t.Fatalf("first Setup failed: %v", err)
	}
	if err := mgr.Setup(context.Background()); err != nil {
		t.Fatalf("second Setup failed (not idempotent): %v", err)
	}
}

func TestTeardownRemovesAllResources(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	if err := mgr.Setup(context.Background()); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := mgr.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown failed: %v", err)
	}

	ctx := context.Background()

	if _, err := c.Get(ctx, client.GVRDeployment, Namespace, MinIOName); err == nil {
		t.Error("Deployment still exists after teardown")
	}
	if _, err := c.Get(ctx, client.GVRService, Namespace, MinIOName); err == nil {
		t.Error("Service still exists after teardown")
	}
	if _, err := c.Get(ctx, client.GVRSecret, Namespace, SecretName); err == nil {
		t.Error("Secret still exists after teardown")
	}
	if _, err := c.Get(ctx, client.GVRMultiClusterObservability, "", MCOName); err == nil {
		t.Error("MCO still exists after teardown")
	}
}

func TestTeardownIsIdempotent(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	if err := mgr.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown on empty cluster failed (not idempotent): %v", err)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status != "NotInstalled" {
		t.Errorf("status = %q, want NotInstalled", status)
	}
}

func TestStatusPending(t *testing.T) {
	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Kind: "MultiClusterObservability",
	})
	mco.SetName(MCOName)

	c := fakeClient(mco)
	mgr := New(c, config.Config{})

	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status != "Pending" {
		t.Errorf("status = %q, want Pending", status)
	}
}

func TestStatusReady(t *testing.T) {
	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Kind: "MultiClusterObservability",
	})
	mco.SetName(MCOName)
	mco.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Ready",
				"status": "True",
			},
		},
	}

	c := fakeClient(mco)
	mgr := New(c, config.Config{})

	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status != "Ready" {
		t.Errorf("status = %q, want Ready", status)
	}
}

func TestStatusProgressing(t *testing.T) {
	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Kind: "MultiClusterObservability",
	})
	mco.SetName(MCOName)
	mco.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Ready",
				"status": "False",
			},
		},
	}

	c := fakeClient(mco)
	mgr := New(c, config.Config{})

	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status != "Progressing" {
		t.Errorf("status = %q, want Progressing", status)
	}
}
