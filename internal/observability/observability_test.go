package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func fakeClient(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRNamespace:                 "NamespaceList",
			client.GVRPersistentVolumeClaim:     "PersistentVolumeClaimList",
			client.GVRDeployment:                "DeploymentList",
			client.GVRService:                   "ServiceList",
			client.GVRSecret:                    "SecretList",
			client.GVRMultiClusterObservability: "MultiClusterObservabilityList",
			client.GVRConfigMap:                 "ConfigMapList",
			client.GVRManagedClusterAddOn:       "ManagedClusterAddOnList",
			client.GVRObjectBucketClaim:         "ObjectBucketClaimList",
			client.GVRManagedCluster:            "ManagedClusterList",
			client.GVRRoute:                     "RouteList",
			client.GVRStatefulSet:               "StatefulSetList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestSetupCreatesAllResources(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

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
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.Setup(context.Background()); err != nil {
		t.Fatalf("first Setup failed: %v", err)
	}
	if err := mgr.Setup(context.Background()); err != nil {
		t.Fatalf("second Setup failed (not idempotent): %v", err)
	}
}

func TestTeardownRemovesAllResources(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

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
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown on empty cluster failed (not idempotent): %v", err)
	}
}

func TestStatusNotInstalled(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

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
	mgr := New(c, config.Config{}, discardLogger)

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
	mgr := New(c, config.Config{}, discardLogger)

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
	mgr := New(c, config.Config{}, discardLogger)

	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status != "Progressing" {
		t.Errorf("status = %q, want Progressing", status)
	}
}

func TestStatusGetError(t *testing.T) {
	c := fakeClient()
	fake := c.Dynamic.(*dynamicfake.FakeDynamicClient)
	fake.PrependReactor("get", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api unavailable")
	})
	mgr := New(c, config.Config{}, discardLogger)

	_, err := mgr.Status(context.Background())
	if err == nil {
		t.Fatal("expected error from Status when API fails")
	}
}

func TestSetupStepError(t *testing.T) {
	c := fakeClient()
	fake := c.Dynamic.(*dynamicfake.FakeDynamicClient)
	fake.PrependReactor("create", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("create blocked")
	})
	fake.PrependReactor("get", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("get blocked")
	})
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.Setup(context.Background())
	if err == nil {
		t.Fatal("expected error from Setup when API fails")
	}
}

func TestTeardownStepError(t *testing.T) {
	c := fakeClient()
	fake := c.Dynamic.(*dynamicfake.FakeDynamicClient)
	fake.PrependReactor("delete", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("delete blocked")
	})
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.Teardown(context.Background())
	if err == nil {
		t.Fatal("expected error from Teardown when delete fails")
	}
}

