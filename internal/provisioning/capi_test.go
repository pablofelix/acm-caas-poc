package provisioning

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func fakeCAPIClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRCAPICluster:            "ClusterList",
			client.GVRCAPIMachineDeployment:  "MachineDeploymentList",
			client.GVRManagedCluster:         "ManagedClusterList",
			client.GVRNamespace:              "NamespaceList",
			client.GVRSecret:                 "SecretList",
			client.GVRAWSCluster:             "AWSClusterList",
			client.GVRAWSMachineTemplate:     "AWSMachineTemplateList",
			client.GVRKubeadmControlPlane:    "KubeadmControlPlaneList",
			client.GVRKubeadmConfigTemplate:  "KubeadmConfigTemplateList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestCreateCAPI(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{
		Name:       "k8s-dev-1",
		PullSecret: `{"auths":{}}`,
	})
	if err != nil {
		t.Fatalf("CreateCAPI failed: %v", err)
	}

	_, err = c.Get(context.Background(), client.GVRNamespace, "", "k8s-dev-1")
	if err != nil {
		t.Fatalf("namespace not created: %v", err)
	}

	cluster, err := c.Get(context.Background(), client.GVRCAPICluster, "k8s-dev-1", "k8s-dev-1")
	if err != nil {
		t.Fatalf("CAPI Cluster not created: %v", err)
	}
	labels := cluster.GetLabels()
	if labels["acmlab.redhat.com/provisioner"] != "capi" {
		t.Errorf("expected provisioner=capi label, got %v", labels)
	}

	md, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-dev-1", "k8s-dev-1-workers")
	if err != nil {
		t.Fatalf("MachineDeployment not created: %v", err)
	}
	spec, _ := md.Object["spec"].(map[string]interface{})
	if spec["clusterName"] != "k8s-dev-1" {
		t.Errorf("clusterName = %v, want k8s-dev-1", spec["clusterName"])
	}
	if spec["replicas"] != int64(2) {
		t.Errorf("replicas = %v, want 2", spec["replicas"])
	}

	mc, err := c.Get(context.Background(), client.GVRManagedCluster, "", "k8s-dev-1")
	if err != nil {
		t.Fatalf("ManagedCluster not created: %v", err)
	}
	mcLabels := mc.GetLabels()
	if mcLabels["vendor"] != "Kubernetes" {
		t.Errorf("vendor = %s, want Kubernetes", mcLabels["vendor"])
	}
	if mcLabels["acmlab.redhat.com/provisioner"] != "capi" {
		t.Errorf("expected provisioner=capi on ManagedCluster, got %v", mcLabels)
	}
}

func TestCreateCAPIRequiresName(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateCAPIDefaults(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	opts := CAPIClusterOpts{Name: "test-defaults"}
	m.applyCAPIDefaults(&opts)

	if opts.Namespace != "test-defaults" {
		t.Errorf("Namespace = %s, want test-defaults", opts.Namespace)
	}
	if opts.InfraProvider != "docker" {
		t.Errorf("InfraProvider = %s, want docker", opts.InfraProvider)
	}
	if opts.KubernetesVersion != "v1.34.8" {
		t.Errorf("KubernetesVersion = %s, want v1.34.8", opts.KubernetesVersion)
	}
	if opts.WorkerReplicas != 2 {
		t.Errorf("WorkerReplicas = %d, want 2", opts.WorkerReplicas)
	}
	if opts.ControlPlaneReplicas != 1 {
		t.Errorf("ControlPlaneReplicas = %d, want 1", opts.ControlPlaneReplicas)
	}
}

func TestCreateCAPIWithCustomOpts(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{
		Name:              "k8s-prod-1",
		InfraProvider:     "aws",
		Region:            "us-east-1",
		KubernetesVersion: "v1.29.0",
		WorkerReplicas:    5,
	})
	if err != nil {
		t.Fatalf("CreateCAPI failed: %v", err)
	}

	cluster, err := c.Get(context.Background(), client.GVRCAPICluster, "k8s-prod-1", "k8s-prod-1")
	if err != nil {
		t.Fatalf("CAPI Cluster not created: %v", err)
	}
	spec, _ := cluster.Object["spec"].(map[string]interface{})
	infraRef, _ := spec["infrastructureRef"].(map[string]interface{})
	if infraRef["kind"] != "AWSCluster" {
		t.Errorf("infraRef.kind = %v, want AWSCluster", infraRef["kind"])
	}

	md, _ := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-prod-1", "k8s-prod-1-workers")
	mdSpec, _ := md.Object["spec"].(map[string]interface{})
	tmpl, _ := mdSpec["template"].(map[string]interface{})
	tmplSpec, _ := tmpl["spec"].(map[string]interface{})
	if tmplSpec["version"] != "v1.29.0" {
		t.Errorf("version = %v, want v1.29.0", tmplSpec["version"])
	}
	infraMachineRef, _ := tmplSpec["infrastructureRef"].(map[string]interface{})
	if infraMachineRef["kind"] != "AWSMachineTemplate" {
		t.Errorf("machine template kind = %v, want AWSMachineTemplate", infraMachineRef["kind"])
	}
}

