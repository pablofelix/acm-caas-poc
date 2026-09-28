package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func fakeSetupClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRDeployment:                "DeploymentList",
			client.GVRNamespace:                 "NamespaceList",
			client.GVRCustomResourceDefinition:  "CustomResourceDefinitionList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func setupManager(c *client.Client) *Manager {
	return New(c, config.Config{Platform: "aws", AWSRegion: "us-east-1"}, discardLogger)
}

func TestAreCAPIControllersInstalled(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	installed, err := m.areCAPIControllersInstalled(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Fatal("expected not installed when no deployments exist")
	}
}

func TestAreCAPIControllersInstalledTrue(t *testing.T) {
	var objs []runtime.Object
	for ns, info := range capiNamespaces {
		dep := &unstructured.Unstructured{}
		dep.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		dep.SetName(info.Deployment)
		dep.SetNamespace(ns)
		objs = append(objs, dep)
	}

	c := fakeSetupClient(objs...)
	m := setupManager(c)

	installed, err := m.areCAPIControllersInstalled(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Fatal("expected installed when all deployments exist")
	}
}

func TestInstallCAPIControllers(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var capturedArgs []string
	m.clusterctlExec = func(args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte("installed"), nil
	}

	err := m.installCAPIControllers(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedArgs) == 0 {
		t.Fatal("clusterctl was not called")
	}
	if capturedArgs[0] != "init" {
		t.Errorf("expected init subcommand, got %s", capturedArgs[0])
	}
	hasAWS := false
	for i, arg := range capturedArgs {
		if arg == "--infrastructure" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "aws" {
			hasAWS = true
		}
	}
	if !hasAWS {
		t.Error("expected --infrastructure aws in clusterctl args")
	}
}

