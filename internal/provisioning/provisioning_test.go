package provisioning

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

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
			client.GVRClusterDeployment:      "ClusterDeploymentList",
			client.GVRClusterImageSet:        "ClusterImageSetList",
			client.GVRManagedCluster:         "ManagedClusterList",
			client.GVRKlusterletAddonConfig:  "KlusterletAddonConfigList",
			client.GVRNamespace:              "NamespaceList",
			client.GVRSecret:                 "SecretList",
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
	m := New(c, testConfig(), discardLogger)

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

	mc, err := c.Get(context.Background(), client.GVRManagedCluster, "", "spoke1")
	if err != nil {
		t.Fatalf("ManagedCluster not created: %v", err)
	}
	mcSpec, _ := mc.Object["spec"].(map[string]interface{})
	if mcSpec["hubAcceptsClient"] != true {
		t.Error("expected hubAcceptsClient = true")
	}

	_, err = c.Get(context.Background(), client.GVRKlusterletAddonConfig, "spoke1", "spoke1")
	if err != nil {
		t.Fatalf("KlusterletAddonConfig not created: %v", err)
	}
}

func TestCreateIsIdempotent(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

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
	m := New(c, config.Config{}, discardLogger)

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
	m := New(c, testConfig(), discardLogger)

	err := m.Create(context.Background(), ClusterOpts{Name: "spoke1", ManifestsDir: testManifestsDir(t)})
	if err == nil {
		t.Fatal("expected error for missing pull secret")
	}
}