func TestConfigurePullSecret(t *testing.T) {
	src := &unstructured.Unstructured{}
	src.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"})
	src.SetName("pull-secret")
	src.SetNamespace(PullSecretSourceNS)
	src.Object["type"] = "kubernetes.io/dockerconfigjson"
	src.Object["data"] = map[string]interface{}{".dockerconfigjson": "dGVzdA=="}

	c := fakeClient(src)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.ensureNamespace(context.Background()); err != nil {
		t.Fatalf("namespace: %v", err)
	}
	if err := mgr.ConfigurePullSecret(context.Background()); err != nil {
		t.Fatalf("ConfigurePullSecret failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRSecret, Namespace, PullSecretName); err != nil {
		t.Errorf("pull secret not found: %v", err)
	}
}

func TestConfigureOBCStorage(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.ConfigureOBCStorage(context.Background(), StorageOpts{Type: "obc", StorageClass: "gp3-csi"}); err != nil {
		t.Fatalf("ConfigureOBCStorage failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRObjectBucketClaim, Namespace, OBCName); err != nil {
		t.Errorf("OBC not found: %v", err)
	}
}

func TestDeployCustomRules(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	rules := "groups:\n- name: custom\n  rules:\n  - alert: HighCPU\n    expr: cpu > 80"
	if err := mgr.DeployCustomRules(context.Background(), CustomRuleOpts{Rules: rules}); err != nil {
		t.Fatalf("DeployCustomRules failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRConfigMap, Namespace, CustomRulesCM); err != nil {
		t.Errorf("custom rules CM not found: %v", err)
	}
}

func TestRemoveCustomRules(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	_ = mgr.DeployCustomRules(context.Background(), CustomRuleOpts{Rules: "groups:\n- name: temp\n  rules: []"})
	if err := mgr.RemoveCustomRules(context.Background()); err != nil {
		t.Fatalf("RemoveCustomRules failed: %v", err)
	}
	_, err := c.Get(context.Background(), client.GVRConfigMap, Namespace, CustomRulesCM)
	if err == nil {
		t.Error("custom rules CM still exists")
	}
}

func TestDeployDashboard(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.DeployDashboard(context.Background(), DashboardOpts{Name: "gpu-overview", JSON: `{"title":"GPU"}`}); err != nil {
		t.Fatalf("DeployDashboard failed: %v", err)
	}
	obj, err := c.Get(context.Background(), client.GVRConfigMap, Namespace, "gpu-overview")
	if err != nil {
		t.Fatalf("dashboard CM not found: %v", err)
	}
	labels := obj.GetLabels()
	if labels[DashboardLabelKey] != DashboardLabelValue {
		t.Errorf("missing dashboard label")
	}
}

func TestRemoveDashboard(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	_ = mgr.DeployDashboard(context.Background(), DashboardOpts{Name: "temp", JSON: "{}"})
	if err := mgr.RemoveDashboard(context.Background(), "temp"); err != nil {
		t.Fatalf("RemoveDashboard failed: %v", err)
	}
}

func TestConfigureMetricsAllowlist(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.ConfigureMetricsAllowlist(context.Background(), MetricsOpts{Metrics: []string{"node_cpu_seconds_total", "container_memory_rss"}}); err != nil {
		t.Fatalf("ConfigureMetricsAllowlist failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRConfigMap, Namespace, MetricsAllowlistCM); err != nil {
		t.Errorf("metrics allowlist CM not found: %v", err)
	}
}

func TestListAddonHealth(t *testing.T) {
	addon := &unstructured.Unstructured{}
	addon.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "addon.open-cluster-management.io", Version: "v1alpha1", Kind: "ManagedClusterAddOn",
	})
	addon.SetName("observability-controller")
	addon.SetNamespace("spoke1")
	addon.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Available", "status": "True"},
			map[string]interface{}{"type": "Degraded", "status": "False"},
		},
	}

	c := fakeClient(addon)
	mgr := New(c, config.Config{}, discardLogger)

	health, err := mgr.ListAddonHealth(context.Background())
	if err != nil {
		t.Fatalf("ListAddonHealth failed: %v", err)
	}
	if len(health) != 1 {
		t.Fatalf("got %d addons, want 1", len(health))
	}
	if !health[0].Available {
		t.Error("expected Available=true")
	}
	if health[0].Degraded {
		t.Error("expected Degraded=false")
	}
}

func TestListAddonHealthEmpty(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	health, err := mgr.ListAddonHealth(context.Background())
	if err != nil {
		t.Fatalf("ListAddonHealth failed: %v", err)
	}
	if len(health) != 0 {
		t.Errorf("got %d, want 0", len(health))
	}
}

func TestConfigureRetention(t *testing.T) {
	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Kind: "MultiClusterObservability",
	})
	mco.SetName(MCOName)
	mco.Object["spec"] = map[string]interface{}{}

	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.ConfigureRetention(context.Background(), RetentionOpts{RetentionInLocal: "24h", BlockDuration: "2h", DeleteDelay: "48h"})
	if err != nil {
		t.Fatalf("ConfigureRetention failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRMultiClusterObservability, "", MCOName)
	spec := obj.Object["spec"].(map[string]interface{})
	ret := spec["retentionConfig"].(map[string]interface{})
	if ret["retentionInLocal"] != "24h" {
		t.Errorf("retentionInLocal = %v, want 24h", ret["retentionInLocal"])
	}
}

func TestStatusConditionBadType(t *testing.T) {
	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Kind: "MultiClusterObservability",
	})
	mco.SetName(MCOName)
	mco.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			"not-a-map",
			map[string]interface{}{
				"type":   "SomethingElse",
				"status": "True",
			},
		},
	}

	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status != "Progressing" {
		t.Errorf("status = %q, want Progressing (no Ready condition)", status)
	}
}