func TestInstallCAPIControllersMultipleProviders(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var capturedArgs []string
	m.clusterctlExec = func(args ...string) ([]byte, error) {
		capturedArgs = args
		return []byte("installed"), nil
	}

	err := m.installCAPIControllers(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws", "ibmcloud"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infraCount := 0
	for _, arg := range capturedArgs {
		if arg == "--infrastructure" {
			infraCount++
		}
	}
	if infraCount != 2 {
		t.Errorf("expected 2 --infrastructure flags, got %d", infraCount)
	}
}

func TestGrantCAPISCCs(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var grantedSAs []string
	m.ocExec = func(args ...string) ([]byte, error) {
		if len(args) >= 5 && args[0] == "adm" && args[2] == "add-scc-to-user" {
			grantedSAs = append(grantedSAs, args[4])
		}
		return []byte("added"), nil
	}

	err := m.grantCAPISCCs(context.Background(), []string{"aws"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSAs := []string{
		"system:serviceaccount:capi-system:capi-manager",
		"system:serviceaccount:capi-kubeadm-bootstrap-system:capi-kubeadm-bootstrap-manager",
		"system:serviceaccount:capi-kubeadm-control-plane-system:capi-kubeadm-control-plane-manager",
		"system:serviceaccount:capa-system:capa-controller-manager",
	}
	for _, expected := range expectedSAs {
		found := false
		for _, sa := range grantedSAs {
			if sa == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected SCC grant for %s, not found in %v", expected, grantedSAs)
		}
	}
}

func TestGrantCAPISCCsUsesPrivilegedSCC(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var sccNames []string
	m.ocExec = func(args ...string) ([]byte, error) {
		if len(args) >= 4 && args[2] == "add-scc-to-user" {
			sccNames = append(sccNames, args[3])
		}
		return []byte("added"), nil
	}

	m.grantCAPISCCs(context.Background(), []string{"aws"})

	for _, name := range sccNames {
		if name != "privileged" {
			t.Errorf("expected privileged SCC, got %s", name)
		}
	}
}

func TestEnsureAWSSSHKeyPairAlreadyExists(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.awsExec = func(args ...string) ([]byte, error) {
		if args[0] == "ec2" && args[1] == "describe-key-pairs" {
			return json.Marshal(map[string]interface{}{
				"KeyPairs": []map[string]interface{}{
					{"KeyName": "test-key"},
				},
			})
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}

	created, err := m.EnsureAWSSSHKeyPair(context.Background(), "us-east-1", "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected not created when key already exists")
	}
}

func TestEnsureAWSSSHKeyPairCreatesNew(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var createCalled bool
	m.awsExec = func(args ...string) ([]byte, error) {
		if args[0] == "ec2" && args[1] == "describe-key-pairs" {
			return nil, fmt.Errorf("not found")
		}
		if args[0] == "ec2" && args[1] == "create-key-pair" {
			createCalled = true
			hasKeyName := false
			for i, arg := range args {
				if arg == "--key-name" && i+1 < len(args) && args[i+1] == "test-key" {
					hasKeyName = true
				}
			}
			if !hasKeyName {
				t.Error("expected --key-name test-key")
			}
			return []byte("key-12345"), nil
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}

	created, err := m.EnsureAWSSSHKeyPair(context.Background(), "us-east-1", "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected key to be created")
	}
	if !createCalled {
		t.Error("expected create-key-pair to be called")
	}
}

func TestEnsureAWSSSHKeyPairRequiresName(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	_, err := m.EnsureAWSSSHKeyPair(context.Background(), "us-east-1", "")
	if err == nil {
		t.Fatal("expected error when key name is empty")
	}
}

func TestDeleteAWSSSHKeyPair(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var deleteCalled bool
	m.awsExec = func(args ...string) ([]byte, error) {
		if args[0] == "ec2" && args[1] == "delete-key-pair" {
			deleteCalled = true
			return []byte("{}"), nil
		}
		return nil, fmt.Errorf("unexpected: %v", args)
	}

	err := m.DeleteAWSSSHKeyPair(context.Background(), "us-east-1", "test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Error("expected delete-key-pair to be called")
	}
}

func TestSanitizeOutput(t *testing.T) {
	tests := []struct {
		input    string
		redacted bool
	}{
		{"normal output", false},
		{"secret_access_key=AKIA...", true},
		{"password=hunter2", true},
		{"token=abc123", true},
		{"credential file not found", true},
		{"deployment scaled", false},
	}
	for _, tt := range tests {
		result := sanitizeOutput([]byte(tt.input))
		if tt.redacted && !strings.Contains(result, "redacted") {
			t.Errorf("expected redacted for input %q, got %q", tt.input, result)
		}
		if !tt.redacted && strings.Contains(result, "redacted") {
			t.Errorf("did not expect redaction for input %q", tt.input)
		}
	}
}

func TestCAPISetupResultSummary(t *testing.T) {
	result := &CAPISetupResult{
		ControllersInstalled: true,
		CRDsUpgraded:         true,
		SCCsGranted:          true,
		PodsReady:            true,
		SSHKeyName:           "test-key",
		SSHKeyCreated:        true,
	}
	summary := result.Summary()
	if !strings.Contains(summary, "Controllers installed: true") {
		t.Error("expected controllers installed in summary")
	}
	if !strings.Contains(summary, "test-key") {
		t.Error("expected SSH key name in summary")
	}
}

func TestCAPISetupResultSummaryWithErrors(t *testing.T) {
	result := &CAPISetupResult{
		Errors: []string{"SCC grant failed", "pods not ready"},
	}
	summary := result.Summary()
	if !strings.Contains(summary, "Errors:") {
		t.Error("expected errors section in summary")
	}
	if !strings.Contains(summary, "SCC grant failed") {
		t.Error("expected specific error in summary")
	}
}

func TestSetupCAPIControllersSkipsInstallWhenPresent(t *testing.T) {
	var objs []runtime.Object
	for ns, info := range capiNamespaces {
		dep := &unstructured.Unstructured{}
		dep.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		dep.SetName(info.Deployment)
		dep.SetNamespace(ns)
		objs = append(objs, dep)
	}

	machineCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "machines.cluster.x-k8s.io"},
			"spec": map[string]interface{}{
				"versions": []interface{}{
					map[string]interface{}{"name": "v1beta1"},
					map[string]interface{}{"name": "v1beta2"},
				},
			},
		},
	}
	objs = append(objs, machineCRD)

	c := fakeSetupClient(objs...)
	m := setupManager(c)

	clusterctlCalled := false
	m.clusterctlExec = func(args ...string) ([]byte, error) {
		clusterctlCalled = true
		return []byte("installed"), nil
	}
	m.ocExec = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	m.kubectlExec = func(args ...string) ([]byte, error) { return []byte("ok"), nil }

	result, err := m.SetupCAPIControllers(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
		WaitTimeout:    1 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clusterctlCalled {
		t.Error("clusterctl should not be called when controllers already installed")
	}
	if !result.ControllersInstalled {
		t.Error("expected controllers installed")
	}
	if result.CRDsUpgraded {
		t.Error("expected CRDs not upgraded when v1beta2 already present")
	}
}

func TestUpgradeCAPICRDsSkipsWhenV1beta2Present(t *testing.T) {
	machineCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "machines.cluster.x-k8s.io"},
			"spec": map[string]interface{}{
				"versions": []interface{}{
					map[string]interface{}{"name": "v1beta1"},
					map[string]interface{}{"name": "v1beta2"},
				},
			},
		},
	}

	c := fakeSetupClient(machineCRD)
	m := setupManager(c)

	upgraded, err := m.upgradeCAPICRDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgraded {
		t.Error("expected not upgraded when v1beta2 already present")
	}
}

