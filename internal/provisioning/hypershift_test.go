package provisioning

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

func fakeHyperShiftClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRHostedCluster:    "HostedClusterList",
			client.GVRNodePool:         "NodePoolList",
			client.GVRManagedCluster:   "ManagedClusterList",
			client.GVRNamespace:        "NamespaceList",
			client.GVRSecret:           "SecretList",
			client.GVRClusterImageSet:  "ClusterImageSetList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func testHyperShiftOpts() HyperShiftOpts {
	return HyperShiftOpts{
		Name:             "hosted1",
		Namespace:        "clusters",
		Platform:         "aws",
		Region:           "us-east-1",
		BaseDomain:       "example.com",
		NodePoolReplicas: 2,
		ReleaseImage:     "quay.io/openshift-release-dev/ocp-release:4.16.0-multi",
		PullSecret:       `{"auths":{}}`,
	}
}

func TestCreateHyperShift(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	err := m.CreateHyperShift(context.Background(), testHyperShiftOpts())
	if err != nil {
		t.Fatalf("CreateHyperShift failed: %v", err)
	}

	hc, err := c.Get(context.Background(), client.GVRHostedCluster, "clusters", "hosted1")
	if err != nil {
		t.Fatalf("HostedCluster not created: %v", err)
	}
	labels := hc.GetLabels()
	if labels["acmlab.redhat.com/managed"] != "true" {
		t.Error("expected managed label on HostedCluster")
	}
	spec, _ := hc.Object["spec"].(map[string]interface{})
	platform, _ := spec["platform"].(map[string]interface{})
	if platform["type"] != "AWS" {
		t.Errorf("platform type = %v, want AWS", platform["type"])
	}

	_, err = c.Get(context.Background(), client.GVRNodePool, "clusters", "hosted1")
	if err != nil {
		t.Fatalf("NodePool not created: %v", err)
	}

	mc, err := c.Get(context.Background(), client.GVRManagedCluster, "", "hosted1")
	if err != nil {
		t.Fatalf("ManagedCluster not created: %v", err)
	}
	mcLabels := mc.GetLabels()
	if mcLabels["acmlab.redhat.com/provisioning-type"] != "hypershift" {
		t.Error("expected provisioning-type=hypershift label on ManagedCluster")
	}
	mcAnnotations := mc.GetAnnotations()
	if mcAnnotations["import.open-cluster-management.io/klusterlet-deploy-mode"] != "Hosted" {
		t.Error("expected Hosted klusterlet deploy mode annotation")
	}
}

func TestCreateHyperShiftIsIdempotent(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	opts := testHyperShiftOpts()
	if err := m.CreateHyperShift(context.Background(), opts); err != nil {
		t.Fatalf("first CreateHyperShift failed: %v", err)
	}
	if err := m.CreateHyperShift(context.Background(), opts); err != nil {
		t.Fatalf("second CreateHyperShift failed: %v", err)
	}
}

func TestCreateHyperShiftMissingPullSecret(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	opts := testHyperShiftOpts()
	opts.PullSecret = ""
	err := m.CreateHyperShift(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for missing pull secret")
	}
}

func TestCreateHyperShiftMissingReleaseImage(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	opts := testHyperShiftOpts()
	opts.ReleaseImage = ""
	err := m.CreateHyperShift(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for missing release image")
	}
}

func TestCreateHyperShiftDefaults(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	opts := HyperShiftOpts{
		Name:         "hosted2",
		ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.16.0",
		PullSecret:   `{"auths":{}}`,
	}
	if err := m.CreateHyperShift(context.Background(), opts); err != nil {
		t.Fatalf("CreateHyperShift failed: %v", err)
	}

	hc, err := c.Get(context.Background(), client.GVRHostedCluster, DefaultHyperShiftNamespace, "hosted2")
	if err != nil {
		t.Fatalf("HostedCluster not found in default namespace: %v", err)
	}
	if hc.GetNamespace() != DefaultHyperShiftNamespace {
		t.Errorf("namespace = %s, want %s", hc.GetNamespace(), DefaultHyperShiftNamespace)
	}

	np, err := c.Get(context.Background(), client.GVRNodePool, DefaultHyperShiftNamespace, "hosted2")
	if err != nil {
		t.Fatalf("NodePool not found: %v", err)
	}
	spec, _ := np.Object["spec"].(map[string]interface{})
	replicas, _ := spec["replicas"].(int64)
	if replicas != 2 {
		t.Errorf("default replicas = %d, want 2", replicas)
	}
}