func TestDisableCluster(t *testing.T) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("spoke1")
	mc.SetLabels(map[string]string{"vendor": "OpenShift"})

	c := fakeClient(mc)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.DisableCluster(context.Background(), "spoke1"); err != nil {
		t.Fatalf("DisableCluster failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRManagedCluster, "", "spoke1")
	labels := obj.GetLabels()
	if labels["observability"] != "disabled" {
		t.Error("expected observability=disabled label")
	}
	if labels["vendor"] != "OpenShift" {
		t.Error("existing label vendor=OpenShift was not preserved")
	}
}

func TestDisableClusterIdempotent(t *testing.T) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("spoke1")
	mc.SetLabels(map[string]string{"observability": "disabled"})

	c := fakeClient(mc)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.DisableCluster(context.Background(), "spoke1"); err != nil {
		t.Fatalf("DisableCluster (idempotent) failed: %v", err)
	}
}

func TestEnableCluster(t *testing.T) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("spoke1")
	mc.SetLabels(map[string]string{"vendor": "OpenShift", "observability": "disabled"})

	c := fakeClient(mc)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.EnableCluster(context.Background(), "spoke1"); err != nil {
		t.Fatalf("EnableCluster failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRManagedCluster, "", "spoke1")
	labels := obj.GetLabels()
	if _, exists := labels["observability"]; exists {
		t.Error("observability label should be removed")
	}
	if labels["vendor"] != "OpenShift" {
		t.Error("existing label vendor=OpenShift was not preserved")
	}
}

func TestEnableClusterAlreadyEnabled(t *testing.T) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("spoke1")

	c := fakeClient(mc)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.EnableCluster(context.Background(), "spoke1"); err != nil {
		t.Fatalf("EnableCluster (already enabled) failed: %v", err)
	}
}

func TestGrafanaURL(t *testing.T) {
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "route.openshift.io", Version: "v1", Kind: "Route",
	})
	route.SetName("grafana")
	route.SetNamespace(Namespace)
	route.Object["spec"] = map[string]interface{}{
		"host": "grafana.apps.example.com",
	}

	c := fakeClient(route)
	mgr := New(c, config.Config{}, discardLogger)

	url, err := mgr.GrafanaURL(context.Background())
	if err != nil {
		t.Fatalf("GrafanaURL failed: %v", err)
	}
	if url != "https://grafana.apps.example.com" {
		t.Errorf("url = %q, want https://grafana.apps.example.com", url)
	}
}

func TestGrafanaURLNotFound(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	_, err := mgr.GrafanaURL(context.Background())
	if err == nil {
		t.Fatal("expected error when no grafana route exists")
	}
	if !strings.Contains(err.Error(), "grafana route not found") {
		t.Errorf("error message should mention grafana route not found, got: %v", err)
	}
}

func TestValidateRulesYAML(t *testing.T) {
	valid := "groups:\n- name: test\n  rules:\n  - alert: HighCPU\n    expr: cpu > 80"
	if err := ValidateRulesYAML(valid); err != nil {
		t.Errorf("valid rules rejected: %v", err)
	}
}

func TestValidateRulesYAMLMissingGroups(t *testing.T) {
	invalid := "rules:\n- alert: HighCPU\n  expr: cpu > 80"
	err := ValidateRulesYAML(invalid)
	if err == nil {
		t.Fatal("expected error for missing groups")
	}
	if !strings.Contains(err.Error(), "groups") {
		t.Errorf("error should mention groups, got: %v", err)
	}
}