func TestCAPIControllerStatusNotInstalled(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	statuses, err := m.CAPIControllerStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range statuses {
		if s.Ready {
			t.Errorf("expected not ready for %s/%s", s.Namespace, s.Deployment)
		}
	}
}

func TestUpgradeCAPIWebhooksUpdatesV1beta1Paths(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var appliedFiles []string
	m.kubectlExec = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "get" {
			whJSON := fmt.Sprintf(`{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind": "MutatingWebhookConfiguration",
				"metadata": {"name": "%s"},
				"webhooks": [{
					"name": "test",
					"clientConfig": {"service": {"path": "/mutate-cluster-x-k8s-io-v1beta1-cluster"}},
					"rules": [{"apiVersions": ["v1beta1"]}]
				}]
			}`, args[2])
			return []byte(whJSON), nil
		}
		if len(args) >= 2 && args[0] == "apply" {
			appliedFiles = append(appliedFiles, args[2])
			return []byte("configured"), nil
		}
		return []byte("ok"), nil
	}

	upgraded, err := m.upgradeCAPIWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upgraded {
		t.Error("expected webhooks to be upgraded")
	}
	if len(appliedFiles) != len(capiWebhookConfigs) {
		t.Errorf("expected %d webhook applies, got %d", len(capiWebhookConfigs), len(appliedFiles))
	}
}

func TestUpgradeCAPIWebhooksSkipsV1beta2(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.kubectlExec = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "get" {
			whJSON := fmt.Sprintf(`{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind": "MutatingWebhookConfiguration",
				"metadata": {"name": "%s"},
				"webhooks": [{
					"name": "test",
					"clientConfig": {"service": {"path": "/mutate-cluster-x-k8s-io-v1beta2-cluster"}},
					"rules": [{"apiVersions": ["v1beta2"]}]
				}]
			}`, args[2])
			return []byte(whJSON), nil
		}
		t.Fatal("should not apply when already v1beta2")
		return nil, nil
	}

	upgraded, err := m.upgradeCAPIWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgraded {
		t.Error("expected no upgrade when already v1beta2")
	}
}

func TestSetupCAPIControllersFullInstall(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.clusterctlExec = func(args ...string) ([]byte, error) {
		return []byte("installed"), nil
	}
	m.ocExec = func(args ...string) ([]byte, error) { return []byte("ok"), nil }
	m.kubectlExec = func(args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "get" && (strings.HasSuffix(args[1], "webhookconfiguration") || strings.HasSuffix(args[1], "webhookconfigurations")) {
			return nil, fmt.Errorf("not found")
		}
		return []byte("ok"), nil
	}
	m.awsExec = func(args ...string) ([]byte, error) {
		if args[1] == "describe-key-pairs" {
			return json.Marshal(map[string]interface{}{"KeyPairs": []interface{}{}})
		}
		return []byte("key-id"), nil
	}

	result, err := m.SetupCAPIControllers(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
		SSHKeyName:     "test-key",
		WaitTimeout:    1 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ControllersInstalled {
		t.Error("expected controllers installed")
	}
	if !result.SCCsGranted {
		t.Error("expected SCCs granted")
	}
	if result.SSHKeyName != "test-key" {
		t.Errorf("expected SSH key name test-key, got %s", result.SSHKeyName)
	}
	if !result.SSHKeyCreated {
		t.Error("expected SSH key created")
	}
}

func TestSetupCAPIControllersInstallError(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.clusterctlExec = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("clusterctl failed")
	}

	result, err := m.SetupCAPIControllers(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
		WaitTimeout:    1 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error on install failure")
	}
	if result.ControllersInstalled {
		t.Error("should not mark installed on error")
	}
}