func TestCreateAppliesDefaults(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

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

func TestCreateAWSUsesAWSBaseDomainAndRegion(t *testing.T) {
	c := fakeClient()
	cfg := testConfig()
	cfg.AWSRegion = "us-east-1"
	cfg.AWSBaseDomain = "aws-zone.example.com"
	m := New(c, cfg, discardLogger)

	err := m.Create(context.Background(), ClusterOpts{
		Name:               "aws-spoke",
		Platform:           "aws",
		PullSecret:         `{"auths":{}}`,
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	cd, _ := c.Get(context.Background(), client.GVRClusterDeployment, "aws-spoke", "aws-spoke")
	spec, _ := cd.Object["spec"].(map[string]interface{})
	if spec["baseDomain"] != "aws-zone.example.com" {
		t.Errorf("baseDomain = %v, want aws-zone.example.com", spec["baseDomain"])
	}
	platform, _ := spec["platform"].(map[string]interface{})
	aws, _ := platform["aws"].(map[string]interface{})
	if aws["region"] != "us-east-1" {
		t.Errorf("region = %v, want us-east-1", aws["region"])
	}
}

func TestCreateAWSFallsBackToBaseDomain(t *testing.T) {
	c := fakeClient()
	cfg := testConfig()
	cfg.AWSRegion = "us-east-1"
	cfg.AWSBaseDomain = ""
	m := New(c, cfg, discardLogger)

	err := m.Create(context.Background(), ClusterOpts{
		Name:               "aws-spoke",
		Platform:           "aws",
		PullSecret:         `{"auths":{}}`,
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	cd, _ := c.Get(context.Background(), client.GVRClusterDeployment, "aws-spoke", "aws-spoke")
	spec, _ := cd.Object["spec"].(map[string]interface{})
	if spec["baseDomain"] != "example.com" {
		t.Errorf("baseDomain = %v, want example.com (fallback)", spec["baseDomain"])
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
	m := New(c, testConfig(), discardLogger)

	if err := m.Destroy(context.Background(), "spoke1"); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	_, err := c.Get(context.Background(), client.GVRClusterDeployment, "spoke1", "spoke1")
	if err == nil {
		t.Fatal("ClusterDeployment should be deleted")
	}
}

func TestDestroyCleansManagedCluster(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")

	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(client.GVRManagedCluster.GroupVersion().WithKind("ManagedCluster"))
	mc.SetName("spoke1")

	c := fakeClient(cd, mc)
	m := New(c, testConfig(), discardLogger)

	if err := m.Destroy(context.Background(), "spoke1"); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	_, err := c.Get(context.Background(), client.GVRManagedCluster, "", "spoke1")
	if err == nil {
		t.Fatal("ManagedCluster should be deleted")
	}
}

func TestDestroyReturnsErrorForNonexistent(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	err := m.Destroy(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Destroy of nonexistent cluster should return an error")
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
	m := New(c, testConfig(), discardLogger)

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
	m := New(c, testConfig(), discardLogger)

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
	m := New(c, testConfig(), discardLogger)

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
	m := New(c, testConfig(), discardLogger)

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
	m := New(c, testConfig(), discardLogger)

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

func fakeClusterDeployment(name string, installed bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	obj.SetName(name)
	obj.SetNamespace(name)
	obj.SetResourceVersion("1")
	if installed {
		obj.Object["spec"] = map[string]interface{}{"installed": true}
		obj.Object["status"] = map[string]interface{}{"installed": true}
	}
	return obj
}

func TestWaitForProvisionInstalled(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", false)
	c := fakeClient(cd)
	fakeD := c.Dynamic.(*dynamicfake.FakeDynamicClient)

	watcher := watch.NewFake()
	fakeD.PrependWatchReactor("clusterdeployments", k8stesting.DefaultWatchReactor(watcher, nil))

	cfg := testConfig()
	cfg.ProvisionTimeout = 5 * time.Second
	m := New(c, cfg, discardLogger)

	go func() {
		obj := fakeClusterDeployment("spoke1", true)
		watcher.Modify(obj)
	}()

	err := m.WaitForProvision(context.Background(), "spoke1", 0)
	if err != nil {
		t.Fatalf("WaitForProvision failed: %v", err)
	}
}

func TestWaitForProvisionFailure(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", false)
	c := fakeClient(cd)
	fakeD := c.Dynamic.(*dynamicfake.FakeDynamicClient)

	watcher := watch.NewFake()
	fakeD.PrependWatchReactor("clusterdeployments", k8stesting.DefaultWatchReactor(watcher, nil))

	m := New(c, testConfig(), discardLogger)

	go func() {
		obj := fakeClusterDeployment("spoke1", false)
		obj.Object["status"] = map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "ProvisionFailed",
					"status": "True",
					"reason": "QuotaExceeded",
				},
			},
		}
		watcher.Modify(obj)
	}()

	err := m.WaitForProvision(context.Background(), "spoke1", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for provision failure")
	}
	if !strings.Contains(err.Error(), "QuotaExceeded") {
		t.Errorf("expected QuotaExceeded in error, got: %v", err)
	}
}

func TestWaitForProvisionTimeout(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", false)
	c := fakeClient(cd)
	fakeD := c.Dynamic.(*dynamicfake.FakeDynamicClient)

	watcher := watch.NewFake()
	fakeD.PrependWatchReactor("clusterdeployments", k8stesting.DefaultWatchReactor(watcher, nil))

	m := New(c, testConfig(), discardLogger)

	err := m.WaitForProvision(context.Background(), "spoke1", 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestWaitForProvisionChannelClosedReconnects(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", false)
	c := fakeClient(cd)
	fakeD := c.Dynamic.(*dynamicfake.FakeDynamicClient)

	watcher1 := watch.NewFake()
	watcher2 := watch.NewFake()
	callCount := 0
	fakeD.PrependWatchReactor("clusterdeployments", func(action k8stesting.Action) (bool, watch.Interface, error) {
		callCount++
		if callCount == 1 {
			return true, watcher1, nil
		}
		return true, watcher2, nil
	})

	m := New(c, testConfig(), discardLogger)

	go func() {
		time.Sleep(50 * time.Millisecond)
		watcher1.Stop()
		time.Sleep(50 * time.Millisecond)
		obj := fakeClusterDeployment("spoke1", true)
		watcher2.Modify(obj)
	}()

	err := m.WaitForProvision(context.Background(), "spoke1", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForProvision should reconnect after channel close, got: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 watch calls (reconnect), got %d", callCount)
	}
}

func TestWaitForProvisionDefaultTimeout(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", false)
	c := fakeClient(cd)
	fakeD := c.Dynamic.(*dynamicfake.FakeDynamicClient)

	watcher := watch.NewFake()
	fakeD.PrependWatchReactor("clusterdeployments", k8stesting.DefaultWatchReactor(watcher, nil))

	cfg := testConfig()
	cfg.ProvisionTimeout = 100 * time.Millisecond
	m := New(c, cfg, discardLogger)

	err := m.WaitForProvision(context.Background(), "spoke1", 0)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestWaitForProvisionNotYetInstalled(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", false)
	c := fakeClient(cd)
	fakeD := c.Dynamic.(*dynamicfake.FakeDynamicClient)

	watcher := watch.NewFake()
	fakeD.PrependWatchReactor("clusterdeployments", k8stesting.DefaultWatchReactor(watcher, nil))

	cfg := testConfig()
	cfg.ProvisionTimeout = 5 * time.Second
	m := New(c, cfg, discardLogger)

	go func() {
		obj1 := fakeClusterDeployment("spoke1", false)
		watcher.Modify(obj1)

		time.Sleep(50 * time.Millisecond)
		obj2 := fakeClusterDeployment("spoke1", true)
		watcher.Modify(obj2)
	}()

	err := m.WaitForProvision(context.Background(), "spoke1", 0)
	if err != nil {
		t.Fatalf("WaitForProvision failed: %v", err)
	}
}

func TestWaitForProvisionAlreadyInstalled(t *testing.T) {
	cd := fakeClusterDeployment("spoke1", true)
	cd.Object["spec"] = map[string]interface{}{"installed": true}
	c := fakeClient(cd)

	m := New(c, testConfig(), discardLogger)

	err := m.WaitForProvision(context.Background(), "spoke1", 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForProvision should return immediately for installed cluster: %v", err)
	}
}

func TestCreateUnsupportedPlatform(t *testing.T) {
	c := fakeClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.Create(context.Background(), ClusterOpts{
		Name:       "spoke1",
		Platform:   "vmware",
		PullSecret: `{"auths":{}}`,
	})
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateAWSPlatform(t *testing.T) {
	c := fakeClient()
	cfg := testConfig()
	cfg.Platform = "aws"
	cfg.IBMCloudAPIKey = ""
	m := New(c, cfg, discardLogger)

	err := m.Create(context.Background(), ClusterOpts{
		Name:               "aws1",
		Platform:           "aws",
		PullSecret:         `{"auths":{}}`,
		Region:             "us-east-1",
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	cd, err := c.Get(context.Background(), client.GVRClusterDeployment, "aws1", "aws1")
	if err != nil {
		t.Fatalf("ClusterDeployment not created: %v", err)
	}
	spec, _ := cd.Object["spec"].(map[string]interface{})
	platform, _ := spec["platform"].(map[string]interface{})
	if _, ok := platform["aws"]; !ok {
		t.Error("expected aws platform block")
	}
	// Verify no manifests secret created for non-ibmcloud
	_, err = c.Get(context.Background(), client.GVRSecret, "aws1", "aws1-manifests")
	if err == nil {
		t.Error("AWS should not create manifests secret")
	}
}

func TestCreateGCPPlatform(t *testing.T) {
	c := fakeClient()
	cfg := testConfig()
	cfg.Platform = "gcp"
	cfg.IBMCloudAPIKey = ""
	m := New(c, cfg, discardLogger)

	err := m.Create(context.Background(), ClusterOpts{
		Name:       "gcp1",
		Platform:   "gcp",
		PullSecret: `{"auths":{}}`,
		Region:     "us-central1",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestCreateAzurePlatform(t *testing.T) {
	c := fakeClient()
	cfg := testConfig()
	cfg.Platform = "azure"
	cfg.IBMCloudAPIKey = ""
	m := New(c, cfg, discardLogger)

	err := m.Create(context.Background(), ClusterOpts{
		Name:       "azure1",
		Platform:   "azure",
		PullSecret: `{"auths":{}}`,
		Region:     "eastus",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestStatusNotFound(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	_, err := m.Status(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}

func TestListEmpty(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	clusters, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}

func TestListImageSetsEmpty(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	sets, err := m.ListImageSets(context.Background())
	if err != nil {
		t.Fatalf("ListImageSets failed: %v", err)
	}
	if len(sets) != 0 {
		t.Errorf("expected 0 image sets, got %d", len(sets))
	}
}

func TestParseClusterInfoInstalledTimestamp(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "test",
		},
		"spec": map[string]interface{}{},
		"status": map[string]interface{}{
			"installed":          false,
			"installedTimestamp": "2024-01-01T00:00:00Z",
		},
	}
	info := parseClusterInfo(obj)
	if !info.Installed {
		t.Error("expected Installed = true when installedTimestamp is present")
	}
}

func TestParseClusterInfoEmptyObject(t *testing.T) {
	info := parseClusterInfo(map[string]interface{}{})
	if info.Name != "" {
		t.Errorf("expected empty name, got %s", info.Name)
	}
	if info.Installed {
		t.Error("expected Installed = false")
	}
}

func TestParseClusterInfoMultipleConditions(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "test",
		},
		"spec": map[string]interface{}{
			"platform": map[string]interface{}{
				"aws": map[string]interface{}{
					"region": "us-east-1",
				},
			},
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
				map[string]interface{}{"type": "Provisioned", "status": "False"},
				"not-a-map",
			},
		},
	}
	info := parseClusterInfo(obj)
	if len(info.Conditions) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(info.Conditions))
	}
	if info.Provisioned {
		t.Error("expected Provisioned = false")
	}
}

func TestParseImageSetInfoEmpty(t *testing.T) {
	info := parseImageSetInfo(map[string]interface{}{})
	if info.Name != "" || info.ReleaseImage != "" {
		t.Error("expected empty ImageSetInfo for empty object")
	}
}

func TestDestroyWithoutAPIKey(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")

	cfg := testConfig()
	cfg.IBMCloudAPIKey = ""
	c := fakeClient(cd)
	m := New(c, cfg, discardLogger)

	if err := m.Destroy(context.Background(), "spoke1"); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
}

func TestCreateIfNotExistsAlreadyExists(t *testing.T) {
	ns := &unstructured.Unstructured{}
	ns.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Namespace"})
	ns.SetName("existing")

	c := fakeClient(ns)

	newNs := buildNamespace("existing")
	err := c.CreateIfNotExists(context.Background(), client.GVRNamespace, "", newNs)
	if err != nil {
		t.Fatalf("CreateIfNotExists should succeed for existing resource: %v", err)
	}
}

func TestListMultipleClusters(t *testing.T) {
	var objs []runtime.Object
	for i := 0; i < 3; i++ {
		cd := &unstructured.Unstructured{}
		cd.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
		})
		cd.SetName(fmt.Sprintf("spoke%d", i))
		cd.SetNamespace(fmt.Sprintf("spoke%d", i))
		cd.SetLabels(map[string]string{"acmlab.redhat.com/managed": "true"})
		cd.Object["spec"] = map[string]interface{}{}
		objs = append(objs, cd)
	}

	c := fakeClient(objs...)
	m := New(c, testConfig(), discardLogger)

	clusters, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(clusters) != 3 {
		t.Errorf("expected 3 clusters, got %d", len(clusters))
	}
}

func TestListMultipleImageSets(t *testing.T) {
	var objs []runtime.Object
	for i := 0; i < 3; i++ {
		imgset := &unstructured.Unstructured{}
		imgset.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
		})
		imgset.SetName(fmt.Sprintf("img-%d", i))
		imgset.Object["spec"] = map[string]interface{}{
			"releaseImage": fmt.Sprintf("quay.io/ocp:%d", i),
		}
		objs = append(objs, imgset)
	}

	c := fakeClient(objs...)
	m := New(c, testConfig(), discardLogger)

	sets, err := m.ListImageSets(context.Background())
	if err != nil {
		t.Fatalf("ListImageSets failed: %v", err)
	}
	if len(sets) != 3 {
		t.Errorf("expected 3 image sets, got %d", len(sets))
	}
}

func TestDestroyIfFailedCleansFailedCluster(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("broken")
	cd.SetNamespace("broken")
	cd.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "ProvisionFailed",
				"status": "True",
				"reason": "InfraError",
			},
		},
	}

	c := fakeClient(cd)
	m := New(c, testConfig(), discardLogger)

	destroyed, err := m.DestroyIfFailed(context.Background(), "broken")
	if err != nil {
		t.Fatalf("DestroyIfFailed returned error: %v", err)
	}
	if !destroyed {
		t.Error("expected destroyed=true for failed cluster")
	}
}

func TestDestroyIfFailedSkipsHealthyCluster(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("healthy")
	cd.SetNamespace("healthy")
	cd.Object["status"] = map[string]interface{}{
		"installed": true,
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Provisioned",
				"status": "True",
			},
		},
	}

	c := fakeClient(cd)
	m := New(c, testConfig(), discardLogger)

	destroyed, err := m.DestroyIfFailed(context.Background(), "healthy")
	if err != nil {
		t.Fatalf("DestroyIfFailed returned error: %v", err)
	}
	if destroyed {
		t.Error("expected destroyed=false for healthy cluster")
	}
}

func TestDestroyIfFailedNonexistentReturnsNoError(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	destroyed, err := m.DestroyIfFailed(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("DestroyIfFailed returned error: %v", err)
	}
	if destroyed {
		t.Error("expected destroyed=false for nonexistent cluster")
	}
}