func TestCreateCAPIIsIdempotent(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	opts := CAPIClusterOpts{Name: "k8s-idem"}
	if err := m.CreateCAPI(context.Background(), opts); err != nil {
		t.Fatalf("first CreateCAPI failed: %v", err)
	}
	if err := m.CreateCAPI(context.Background(), opts); err != nil {
		t.Fatalf("second CreateCAPI failed: %v", err)
	}
}

func TestDestroyCAPI(t *testing.T) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster",
	})
	cluster.SetName("k8s-dev-1")
	cluster.SetNamespace("k8s-dev-1")

	md := &unstructured.Unstructured{}
	md.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta1", Kind: "MachineDeployment",
	})
	md.SetName("k8s-dev-1-workers")
	md.SetNamespace("k8s-dev-1")

	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("k8s-dev-1")

	c := fakeCAPIClient(cluster, md, mc)
	m := New(c, config.Config{}, discardLogger)

	if err := m.DestroyCAPI(context.Background(), "k8s-dev-1"); err != nil {
		t.Fatalf("DestroyCAPI failed: %v", err)
	}

	_, err := c.Get(context.Background(), client.GVRCAPICluster, "k8s-dev-1", "k8s-dev-1")
	if err == nil {
		t.Error("CAPI Cluster should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-dev-1", "k8s-dev-1-workers")
	if err == nil {
		t.Error("MachineDeployment should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRManagedCluster, "", "k8s-dev-1")
	if err == nil {
		t.Error("ManagedCluster should be deleted")
	}
}

func TestDestroyCAPIIdempotent(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	if err := m.DestroyCAPI(context.Background(), "nonexistent"); err != nil {
		t.Fatalf("DestroyCAPI of nonexistent should not error: %v", err)
	}
}

func TestStatusCAPI(t *testing.T) {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster",
	})
	cluster.SetName("k8s-dev-1")
	cluster.SetNamespace("k8s-dev-1")
	cluster.Object["status"] = map[string]interface{}{
		"phase": "Provisioned",
		"conditions": []interface{}{
			map[string]interface{}{"type": "Ready", "status": "True"},
			map[string]interface{}{"type": "InfrastructureReady", "status": "True"},
		},
	}

	c := fakeCAPIClient(cluster)
	m := New(c, config.Config{}, discardLogger)

	info, err := m.StatusCAPI(context.Background(), "k8s-dev-1")
	if err != nil {
		t.Fatalf("StatusCAPI failed: %v", err)
	}
	if info.Phase != "Provisioned" {
		t.Errorf("Phase = %s, want Provisioned", info.Phase)
	}
	if !info.Ready {
		t.Error("expected Ready = true")
	}
	if len(info.Conditions) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(info.Conditions))
	}
}

func TestStatusCAPINotFound(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	_, err := m.StatusCAPI(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}

func TestListCAPI(t *testing.T) {
	var objs []runtime.Object
	for _, name := range []string{"k8s-dev-1", "k8s-dev-2"} {
		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster",
		})
		cluster.SetName(name)
		cluster.SetNamespace(name)
		cluster.SetLabels(map[string]string{"acmlab.redhat.com/managed": "true"})
		cluster.Object["spec"] = map[string]interface{}{}
		objs = append(objs, cluster)
	}

	c := fakeCAPIClient(objs...)
	m := New(c, config.Config{}, discardLogger)

	clusters, err := m.ListCAPI(context.Background())
	if err != nil {
		t.Fatalf("ListCAPI failed: %v", err)
	}
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(clusters))
	}
}

func TestListCAPIEmpty(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	clusters, err := m.ListCAPI(context.Background())
	if err != nil {
		t.Fatalf("ListCAPI failed: %v", err)
	}
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}

func TestParseCAPIClusterInfoEmpty(t *testing.T) {
	info := parseCAPIClusterInfo(map[string]interface{}{})
	if info.Name != "" {
		t.Errorf("expected empty name, got %s", info.Name)
	}
	if info.Ready {
		t.Error("expected Ready = false")
	}
}

