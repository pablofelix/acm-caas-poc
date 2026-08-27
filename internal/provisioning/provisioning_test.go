package provisioning

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func testManifestsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "creds.yaml"), []byte("apiVersion: v1\nkind: Secret\n"), 0644)
	return dir
}

func fakeClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRClusterDeployment: "ClusterDeploymentList",
			client.GVRClusterImageSet:   "ClusterImageSetList",
			client.GVRNamespace:         "NamespaceList",
			client.GVRSecret:            "SecretList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func testConfig() config.Config {
	return config.Config{
		Platform:              "ibmcloud",
		IBMCloudAPIKey:        "test-api-key",
		IBMCloudRegion:        "us-south",
		BaseDomain:            "example.com",
		ClusterImageSet:       "img4.20.0-multi-appsub",
		DefaultWorkerType:     "bx2-4x16",
		DefaultMasterType:     "bx2-8x32",
		DefaultWorkerReplicas: 2,
		DefaultMasterReplicas: 3,
	}
}

func TestCreateCreatesAllResources(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig())

	err := m.Create(context.Background(), ClusterOpts{
		Name:         "spoke1",
		PullSecret:   `{"auths":{}}`,
		ManifestsDir: testManifestsDir(t),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	ns, err := c.Get(context.Background(), client.GVRNamespace, "", "spoke1")
	if err != nil {
		t.Fatalf("namespace not created: %v", err)
	}
	if ns.GetName() != "spoke1" {
		t.Errorf("namespace name = %s, want spoke1", ns.GetName())
	}

	_, err = c.Get(context.Background(), client.GVRSecret, "spoke1", "spoke1-ibmcloud-creds")
	if err != nil {
		t.Fatalf("credentials secret not created: %v", err)
	}

	_, err = c.Get(context.Background(), client.GVRSecret, "spoke1", "spoke1-pull-secret")
	if err != nil {
		t.Fatalf("pull secret not created: %v", err)
	}

	_, err = c.Get(context.Background(), client.GVRSecret, "spoke1", "spoke1-install-config")
	if err != nil {
		t.Fatalf("install-config secret not created: %v", err)
	}

	cd, err := c.Get(context.Background(), client.GVRClusterDeployment, "spoke1", "spoke1")
	if err != nil {
		t.Fatalf("ClusterDeployment not created: %v", err)
	}
	spec, _ := cd.Object["spec"].(map[string]interface{})
	platform, _ := spec["platform"].(map[string]interface{})
	ibm, _ := platform["ibmcloud"].(map[string]interface{})
	if ibm["region"] != "us-south" {
		t.Errorf("region = %v, want us-south", ibm["region"])
	}
}

func TestCreateIsIdempotent(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig())

	opts := ClusterOpts{Name: "spoke1", PullSecret: `{"auths":{}}`, ManifestsDir: testManifestsDir(t)}
	if err := m.Create(context.Background(), opts); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	if err := m.Create(context.Background(), opts); err != nil {
		t.Fatalf("second Create failed: %v", err)
	}
}

func TestCreateRequiresAPIKey(t *testing.T) {
	c := fakeClient()
	m := New(c, config.Config{})

	err := m.Create(context.Background(), ClusterOpts{
		Name:         "spoke1",
		PullSecret:   `{"auths":{}}`,
		ManifestsDir: testManifestsDir(t),
	})
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestCreateRequiresPullSecret(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig())

	err := m.Create(context.Background(), ClusterOpts{Name: "spoke1", ManifestsDir: testManifestsDir(t)})
	if err == nil {
		t.Fatal("expected error for missing pull secret")
	}
}