func TestDestroyHyperShift(t *testing.T) {
	hc := &unstructured.Unstructured{}
	hc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hypershift.openshift.io", Version: "v1beta1", Kind: "HostedCluster",
	})
	hc.SetName("hosted1")
	hc.SetNamespace("clusters")

	np := &unstructured.Unstructured{}
	np.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hypershift.openshift.io", Version: "v1beta1", Kind: "NodePool",
	})
	np.SetName("hosted1")
	np.SetNamespace("clusters")

	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("hosted1")

	c := fakeHyperShiftClient(hc, np, mc)
	m := New(c, testConfig(), discardLogger)

	if err := m.DestroyHyperShift(context.Background(), "clusters", "hosted1"); err != nil {
		t.Fatalf("DestroyHyperShift failed: %v", err)
	}

	_, err := c.Get(context.Background(), client.GVRHostedCluster, "clusters", "hosted1")
	if err == nil {
		t.Error("HostedCluster should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRNodePool, "clusters", "hosted1")
	if err == nil {
		t.Error("NodePool should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRManagedCluster, "", "hosted1")
	if err == nil {
		t.Error("ManagedCluster should be deleted")
	}
}

func TestDestroyHyperShiftNonexistent(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	if err := m.DestroyHyperShift(context.Background(), "clusters", "nonexistent"); err != nil {
		t.Fatalf("DestroyHyperShift of nonexistent should not error: %v", err)
	}
}

func TestDestroyHyperShiftDefaultNamespace(t *testing.T) {
	hc := &unstructured.Unstructured{}
	hc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hypershift.openshift.io", Version: "v1beta1", Kind: "HostedCluster",
	})
	hc.SetName("hosted1")
	hc.SetNamespace(DefaultHyperShiftNamespace)

	c := fakeHyperShiftClient(hc)
	m := New(c, testConfig(), discardLogger)

	if err := m.DestroyHyperShift(context.Background(), "", "hosted1"); err != nil {
		t.Fatalf("DestroyHyperShift failed: %v", err)
	}
}

func TestStatusHyperShift(t *testing.T) {
	hc := &unstructured.Unstructured{}
	hc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hypershift.openshift.io", Version: "v1beta1", Kind: "HostedCluster",
	})
	hc.SetName("hosted1")
	hc.SetNamespace("clusters")
	hc.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Available", "status": "True"},
			map[string]interface{}{"type": "Progressing", "status": "False"},
		},
		"version": map[string]interface{}{
			"desired": map[string]interface{}{
				"image": "quay.io/openshift-release-dev/ocp-release:4.16.0",
			},
		},
	}

	c := fakeHyperShiftClient(hc)
	m := New(c, testConfig(), discardLogger)

	info, err := m.StatusHyperShift(context.Background(), "clusters", "hosted1")
	if err != nil {
		t.Fatalf("StatusHyperShift failed: %v", err)
	}
	if !info.Available {
		t.Error("expected Available = true")
	}
	if info.Version != "quay.io/openshift-release-dev/ocp-release:4.16.0" {
		t.Errorf("Version = %s, unexpected", info.Version)
	}
	if len(info.Conditions) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(info.Conditions))
	}
}

func TestStatusHyperShiftNotFound(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	_, err := m.StatusHyperShift(context.Background(), "clusters", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent HostedCluster")
	}
}

func TestStatusHyperShiftNoStatus(t *testing.T) {
	hc := &unstructured.Unstructured{}
	hc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hypershift.openshift.io", Version: "v1beta1", Kind: "HostedCluster",
	})
	hc.SetName("hosted1")
	hc.SetNamespace("clusters")
	hc.Object["spec"] = map[string]interface{}{}

	c := fakeHyperShiftClient(hc)
	m := New(c, testConfig(), discardLogger)

	info, err := m.StatusHyperShift(context.Background(), "clusters", "hosted1")
	if err != nil {
		t.Fatalf("StatusHyperShift failed: %v", err)
	}
	if info.Available {
		t.Error("expected Available = false for cluster with no status")
	}
}