func TestParseCAPIClusterInfoWithConditions(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "test",
		},
		"status": map[string]interface{}{
			"phase": "Provisioning",
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "False"},
				map[string]interface{}{"type": "InfrastructureReady", "status": "True"},
				"not-a-map",
			},
		},
	}
	info := parseCAPIClusterInfo(obj)
	if info.Phase != "Provisioning" {
		t.Errorf("Phase = %s, want Provisioning", info.Phase)
	}
	if info.Ready {
		t.Error("expected Ready = false")
	}
	if len(info.Conditions) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(info.Conditions))
	}
}

func TestBuildCAPIClusterInfraKinds(t *testing.T) {
	providers := map[string]string{
		"aws":     "AWSCluster",
		"azure":   "AzureCluster",
		"gcp":     "GCPCluster",
		"docker":  "DockerCluster",
		"unknown": "InfrastructureCluster",
	}
	for provider, expectedKind := range providers {
		kind := infraClusterKind(provider)
		if kind != expectedKind {
			t.Errorf("infraClusterKind(%s) = %s, want %s", provider, kind, expectedKind)
		}
	}
}

func TestBuildCAPIMachineTemplateKinds(t *testing.T) {
	providers := map[string]string{
		"aws":     "AWSMachineTemplate",
		"azure":   "AzureMachineTemplate",
		"gcp":     "GCPMachineTemplate",
		"docker":  "DockerMachineTemplate",
		"unknown": "InfrastructureMachineTemplate",
	}
	for provider, expectedKind := range providers {
		kind := infraMachineTemplateKind(provider)
		if kind != expectedKind {
			t.Errorf("infraMachineTemplateKind(%s) = %s, want %s", provider, kind, expectedKind)
		}
	}
}

func TestCreateCAPINamespaceError(t *testing.T) {
	c := fakeCAPIClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected ns error")
	})
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{Name: "test-ns-err"})
	if err == nil || !contains(err.Error(), "creating namespace") {
		t.Fatalf("expected namespace error, got: %v", err)
	}
}

func TestCreateCAPIClusterError(t *testing.T) {
	c := fakeCAPIClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "clusters", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected cluster error")
	})
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{Name: "test-cluster-err"})
	if err == nil || !contains(err.Error(), "creating CAPI Cluster") {
		t.Fatalf("expected CAPI Cluster error, got: %v", err)
	}
}

func TestCreateCAPIMachineDeploymentError(t *testing.T) {
	c := fakeCAPIClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "machinedeployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected md error")
	})
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{Name: "test-md-err"})
	if err == nil || !contains(err.Error(), "creating CAPI MachineDeployment") {
		t.Fatalf("expected MachineDeployment error, got: %v", err)
	}
}