func TestUpgradeCAPICRDsNeedUpgrade(t *testing.T) {
	machineCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "machines.cluster.x-k8s.io"},
			"spec": map[string]interface{}{
				"versions": []interface{}{
					map[string]interface{}{"name": "v1beta1"},
				},
			},
		},
	}

	c := fakeSetupClient(machineCRD)
	m := setupManager(c)

	m.kubectlExec = func(args ...string) ([]byte, error) {
		return []byte("applied"), nil
	}

	upgraded, err := m.upgradeCAPICRDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upgraded {
		t.Error("expected CRDs to be upgraded when only v1beta1")
	}
}

func TestCAPIControllerStatusWithDeployments(t *testing.T) {
	var objs []runtime.Object
	for ns, info := range capiNamespaces {
		dep := &unstructured.Unstructured{}
		dep.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		dep.SetName(info.Deployment)
		dep.SetNamespace(ns)
		dep.Object["status"] = map[string]interface{}{
			"readyReplicas": int64(1),
		}
		dep.Object["spec"] = map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"image": "registry.k8s.io/cluster-api/cluster-api-controller:v1.14.2",
						},
					},
				},
			},
		}
		objs = append(objs, dep)
	}
	for _, infra := range capiInfraNamespaces {
		dep := &unstructured.Unstructured{}
		dep.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		dep.SetName(infra.Deployment)
		dep.SetNamespace(infra.Namespace)
		dep.Object["status"] = map[string]interface{}{
			"readyReplicas": int64(1),
		}
		dep.Object["spec"] = map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"image": "registry.k8s.io/cluster-api-aws/cluster-api-aws-controller:v2.13.0",
						},
					},
				},
			},
		}
		objs = append(objs, dep)
	}

	c := fakeSetupClient(objs...)
	m := setupManager(c)

	statuses, err := m.CAPIControllerStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range statuses {
		if !s.Ready {
			t.Errorf("expected ready for %s/%s", s.Namespace, s.Deployment)
		}
		if s.Image == "" {
			t.Errorf("expected image for %s/%s", s.Namespace, s.Deployment)
		}
	}
}

func TestFormatCAPIControllerStatus(t *testing.T) {
	statuses := []CAPIControllerStatus{
		{Namespace: "capi-system", Deployment: "capi-controller-manager", Ready: true, Image: "test:v1"},
		{Namespace: "capa-system", Deployment: "capa-controller-manager", Ready: false},
	}
	output := FormatCAPIControllerStatus(statuses)
	if !strings.Contains(output, "ready") {
		t.Error("expected 'ready' in output")
	}
	if !strings.Contains(output, "not ready") {
		t.Error("expected 'not ready' in output")
	}
	if !strings.Contains(output, "test:v1") {
		t.Error("expected image in output")
	}
}