func TestListHyperShift(t *testing.T) {
	var objs []runtime.Object
	for _, name := range []string{"hosted1", "hosted2"} {
		hc := &unstructured.Unstructured{}
		hc.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "hypershift.openshift.io", Version: "v1beta1", Kind: "HostedCluster",
		})
		hc.SetName(name)
		hc.SetNamespace("clusters")
		hc.SetLabels(map[string]string{"acmlab.redhat.com/managed": "true"})
		hc.Object["spec"] = map[string]interface{}{}
		objs = append(objs, hc)
	}

	c := fakeHyperShiftClient(objs...)
	m := New(c, testConfig(), discardLogger)

	infos, err := m.ListHyperShift(context.Background())
	if err != nil {
		t.Fatalf("ListHyperShift failed: %v", err)
	}
	if len(infos) != 2 {
		t.Errorf("expected 2 HostedClusters, got %d", len(infos))
	}
}

func TestListHyperShiftEmpty(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	infos, err := m.ListHyperShift(context.Background())
	if err != nil {
		t.Fatalf("ListHyperShift failed: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 HostedClusters, got %d", len(infos))
	}
}

func TestBuildHostedCluster(t *testing.T) {
	opts := testHyperShiftOpts()
	hc := buildHostedCluster(opts)

	if hc.GetName() != "hosted1" {
		t.Errorf("name = %s, want hosted1", hc.GetName())
	}
	if hc.GetNamespace() != "clusters" {
		t.Errorf("namespace = %s, want clusters", hc.GetNamespace())
	}
	spec, _ := hc.Object["spec"].(map[string]interface{})
	etcd, _ := spec["etcd"].(map[string]interface{})
	if etcd["managementType"] != "Managed" {
		t.Errorf("etcd.managementType = %v, want Managed", etcd["managementType"])
	}
	release, _ := spec["release"].(map[string]interface{})
	if release["image"] != opts.ReleaseImage {
		t.Errorf("release.image = %v, want %s", release["image"], opts.ReleaseImage)
	}
}

func TestBuildNodePool(t *testing.T) {
	opts := testHyperShiftOpts()
	np := buildNodePool(opts)

	if np.GetName() != "hosted1" {
		t.Errorf("name = %s, want hosted1", np.GetName())
	}
	spec, _ := np.Object["spec"].(map[string]interface{})
	if spec["clusterName"] != "hosted1" {
		t.Errorf("clusterName = %v, want hosted1", spec["clusterName"])
	}
	replicas, _ := spec["replicas"].(int64)
	if replicas != 2 {
		t.Errorf("replicas = %d, want 2", replicas)
	}
}

func TestPlatformToHyperShiftType(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"aws", "AWS"},
		{"azure", "Azure"},
		{"ibmcloud", "IBMCloud"},
		{"kubevirt", "KubeVirt"},
		{"unknown", "None"},
		{"", "None"},
	}
	for _, tc := range cases {
		got := platformToHyperShiftType(tc.input)
		if got != tc.want {
			t.Errorf("platformToHyperShiftType(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCreateHyperShiftNamespaceError(t *testing.T) {
	c := fakeHyperShiftClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected ns error")
	})
	m := New(c, testConfig(), discardLogger)

	err := m.CreateHyperShift(context.Background(), testHyperShiftOpts())
	if err == nil || !strings.Contains(err.Error(), "creating namespace") {
		t.Fatalf("expected namespace error, got: %v", err)
	}
}

func TestCreateHyperShiftPullSecretError(t *testing.T) {
	c := fakeHyperShiftClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected secret error")
	})
	m := New(c, testConfig(), discardLogger)

	err := m.CreateHyperShift(context.Background(), testHyperShiftOpts())
	if err == nil || !strings.Contains(err.Error(), "pull secret") {
		t.Fatalf("expected pull secret error, got: %v", err)
	}
}

func TestCreateHyperShiftHostedClusterError(t *testing.T) {
	c := fakeHyperShiftClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "hostedclusters", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected hc error")
	})
	m := New(c, testConfig(), discardLogger)

	err := m.CreateHyperShift(context.Background(), testHyperShiftOpts())
	if err == nil || !strings.Contains(err.Error(), "HostedCluster") {
		t.Fatalf("expected HostedCluster error, got: %v", err)
	}
}