func TestCreateCAPIManagedClusterError(t *testing.T) {
	c := fakeCAPIClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "managedclusters", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected mc error")
	})
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{Name: "test-mc-err"})
	if err == nil || !contains(err.Error(), "creating ManagedCluster") {
		t.Fatalf("expected ManagedCluster error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCreateCAPIAWS(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{AWSRegion: "us-east-1"}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{
		Name:          "k8s-aws-1",
		InfraProvider: "aws",
		Region:        "us-east-1",
		SSHKeyName:    "test-key",
	})
	if err != nil {
		t.Fatalf("CreateCAPI AWS failed: %v", err)
	}

	awsCluster, err := c.Get(context.Background(), client.GVRAWSCluster, "k8s-aws-1", "k8s-aws-1")
	if err != nil {
		t.Fatalf("AWSCluster not created: %v", err)
	}
	spec, _ := awsCluster.Object["spec"].(map[string]interface{})
	if spec["region"] != "us-east-1" {
		t.Errorf("AWSCluster region = %v, want us-east-1", spec["region"])
	}
	if spec["sshKeyName"] != "test-key" {
		t.Errorf("AWSCluster sshKeyName = %v, want test-key", spec["sshKeyName"])
	}

	_, err = c.Get(context.Background(), client.GVRAWSMachineTemplate, "k8s-aws-1", "k8s-aws-1-control-plane")
	if err != nil {
		t.Fatalf("AWSMachineTemplate control-plane not created: %v", err)
	}

	kcp, err := c.Get(context.Background(), client.GVRKubeadmControlPlane, "k8s-aws-1", "k8s-aws-1-control-plane")
	if err != nil {
		t.Fatalf("KubeadmControlPlane not created: %v", err)
	}
	kcpSpec, _ := kcp.Object["spec"].(map[string]interface{})
	if kcpSpec["version"] != "v1.34.8" {
		t.Errorf("KubeadmControlPlane version = %v, want v1.34.8", kcpSpec["version"])
	}
	if kcpSpec["replicas"] != int64(1) {
		t.Errorf("KubeadmControlPlane replicas = %v, want 1", kcpSpec["replicas"])
	}

	cluster, err := c.Get(context.Background(), client.GVRCAPICluster, "k8s-aws-1", "k8s-aws-1")
	if err != nil {
		t.Fatalf("CAPI Cluster not created: %v", err)
	}
	clusterSpec, _ := cluster.Object["spec"].(map[string]interface{})
	infraRef, _ := clusterSpec["infrastructureRef"].(map[string]interface{})
	if infraRef["kind"] != "AWSCluster" {
		t.Errorf("infrastructureRef.kind = %v, want AWSCluster", infraRef["kind"])
	}
	cpRef, _ := clusterSpec["controlPlaneRef"].(map[string]interface{})
	if cpRef["kind"] != "KubeadmControlPlane" {
		t.Errorf("controlPlaneRef.kind = %v, want KubeadmControlPlane", cpRef["kind"])
	}

	_, err = c.Get(context.Background(), client.GVRAWSMachineTemplate, "k8s-aws-1", "k8s-aws-1-workers")
	if err != nil {
		t.Fatalf("AWSMachineTemplate workers not created: %v", err)
	}

	_, err = c.Get(context.Background(), client.GVRKubeadmConfigTemplate, "k8s-aws-1", "k8s-aws-1-workers")
	if err != nil {
		t.Fatalf("KubeadmConfigTemplate not created: %v", err)
	}

	md, err := c.Get(context.Background(), client.GVRCAPIMachineDeployment, "k8s-aws-1", "k8s-aws-1-workers")
	if err != nil {
		t.Fatalf("MachineDeployment not created: %v", err)
	}
	mdSpec, _ := md.Object["spec"].(map[string]interface{})
	if mdSpec["replicas"] != int64(2) {
		t.Errorf("MachineDeployment replicas = %v, want 2", mdSpec["replicas"])
	}

	mc, err := c.Get(context.Background(), client.GVRManagedCluster, "", "k8s-aws-1")
	if err != nil {
		t.Fatalf("ManagedCluster not created: %v", err)
	}
	mcLabels := mc.GetLabels()
	if mcLabels["vendor"] != "Kubernetes" {
		t.Errorf("ManagedCluster vendor = %s, want Kubernetes", mcLabels["vendor"])
	}
}

func TestCreateCAPIAWSRequiresRegion(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{}, discardLogger)

	err := m.CreateCAPI(context.Background(), CAPIClusterOpts{
		Name:          "k8s-no-region",
		InfraProvider: "aws",
	})
	if err == nil || !contains(err.Error(), "region is required") {
		t.Fatalf("expected region error, got: %v", err)
	}
}