func TestCreateAppliesDefaults(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig())

	err := m.Create(context.Background(), ClusterOpts{
		Name:         "spoke1",
		PullSecret:   `{"auths":{}}`,
		ManifestsDir: testManifestsDir(t),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	cd, _ := c.Get(context.Background(), client.GVRClusterDeployment, "spoke1", "spoke1")
	spec, _ := cd.Object["spec"].(map[string]interface{})
	if spec["baseDomain"] != "example.com" {
		t.Errorf("baseDomain = %v, want example.com", spec["baseDomain"])
	}
	prov, _ := spec["provisioning"].(map[string]interface{})
	imgRef, _ := prov["imageSetRef"].(map[string]interface{})
	if imgRef["name"] != "img4.20.0-multi-appsub" {
		t.Errorf("imageSet = %v, want img4.20.0-multi-appsub", imgRef["name"])
	}
}

func TestDestroyDeletesClusterDeployment(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")

	c := fakeClient(cd)
	m := New(c, testConfig())

	if err := m.Destroy(context.Background(), "spoke1"); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	_, err := c.Get(context.Background(), client.GVRClusterDeployment, "spoke1", "spoke1")
	if err == nil {
		t.Fatal("ClusterDeployment should be deleted")
	}
}

func TestDestroyIsIdempotent(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig())

	if err := m.Destroy(context.Background(), "nonexistent"); err != nil {
		t.Fatalf("Destroy of nonexistent should not error: %v", err)
	}
}

func TestStatusParsesClusterDeployment(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")
	cd.Object["spec"] = map[string]interface{}{
		"baseDomain": "example.com",
		"platform": map[string]interface{}{
			"ibmcloud": map[string]interface{}{
				"region": "us-south",
			},
		},
		"provisioning": map[string]interface{}{
			"imageSetRef": map[string]interface{}{
				"name": "img4.20.0-multi-appsub",
			},
		},
	}
	cd.Object["status"] = map[string]interface{}{
		"installed": true,
		"conditions": []interface{}{
			map[string]interface{}{"type": "Provisioned", "status": "True"},
		},
	}

	c := fakeClient(cd)
	m := New(c, testConfig())

	info, err := m.Status(context.Background(), "spoke1")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !info.Installed {
		t.Error("expected Installed = true")
	}
	if !info.Provisioned {
		t.Error("expected Provisioned = true")
	}
	if info.Region != "us-south" {
		t.Errorf("Region = %s, want us-south", info.Region)
	}
}

func TestStatusParsesFailure(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")
	cd.Object["spec"] = map[string]interface{}{}
	cd.Object["status"] = map[string]interface{}{
		"installed": false,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "ProvisionFailed",
				"status": "True",
				"reason": "InsufficientQuota",
			},
		},
	}

	c := fakeClient(cd)
	m := New(c, testConfig())

	info, err := m.Status(context.Background(), "spoke1")
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if info.FailureReason != "InsufficientQuota" {
		t.Errorf("FailureReason = %s, want InsufficientQuota", info.FailureReason)
	}
}

func TestListClusters(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")
	cd.SetLabels(map[string]string{"acmlab.redhat.com/managed": "true"})
	cd.Object["spec"] = map[string]interface{}{}

	c := fakeClient(cd)
	m := New(c, testConfig())

	clusters, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	if clusters[0].Name != "spoke1" {
		t.Errorf("Name = %s, want spoke1", clusters[0].Name)
	}
}

func TestListImageSets(t *testing.T) {
	imgset := &unstructured.Unstructured{}
	imgset.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	imgset.SetName("img4.20.0-multi-appsub")
	imgset.Object["spec"] = map[string]interface{}{
		"releaseImage": "quay.io/openshift-release-dev/ocp-release@sha256:abc123",
	}

	c := fakeClient(imgset)
	m := New(c, testConfig())

	sets, err := m.ListImageSets(context.Background())
	if err != nil {
		t.Fatalf("ListImageSets failed: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("got %d sets, want 1", len(sets))
	}
	if sets[0].Name != "img4.20.0-multi-appsub" {
		t.Errorf("Name = %s, want img4.20.0-multi-appsub", sets[0].Name)
	}
}

func TestCreateWithSSHKey(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig())

	err := m.Create(context.Background(), ClusterOpts{
		Name:          "spoke1",
		PullSecret:    `{"auths":{}}`,
		SSHKey:        "ssh-rsa AAAA...",
		SSHPrivateKey: "test-ssh-private-key-data",
		ManifestsDir:  testManifestsDir(t),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = c.Get(context.Background(), client.GVRSecret, "spoke1", "spoke1-ssh-private-key")
	if err != nil {
		t.Fatalf("SSH private key secret not created: %v", err)
	}
}