func TestCreateHyperShiftNodePoolError(t *testing.T) {
	c := fakeHyperShiftClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "nodepools", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected np error")
	})
	m := New(c, testConfig(), discardLogger)

	err := m.CreateHyperShift(context.Background(), testHyperShiftOpts())
	if err == nil || !strings.Contains(err.Error(), "NodePool") {
		t.Fatalf("expected NodePool error, got: %v", err)
	}
}

func TestCreateHyperShiftManagedClusterError(t *testing.T) {
	c := fakeHyperShiftClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "managedclusters", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected mc error")
	})
	m := New(c, testConfig(), discardLogger)

	err := m.CreateHyperShift(context.Background(), testHyperShiftOpts())
	if err == nil || !strings.Contains(err.Error(), "ManagedCluster") {
		t.Fatalf("expected ManagedCluster error, got: %v", err)
	}
}

func TestResolveHyperShiftReleaseImageFromClusterImageSet(t *testing.T) {
	imageSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hive.openshift.io/v1",
			"kind":       "ClusterImageSet",
			"metadata": map[string]interface{}{
				"name": "img4.16.0-multi",
			},
			"spec": map[string]interface{}{
				"releaseImage": "quay.io/openshift-release-dev/ocp-release@sha256:abc123",
			},
		},
	}
	c := fakeHyperShiftClient(imageSet)
	m := New(c, testConfig(), discardLogger)

	opts := &HyperShiftOpts{ReleaseImage: "img4.16.0-multi"}
	err := m.resolveHyperShiftReleaseImage(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ReleaseImage != "quay.io/openshift-release-dev/ocp-release@sha256:abc123" {
		t.Fatalf("expected resolved image, got: %s", opts.ReleaseImage)
	}
}

func TestResolveHyperShiftReleaseImagePassthrough(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	opts := &HyperShiftOpts{ReleaseImage: "quay.io/openshift-release-dev/ocp-release:4.16.0"}
	err := m.resolveHyperShiftReleaseImage(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ReleaseImage != "quay.io/openshift-release-dev/ocp-release:4.16.0" {
		t.Fatalf("image should be unchanged, got: %s", opts.ReleaseImage)
	}
}

func TestResolveHyperShiftReleaseImageNotFound(t *testing.T) {
	c := fakeHyperShiftClient()
	m := New(c, testConfig(), discardLogger)

	opts := &HyperShiftOpts{ReleaseImage: "nonexistent-image-set"}
	err := m.resolveHyperShiftReleaseImage(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "resolving ClusterImageSet") {
		t.Fatalf("expected resolution error, got: %v", err)
	}
}

func TestResolveHyperShiftReleaseImageEmpty(t *testing.T) {
	imageSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hive.openshift.io/v1",
			"kind":       "ClusterImageSet",
			"metadata": map[string]interface{}{
				"name": "empty-set",
			},
			"spec": map[string]interface{}{},
		},
	}
	c := fakeHyperShiftClient(imageSet)
	m := New(c, testConfig(), discardLogger)

	opts := &HyperShiftOpts{ReleaseImage: "empty-set"}
	err := m.resolveHyperShiftReleaseImage(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "no spec.releaseImage") {
		t.Fatalf("expected empty image error, got: %v", err)
	}
}

func TestCreateHyperShiftWithClusterImageSetResolution(t *testing.T) {
	imageSet := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hive.openshift.io/v1",
			"kind":       "ClusterImageSet",
			"metadata": map[string]interface{}{
				"name": "img4.16.0-multi",
			},
			"spec": map[string]interface{}{
				"releaseImage": "quay.io/openshift-release-dev/ocp-release@sha256:abc123",
			},
		},
	}
	c := fakeHyperShiftClient(imageSet)
	m := New(c, testConfig(), discardLogger)

	opts := testHyperShiftOpts()
	opts.ReleaseImage = "img4.16.0-multi"
	err := m.CreateHyperShift(context.Background(), opts)
	if err != nil {
		t.Fatalf("CreateHyperShift with image set resolution failed: %v", err)
	}
}