func TestValidateRulesYAMLInvalidYAML(t *testing.T) {
	if err := ValidateRulesYAML(":::bad yaml"); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidateDashboardJSON(t *testing.T) {
	if err := ValidateDashboardJSON(`{"title":"GPU"}`); err != nil {
		t.Errorf("valid JSON rejected: %v", err)
	}
	if err := ValidateDashboardJSON(`not json`); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateAlertmanagerConfig(t *testing.T) {
	valid := "route:\n  receiver: default\nreceivers:\n- name: default"
	if err := ValidateAlertmanagerConfig(valid); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}

func TestValidateAlertmanagerConfigMissingReceivers(t *testing.T) {
	invalid := "route:\n  receiver: default"
	err := ValidateAlertmanagerConfig(invalid)
	if err == nil {
		t.Fatal("expected error for missing receivers")
	}
	if !strings.Contains(err.Error(), "receivers") {
		t.Errorf("error should mention receivers, got: %v", err)
	}
}

func TestConfigureAlertmanager(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	cfg := "route:\n  receiver: default\nreceivers:\n- name: default"
	if err := mgr.ConfigureAlertmanager(context.Background(), AlertmanagerOpts{Config: cfg}); err != nil {
		t.Fatalf("ConfigureAlertmanager failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRSecret, Namespace, AlertmanagerSecretName); err != nil {
		t.Errorf("alertmanager secret not found: %v", err)
	}
}

func TestConfigureAlertmanagerInvalid(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.ConfigureAlertmanager(context.Background(), AlertmanagerOpts{Config: "invalid: true"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func newMCO() *unstructured.Unstructured {
	mco := &unstructured.Unstructured{}
	mco.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "observability.open-cluster-management.io", Version: "v1beta2", Kind: "MultiClusterObservability",
	})
	mco.SetName(MCOName)
	mco.Object["spec"] = map[string]interface{}{}
	return mco
}

func TestDisableAlertForwarding(t *testing.T) {
	mco := newMCO()
	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.DisableAlertForwarding(context.Background()); err != nil {
		t.Fatalf("DisableAlertForwarding failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRMultiClusterObservability, "", MCOName)
	annotations := obj.GetAnnotations()
	if annotations["mco-disable-alerting"] != "true" {
		t.Error("expected mco-disable-alerting annotation")
	}
}

func TestEnableAlertForwarding(t *testing.T) {
	mco := newMCO()
	mco.SetAnnotations(map[string]string{"mco-disable-alerting": "true"})
	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.EnableAlertForwarding(context.Background()); err != nil {
		t.Fatalf("EnableAlertForwarding failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRMultiClusterObservability, "", MCOName)
	annotations := obj.GetAnnotations()
	if _, exists := annotations["mco-disable-alerting"]; exists {
		t.Error("mco-disable-alerting annotation should be removed")
	}
}

func TestConfigureAdvanced(t *testing.T) {
	mco := newMCO()
	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	replicas := int64(6)
	interval := int64(30)
	if err := mgr.ConfigureAdvanced(context.Background(), AdvancedOpts{
		ReceiveReplicas:    &replicas,
		CollectionInterval: &interval,
	}); err != nil {
		t.Fatalf("ConfigureAdvanced failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRMultiClusterObservability, "", MCOName)
	spec := obj.Object["spec"].(map[string]interface{})
	advanced := spec["advanced"].(map[string]interface{})
	receive := advanced["receive"].(map[string]interface{})
	if receive["replicas"] != int64(6) {
		t.Errorf("replicas = %v, want 6", receive["replicas"])
	}
	addon := spec["observabilityAddonSpec"].(map[string]interface{})
	if addon["interval"] != int64(30) {
		t.Errorf("interval = %v, want 30", addon["interval"])
	}
}

func TestConfigureAdvancedPreservesExisting(t *testing.T) {
	mco := newMCO()
	mco.Object["spec"].(map[string]interface{})["advanced"] = map[string]interface{}{
		"receive": map[string]interface{}{"replicas": int64(3)},
		"query":   map[string]interface{}{"replicas": int64(2)},
	}
	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	ds := true
	if err := mgr.ConfigureAdvanced(context.Background(), AdvancedOpts{Downsampling: &ds}); err != nil {
		t.Fatalf("ConfigureAdvanced failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRMultiClusterObservability, "", MCOName)
	spec := obj.Object["spec"].(map[string]interface{})
	if spec["enableDownsampling"] != true {
		t.Error("downsampling should be true")
	}
	advanced := spec["advanced"].(map[string]interface{})
	query := advanced["query"].(map[string]interface{})
	if query["replicas"] != int64(2) {
		t.Error("query replicas should be preserved")
	}
}

func TestConfigureMetricsWorkloadError(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.ConfigureMetrics(context.Background(), MetricsConfigOpts{
		Scope:   MetricsScopeWorkload,
		Metrics: []string{"my_app_requests_total"},
	})
	if err == nil {
		t.Fatal("expected error for workload-scoped metrics")
	}
	if !strings.Contains(err.Error(), "managed cluster") {
		t.Errorf("error should mention managed cluster, got: %v", err)
	}
}

func TestConfigureMetricsClusterError(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.ConfigureMetrics(context.Background(), MetricsConfigOpts{
		Scope:   MetricsScopeCluster,
		Cluster: "spoke1",
		Metrics: []string{"foo"},
	})
	if err == nil {
		t.Fatal("expected error for cluster-scoped metrics")
	}
	if !strings.Contains(err.Error(), "direct access") {
		t.Errorf("error should mention direct access, got: %v", err)
	}
}

func TestVerify(t *testing.T) {
	mco := newMCO()
	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	result, err := mgr.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.MCOStatus != "Pending" {
		t.Errorf("MCOStatus = %q, want Pending", result.MCOStatus)
	}
	if !result.PVCsBound {
		t.Error("PVCsBound should be true when no PVCs exist")
	}
}

func TestVerifyWithWorkloads(t *testing.T) {
	mco := newMCO()
	dep := &unstructured.Unstructured{}
	dep.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	dep.SetName("observability-observatorium-api")
	dep.SetNamespace(Namespace)
	dep.Object["spec"] = map[string]interface{}{
		"replicas": int64(1),
	}
	dep.Object["status"] = map[string]interface{}{
		"availableReplicas": int64(1),
	}

	c := fakeClient(mco, dep)
	mgr := New(c, config.Config{}, discardLogger)

	result, err := mgr.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if len(result.Workloads) == 0 {
		t.Fatal("expected at least one workload")
	}
	if !result.Workloads[0].Ready {
		t.Error("workload should be ready")
	}
}

func TestConfigureRetentionPreservesExisting(t *testing.T) {
	mco := newMCO()
	mco.Object["spec"].(map[string]interface{})["retentionConfig"] = map[string]interface{}{
		"retentionInLocal": "24h",
		"blockDuration":    "2h",
	}
	c := fakeClient(mco)
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.ConfigureRetention(context.Background(), RetentionOpts{DeleteDelay: "48h"}); err != nil {
		t.Fatalf("ConfigureRetention failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRMultiClusterObservability, "", MCOName)
	spec := obj.Object["spec"].(map[string]interface{})
	ret := spec["retentionConfig"].(map[string]interface{})
	if ret["retentionInLocal"] != "24h" {
		t.Error("retentionInLocal should be preserved")
	}
	if ret["blockDuration"] != "2h" {
		t.Error("blockDuration should be preserved")
	}
	if ret["deleteDelay"] != "48h" {
		t.Error("deleteDelay should be 48h")
	}
}

func TestDeployCustomRulesUpdate(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	rules1 := "groups:\n- name: v1\n  rules: []"
	if err := mgr.DeployCustomRules(context.Background(), CustomRuleOpts{Rules: rules1}); err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}
	rules2 := "groups:\n- name: v2\n  rules: []"
	if err := mgr.DeployCustomRules(context.Background(), CustomRuleOpts{Rules: rules2}); err != nil {
		t.Fatalf("update deploy failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRConfigMap, Namespace, CustomRulesCM)
	data := obj.Object["data"].(map[string]interface{})
	content := data["custom_rules.yaml"].(string)
	if !strings.Contains(content, "v2") {
		t.Error("expected updated rules content")
	}
}

func TestConfigureMetricsAllowlistFromYAML(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	yamlContent := "names:\n  - node_cpu_seconds_total\n"
	if err := mgr.ConfigureMetricsAllowlistFromYAML(context.Background(), yamlContent); err != nil {
		t.Fatalf("ConfigureMetricsAllowlistFromYAML failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRConfigMap, Namespace, MetricsAllowlistCM); err != nil {
		t.Errorf("metrics CM not found: %v", err)
	}
}

func TestVerifyWithPVCsNotBound(t *testing.T) {
	mco := newMCO()
	pvc := &unstructured.Unstructured{}
	pvc.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"})
	pvc.SetName("data-pvc")
	pvc.SetNamespace(Namespace)
	pvc.Object["status"] = map[string]interface{}{
		"phase": "Pending",
	}

	c := fakeClient(mco, pvc)
	mgr := New(c, config.Config{}, discardLogger)

	result, err := mgr.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.PVCsBound {
		t.Error("PVCsBound should be false when PVC is Pending")
	}
}

func TestVerifyWithStatefulSets(t *testing.T) {
	mco := newMCO()
	sts := &unstructured.Unstructured{}
	sts.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"})
	sts.SetName("observability-thanos-receive")
	sts.SetNamespace(Namespace)
	sts.Object["spec"] = map[string]interface{}{
		"replicas": int64(3),
	}
	sts.Object["status"] = map[string]interface{}{
		"readyReplicas": int64(3),
	}

	c := fakeClient(mco, sts)
	mgr := New(c, config.Config{}, discardLogger)

	result, err := mgr.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	found := false
	for _, w := range result.Workloads {
		if w.Name == "observability-thanos-receive" {
			found = true
			if !w.Ready {
				t.Error("sts should be ready")
			}
		}
	}
	if !found {
		t.Error("expected statefulset in workloads")
	}
}

func TestVerifyWithAddonHealth(t *testing.T) {
	mco := newMCO()
	addon := &unstructured.Unstructured{}
	addon.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "addon.open-cluster-management.io", Version: "v1alpha1", Kind: "ManagedClusterAddOn",
	})
	addon.SetName("observability-controller")
	addon.SetNamespace("spoke1")
	addon.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{"type": "Available", "status": "True"},
		},
	}

	c := fakeClient(mco, addon)
	mgr := New(c, config.Config{}, discardLogger)

	result, err := mgr.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if len(result.AddonHealth) != 1 {
		t.Fatalf("expected 1 addon health, got %d", len(result.AddonHealth))
	}
}

func TestConfigureMetricsGlobal(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{}, discardLogger)

	if err := mgr.ConfigureMetrics(context.Background(), MetricsConfigOpts{
		Scope:   MetricsScopeGlobal,
		Metrics: []string{"node_cpu_seconds_total"},
	}); err != nil {
		t.Fatalf("ConfigureMetrics (global) failed: %v", err)
	}
	obj, _ := c.Get(context.Background(), client.GVRConfigMap, Namespace, MetricsAllowlistCM)
	data := obj.Object["data"].(map[string]interface{})
	if _, ok := data["metrics_list.yaml"]; !ok {
		t.Error("expected metrics_list.yaml key")
	}
}

func TestListAddonHealthNilStatus(t *testing.T) {
	addon := &unstructured.Unstructured{}
	addon.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "addon.open-cluster-management.io", Version: "v1alpha1", Kind: "ManagedClusterAddOn",
	})
	addon.SetName("observability-controller")
	addon.SetNamespace("spoke1")

	c := fakeClient(addon)
	mgr := New(c, config.Config{}, discardLogger)

	health, err := mgr.ListAddonHealth(context.Background())
	if err != nil {
		t.Fatalf("ListAddonHealth failed: %v", err)
	}
	if len(health) != 1 {
		t.Fatalf("got %d, want 1", len(health))
	}
	if health[0].Available {
		t.Error("should not be available with nil status")
	}
}

func TestVerifyPVCWithoutStatus(t *testing.T) {
	mco := newMCO()
	pvc := &unstructured.Unstructured{}
	pvc.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"})
	pvc.SetName("no-status-pvc")
	pvc.SetNamespace(Namespace)

	c := fakeClient(mco, pvc)
	mgr := New(c, config.Config{}, discardLogger)

	result, err := mgr.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if result.PVCsBound {
		t.Error("PVCsBound should be false when PVC has no status")
	}
}

func TestMergeMetricsKeyGetError(t *testing.T) {
	c := fakeClient()
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("get", "configmaps", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api error")
	})
	mgr := New(c, config.Config{}, discardLogger)

	err := mgr.ConfigureMetrics(context.Background(), MetricsConfigOpts{
		Scope:   MetricsScopeGlobal,
		Metrics: []string{"test_metric"},
	})
	if err == nil {
		t.Fatal("expected error when get fails")
	}
}

func TestReviewPreserveMetricsKeys(t *testing.T) {
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      MetricsAllowlistCM,
				"namespace": Namespace,
			},
			"data": map[string]interface{}{
				"metrics_list.yaml":     "names:\n  - existing_platform_metric\n",
				"uwl_metrics_list.yaml": "names:\n  - existing_workload_metric\n",
			},
		},
	}
	cm.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	c := fakeClient(cm)
	m := New(c, config.Config{}, discardLogger)
	ctx := context.Background()
	if err := m.ConfigureMetricsAllowlist(ctx, MetricsOpts{Metrics: []string{"new_platform_metric"}}); err != nil {
		t.Fatal(err)
	}
	obj, err := c.Get(ctx, client.GVRConfigMap, Namespace, MetricsAllowlistCM)
	if err != nil {
		t.Fatal(err)
	}
	data := obj.Object["data"].(map[string]interface{})
	if _, ok := data["uwl_metrics_list.yaml"]; !ok {
		t.Fatal("ConfigureMetricsAllowlist erased uwl_metrics_list.yaml key")
	}
	if _, ok := data["metrics_list.yaml"]; !ok {
		t.Fatal("metrics_list.yaml key missing after update")
	}
}

func TestReviewVerifyReportsReadFailure(t *testing.T) {
	c := fakeClient(newMCO())
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("list", "persistentvolumeclaims", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("forbidden")
	})
	r, err := New(c, config.Config{}, discardLogger).Verify(context.Background())
	if err == nil {
		t.Fatalf("read error swallowed: PVCsBound=%v", r.PVCsBound)
	}
}

func TestReviewVerifyDesiredReplicas(t *testing.T) {
	d := &unstructured.Unstructured{}
	d.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	d.SetName("observability-test")
	d.SetNamespace(Namespace)
	d.Object["spec"] = map[string]interface{}{"replicas": int64(3)}
	d.Object["status"] = map[string]interface{}{"replicas": int64(1), "availableReplicas": int64(1)}
	r, err := New(fakeClient(newMCO(), d), config.Config{}, discardLogger).Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Workloads[0].Ready {
		t.Fatal("deployment marked ready with only 1 of 3 desired replicas")
	}
}

func TestReviewVerifyStaleGeneration(t *testing.T) {
	d := &unstructured.Unstructured{}
	d.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	d.SetName("observability-stale")
	d.SetNamespace(Namespace)
	d.SetGeneration(3)
	d.Object["spec"] = map[string]interface{}{"replicas": int64(2)}
	d.Object["status"] = map[string]interface{}{
		"observedGeneration": int64(2),
		"availableReplicas":  int64(2),
	}
	r, err := New(fakeClient(newMCO(), d), config.Config{}, discardLogger).Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Workloads[0].Ready {
		t.Fatal("deployment marked ready but observedGeneration < generation")
	}
}

func TestReviewObservedButIncompleteRollout(t *testing.T) {
	obj := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata":  map[string]interface{}{"name": "observability-test", "generation": int64(2)},
		"spec":      map[string]interface{}{"replicas": int64(3)},
		"status":    map[string]interface{}{"observedGeneration": int64(2), "availableReplicas": int64(3), "updatedReplicas": int64(0)},
	}}
	if workloadFromUnstructured(obj).Ready {
		t.Fatal("Ready=true with 0/3 updated replicas despite observed generation")
	}
}

func TestReviewInvalidRulesRejectedByDeploy(t *testing.T) {
	err := New(fakeClient(), config.Config{}, discardLogger).DeployCustomRules(context.Background(), CustomRuleOpts{Rules: "groups: ["})
	if err == nil {
		t.Fatal("DeployCustomRules accepted syntactically invalid YAML")
	}
}

func TestReviewInvalidDashboardRejectedByDeploy(t *testing.T) {
	err := New(fakeClient(), config.Config{}, discardLogger).DeployDashboard(context.Background(), DashboardOpts{Name: "bad-dashboard", JSON: "not json"})
	if err == nil {
		t.Fatal("DeployDashboard accepted invalid JSON")
	}
}