func TestRestartCAPIDeployments(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var restartedNamespaces []string
	m.kubectlExec = func(args ...string) ([]byte, error) {
		if args[0] == "rollout" && args[1] == "restart" {
			for i, a := range args {
				if a == "-n" && i+1 < len(args) {
					restartedNamespaces = append(restartedNamespaces, args[i+1])
				}
			}
		}
		return []byte("ok"), nil
	}

	err := m.restartCAPIDeployments(context.Background(), []string{"aws"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(restartedNamespaces) != 4 {
		t.Errorf("expected 4 restarts (3 core + 1 infra), got %d", len(restartedNamespaces))
	}
}

func TestDeleteAWSSSHKeyPairError(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.awsExec = func(args ...string) ([]byte, error) {
		return []byte("error"), fmt.Errorf("delete failed")
	}

	err := m.DeleteAWSSSHKeyPair(context.Background(), "us-east-1", "test-key")
	if err == nil {
		t.Fatal("expected error on delete failure")
	}
}

func TestRunClusterctlDefault(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	_, err := m.runClusterctl("version")
	if err == nil {
		t.Skip("clusterctl not available")
	}
}

func TestRunOCDefault(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	_, err := m.runOC("version")
	if err == nil {
		t.Skip("oc not available")
	}
}

func TestRunKubectlDefault(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	_, err := m.runKubectl("version", "--client")
	if err == nil {
		t.Skip("kubectl not available")
	}
}

func TestUpgradeCAPIWebhooksParseError(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.kubectlExec = func(args ...string) ([]byte, error) {
		if args[0] == "get" {
			return []byte("not json"), nil
		}
		return []byte("ok"), nil
	}

	_, err := m.upgradeCAPIWebhooks(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestWaitForCAPIPods(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.kubectlExec = func(args ...string) ([]byte, error) {
		return []byte("deployment successfully rolled out"), nil
	}

	err := m.waitForCAPIPods(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
		WaitTimeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForCAPIPodsTimeout(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.kubectlExec = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("timed out")
	}

	err := m.waitForCAPIPods(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
		WaitTimeout:    1 * time.Second,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGrantCAPISCCsError(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.ocExec = func(args ...string) ([]byte, error) {
		return []byte("forbidden"), fmt.Errorf("permission denied")
	}

	err := m.grantCAPISCCs(context.Background(), []string{"aws"})
	if err == nil {
		t.Fatal("expected error on SCC grant failure")
	}
}

func TestInstallCAPIControllersError(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.clusterctlExec = func(args ...string) ([]byte, error) {
		return []byte("error"), fmt.Errorf("clusterctl failed")
	}

	err := m.installCAPIControllers(context.Background(), CAPISetupOpts{
		InfraProviders: []string{"aws"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyCRDsViaDownload(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.curlExec = func(args ...string) ([]byte, error) {
		return []byte("downloaded"), nil
	}
	m.kubectlExec = func(args ...string) ([]byte, error) {
		return []byte("applied"), nil
	}

	out, err := m.applyCRDsViaDownload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "applied" {
		t.Errorf("expected 'applied', got %s", string(out))
	}
}

func TestApplyCRDsViaDownloadCurlError(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	m.curlExec = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("curl failed")
	}

	_, err := m.applyCRDsViaDownload(context.Background())
	if err == nil {
		t.Fatal("expected error on curl failure")
	}
}

func TestUpgradeCAPICRDsFallback(t *testing.T) {
	machineCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]interface{}{"name": "machines.cluster.x-k8s.io"},
			"spec": map[string]interface{}{
				"versions": []interface{}{
					map[string]interface{}{"name": "v1beta1"},
				},
			},
		},
	}

	c := fakeSetupClient(machineCRD)
	m := setupManager(c)

	callCount := 0
	m.kubectlExec = func(args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("server-side apply failed")
		}
		return []byte("applied via fallback"), nil
	}
	m.curlExec = func(args ...string) ([]byte, error) {
		return []byte("downloaded"), nil
	}

	upgraded, err := m.upgradeCAPICRDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upgraded {
		t.Error("expected CRDs upgraded via fallback")
	}
}

func TestEnsureAWSSSHKeyPairUsesConfigRegion(t *testing.T) {
	c := fakeSetupClient()
	m := setupManager(c)

	var capturedRegion string
	m.awsExec = func(args ...string) ([]byte, error) {
		for i, arg := range args {
			if arg == "--region" && i+1 < len(args) {
				capturedRegion = args[i+1]
			}
		}
		if args[1] == "describe-key-pairs" {
			return json.Marshal(map[string]interface{}{"KeyPairs": []interface{}{}})
		}
		return []byte("key-id"), nil
	}

	m.EnsureAWSSSHKeyPair(context.Background(), "", "test-key")
	if capturedRegion != "us-east-1" {
		t.Errorf("expected region us-east-1 from config, got %s", capturedRegion)
	}
}
