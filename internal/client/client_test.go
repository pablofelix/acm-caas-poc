package client

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newFakeClient(objs ...runtime.Object) *Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "test.io", Version: "v1", Resource: "things"}:  "ThingList",
			{Group: "test.io", Version: "v1", Resource: "globals"}: "GlobalList",
		}, objs...)
	return &Client{Dynamic: fake}
}

var testGVR = schema.GroupVersionResource{Group: "test.io", Version: "v1", Resource: "things"}
var testGVK = schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "Thing"}
var clusterGVR = schema.GroupVersionResource{Group: "test.io", Version: "v1", Resource: "globals"}
var clusterGVK = schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "Global"}

func newObj(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(testGVK)
	obj.SetName(name)
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	return obj
}

func TestCreateAndGetNamespacedResource(t *testing.T) {
	c := newFakeClient()

	obj := newObj("test-thing", "default")
	created, err := c.Create(context.Background(), testGVR, "default", obj)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.GetName() != "test-thing" {
		t.Errorf("created name = %q, want %q", created.GetName(), "test-thing")
	}

	got, err := c.Get(context.Background(), testGVR, "default", "test-thing")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.GetName() != "test-thing" {
		t.Errorf("got name = %q, want %q", got.GetName(), "test-thing")
	}
}

func TestCreateAndGetClusterScopedResource(t *testing.T) {
	c := newFakeClient()

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(clusterGVK)
	obj.SetName("cluster-thing")

	_, err := c.Create(context.Background(), clusterGVR, "", obj)
	if err != nil {
		t.Fatalf("Create cluster-scoped failed: %v", err)
	}

	got, err := c.Get(context.Background(), clusterGVR, "", "cluster-thing")
	if err != nil {
		t.Fatalf("Get cluster-scoped failed: %v", err)
	}
	if got.GetName() != "cluster-thing" {
		t.Errorf("name = %q, want %q", got.GetName(), "cluster-thing")
	}
}

func TestListReturnsCreatedResources(t *testing.T) {
	c := newFakeClient()

	for _, name := range []string{"thing-1", "thing-2", "thing-3"} {
		obj := newObj(name, "ns-1")
		if _, err := c.Create(context.Background(), testGVR, "ns-1", obj); err != nil {
			t.Fatalf("Create %s failed: %v", name, err)
		}
	}

	list, err := c.List(context.Background(), testGVR, "ns-1", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list.Items) != 3 {
		t.Errorf("List returned %d items, want 3", len(list.Items))
	}
}

func TestListWithLabelSelector(t *testing.T) {
	c := newFakeClient()

	labeled := newObj("labeled", "ns-1")
	labeled.SetLabels(map[string]string{"env": "dev"})
	if _, err := c.Create(context.Background(), testGVR, "ns-1", labeled); err != nil {
		t.Fatalf("Create labeled failed: %v", err)
	}

	unlabeled := newObj("unlabeled", "ns-1")
	if _, err := c.Create(context.Background(), testGVR, "ns-1", unlabeled); err != nil {
		t.Fatalf("Create unlabeled failed: %v", err)
	}

	list, err := c.List(context.Background(), testGVR, "ns-1", "env=dev")
	if err != nil {
		t.Fatalf("List with selector failed: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("List with selector returned %d items, want 1", len(list.Items))
	}
}

func TestDeleteRemovesResource(t *testing.T) {
	c := newFakeClient()

	obj := newObj("to-delete", "default")
	if _, err := c.Create(context.Background(), testGVR, "default", obj); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := c.Delete(context.Background(), testGVR, "default", "to-delete"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := c.Get(context.Background(), testGVR, "default", "to-delete")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestDeleteNonexistentReturnsError(t *testing.T) {
	c := newFakeClient()

	err := c.Delete(context.Background(), testGVR, "default", "nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent resource, got nil")
	}
}

func TestGetNonexistentReturnsError(t *testing.T) {
	c := newFakeClient()

	_, err := c.Get(context.Background(), testGVR, "default", "nonexistent")
	if err == nil {
		t.Error("expected error getting nonexistent resource, got nil")
	}
}

func TestPatchUpdatesResource(t *testing.T) {
	c := newFakeClient()

	obj := newObj("to-patch", "default")
	if _, err := c.Create(context.Background(), testGVR, "default", obj); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	patch := []byte(`{"metadata":{"labels":{"patched":"true"}}}`)
	patched, err := c.Patch(context.Background(), testGVR, "default", "to-patch", "application/merge-patch+json", patch)
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if patched.GetLabels()["patched"] != "true" {
		t.Errorf("label patched = %q, want %q", patched.GetLabels()["patched"], "true")
	}
}

func TestWatchReturnsWatcher(t *testing.T) {
	c := newFakeClient()

	w, err := c.Watch(context.Background(), testGVR, "default", metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	defer w.Stop()

	if w.ResultChan() == nil {
		t.Error("Watch returned nil channel")
	}
}

func TestListEmptyNamespaceReturnsEmpty(t *testing.T) {
	c := newFakeClient()

	list, err := c.List(context.Background(), testGVR, "empty-ns", "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list.Items) != 0 {
		t.Errorf("List returned %d items, want 0", len(list.Items))
	}
}