func TestDestroyCAPIAWS(t *testing.T) {
	awsCluster := &unstructured.Unstructured{}
	awsCluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "AWSCluster",
	})
	awsCluster.SetName("k8s-aws-1")
	awsCluster.SetNamespace("k8s-aws-1")

	cpTemplate := &unstructured.Unstructured{}
	cpTemplate.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "AWSMachineTemplate",
	})
	cpTemplate.SetName("k8s-aws-1-control-plane")
	cpTemplate.SetNamespace("k8s-aws-1")

	kcp := &unstructured.Unstructured{}
	kcp.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "controlplane.cluster.x-k8s.io", Version: "v1beta1", Kind: "KubeadmControlPlane",
	})
	kcp.SetName("k8s-aws-1-control-plane")
	kcp.SetNamespace("k8s-aws-1")

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Cluster",
	})
	cluster.SetName("k8s-aws-1")
	cluster.SetNamespace("k8s-aws-1")

	workerTemplate := &unstructured.Unstructured{}
	workerTemplate.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2", Kind: "AWSMachineTemplate",
	})
	workerTemplate.SetName("k8s-aws-1-workers")
	workerTemplate.SetNamespace("k8s-aws-1")

	bootstrapTemplate := &unstructured.Unstructured{}
	bootstrapTemplate.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "bootstrap.cluster.x-k8s.io", Version: "v1beta1", Kind: "KubeadmConfigTemplate",
	})
	bootstrapTemplate.SetName("k8s-aws-1-workers")
	bootstrapTemplate.SetNamespace("k8s-aws-1")

	md := &unstructured.Unstructured{}
	md.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta1", Kind: "MachineDeployment",
	})
	md.SetName("k8s-aws-1-workers")
	md.SetNamespace("k8s-aws-1")

	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("k8s-aws-1")

	c := fakeCAPIClient(awsCluster, cpTemplate, kcp, cluster, workerTemplate, bootstrapTemplate, md, mc)
	m := New(c, config.Config{}, discardLogger)

	if err := m.DestroyCAPI(context.Background(), "k8s-aws-1"); err != nil {
		t.Fatalf("DestroyCAPI AWS failed: %v", err)
	}

	_, err := c.Get(context.Background(), client.GVRAWSCluster, "k8s-aws-1", "k8s-aws-1")
	if err == nil {
		t.Error("AWSCluster should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRKubeadmControlPlane, "k8s-aws-1", "k8s-aws-1-control-plane")
	if err == nil {
		t.Error("KubeadmControlPlane should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRCAPICluster, "k8s-aws-1", "k8s-aws-1")
	if err == nil {
		t.Error("CAPI Cluster should be deleted")
	}
	_, err = c.Get(context.Background(), client.GVRManagedCluster, "", "k8s-aws-1")
	if err == nil {
		t.Error("ManagedCluster should be deleted")
	}
}

func TestCreateCAPIAWSDefaults(t *testing.T) {
	c := fakeCAPIClient()
	m := New(c, config.Config{Platform: "aws", AWSRegion: "eu-west-1"}, discardLogger)

	opts := CAPIClusterOpts{Name: "test-defaults-aws"}
	m.applyCAPIDefaults(&opts)

	if opts.InfraProvider != "aws" {
		t.Errorf("InfraProvider = %s, want aws", opts.InfraProvider)
	}
	if opts.Region != "eu-west-1" {
		t.Errorf("Region = %s, want eu-west-1", opts.Region)
	}
	if opts.InstanceType != "t3.large" {
		t.Errorf("InstanceType = %s, want t3.large", opts.InstanceType)
	}
	if opts.RootVolumeSize != 80 {
		t.Errorf("RootVolumeSize = %d, want 80", opts.RootVolumeSize)
	}
}

func TestBuildAWSCluster(t *testing.T) {
	opts := CAPIClusterOpts{
		Name:      "test-aws",
		Namespace: "test-aws",
		Region:    "us-east-1",
		SSHKeyName: "my-key",
	}
	obj := buildAWSCluster(opts)
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec["region"] != "us-east-1" {
		t.Errorf("region = %v, want us-east-1", spec["region"])
	}
	if spec["sshKeyName"] != "my-key" {
		t.Errorf("sshKeyName = %v, want my-key", spec["sshKeyName"])
	}
}

func TestBuildAWSMachineTemplateWithAMI(t *testing.T) {
	opts := CAPIClusterOpts{
		Name:           "test-ami",
		Namespace:      "test-ami",
		InstanceType:   "t3.xlarge",
		AMI:            "ami-12345678",
		RootVolumeSize: 100,
		SSHKeyName:     "my-key",
	}
	obj := buildAWSMachineTemplate("test-ami-cp", opts)
	spec, _ := obj.Object["spec"].(map[string]interface{})
	tmpl, _ := spec["template"].(map[string]interface{})
	tmplSpec, _ := tmpl["spec"].(map[string]interface{})
	if tmplSpec["instanceType"] != "t3.xlarge" {
		t.Errorf("instanceType = %v, want t3.xlarge", tmplSpec["instanceType"])
	}
	ami, _ := tmplSpec["ami"].(map[string]interface{})
	if ami["id"] != "ami-12345678" {
		t.Errorf("ami.id = %v, want ami-12345678", ami["id"])
	}
	rootVol, _ := tmplSpec["rootVolume"].(map[string]interface{})
	if rootVol["size"] != int64(100) {
		t.Errorf("rootVolume.size = %v, want 100", rootVol["size"])
	}
	if tmplSpec["sshKeyName"] != "my-key" {
		t.Errorf("sshKeyName = %v, want my-key", tmplSpec["sshKeyName"])
	}
}

func TestGetLabelsEmpty(t *testing.T) {
	_, ok := getLabels(map[string]interface{}{})
	if ok {
		t.Error("expected ok=false for empty object")
	}

	_, ok = getLabels(map[string]interface{}{
		"metadata": map[string]interface{}{},
	})
	if ok {
		t.Error("expected ok=false for metadata without labels")
	}
}
