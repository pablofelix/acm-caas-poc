package policy

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
			client.GVRPolicy:           "PolicyList",
			client.GVRPlacement:        "PlacementList",
			client.GVRPlacementBinding: "PlacementBindingList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestApplyCreatesAllResources(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := PolicyOpts{
		Name:              "test-policy",
		Namespace:         DefaultNamespace,
		RemediationAction: "inform",
		ClusterLabels:     map[string]string{"env": "dev"},
	}
	if err := mgr.Apply(context.Background(), opts); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	ctx := context.Background()
	if _, err := c.Get(ctx, client.GVRPolicy, DefaultNamespace, "test-policy"); err != nil {
		t.Errorf("policy not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRPlacement, DefaultNamespace, "test-policy-placement"); err != nil {
		t.Errorf("placement not created: %v", err)
	}
	if _, err := c.Get(ctx, client.GVRPlacementBinding, DefaultNamespace, "test-policy-placement-binding"); err != nil {
		t.Errorf("placement binding not created: %v", err)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := PolicyOpts{
		Name:      "test-policy",
		Namespace: DefaultNamespace,
	}
	if err := mgr.Apply(context.Background(), opts); err != nil {
		t.Fatalf("first Apply failed: %v", err)
	}
	if err := mgr.Apply(context.Background(), opts); err != nil {
		t.Fatalf("second Apply failed (not idempotent): %v", err)
	}
}

func TestApplyWithRegistries(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := PolicyOpts{
		Name:              "registry-policy",
		Namespace:         DefaultNamespace,
		RemediationAction: "enforce",
		AllowedRegistries: []string{"registry.redhat.io", "quay.io/myorg"},
	}
	if err := mgr.Apply(context.Background(), opts); err != nil {
		t.Fatalf("Apply with registries failed: %v", err)
	}

	obj, err := c.Get(context.Background(), client.GVRPolicy, DefaultNamespace, "registry-policy")
	if err != nil {
		t.Fatalf("policy not created: %v", err)
	}

	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec["remediationAction"] != "enforce" {
		t.Errorf("remediationAction = %v, want enforce", spec["remediationAction"])
	}
}

func TestApplyDefaultsNamespace(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := PolicyOpts{Name: "test-policy"}
	if err := mgr.Apply(context.Background(), opts); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if _, err := c.Get(context.Background(), client.GVRPolicy, DefaultNamespace, "test-policy"); err != nil {
		t.Errorf("policy not in default namespace: %v", err)
	}
}

func TestRemoveDeletesAllResources(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	opts := PolicyOpts{Name: "test-policy", Namespace: DefaultNamespace}
	if err := mgr.Apply(context.Background(), opts); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if err := mgr.Remove(context.Background(), "test-policy", DefaultNamespace); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	ctx := context.Background()
	if _, err := c.Get(ctx, client.GVRPolicy, DefaultNamespace, "test-policy"); err == nil {
		t.Error("policy still exists after remove")
	}
	if _, err := c.Get(ctx, client.GVRPlacement, DefaultNamespace, "test-policy-placement"); err == nil {
		t.Error("placement still exists after remove")
	}
	if _, err := c.Get(ctx, client.GVRPlacementBinding, DefaultNamespace, "test-policy-placement-binding"); err == nil {
		t.Error("placement binding still exists after remove")
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	c := fakeClient()
	mgr := New(c, config.Config{})

	if err := mgr.Remove(context.Background(), "nonexistent", DefaultNamespace); err != nil {
		t.Fatalf("Remove on empty cluster failed (not idempotent): %v", err)
	}
}

func TestListPolicies(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("test-pol")
	pol.SetNamespace(DefaultNamespace)
	pol.Object["spec"] = map[string]interface{}{
		"remediationAction": "enforce",
		"disabled":          false,
	}

	c := fakeClient(pol)
	mgr := New(c, config.Config{})

	policies, err := mgr.List(context.Background(), DefaultNamespace)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("got %d policies, want 1", len(policies))
	}
	if policies[0].Name != "test-pol" {
		t.Errorf("name = %q, want test-pol", policies[0].Name)
	}
	if policies[0].RemediationAction != "enforce" {
		t.Errorf("remediation = %q, want enforce", policies[0].RemediationAction)
	}
}

func TestGetPolicy(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("my-policy")
	pol.SetNamespace(DefaultNamespace)
	pol.Object["spec"] = map[string]interface{}{
		"remediationAction": "inform",
		"disabled":          true,
	}
	pol.Object["status"] = map[string]interface{}{
		"compliant": "NonCompliant",
		"status": []interface{}{
			map[string]interface{}{
				"clustername": "infraops1",
				"compliant":   "NonCompliant",
			},
		},
	}

	c := fakeClient(pol)
	mgr := New(c, config.Config{})

	info, err := mgr.Get(context.Background(), "my-policy", DefaultNamespace)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if info.Compliant != "NonCompliant" {
		t.Errorf("compliant = %q, want NonCompliant", info.Compliant)
	}
	if !info.Disabled {
		t.Error("expected disabled=true")
	}
	if len(info.ClusterCompliance) != 1 {
		t.Fatalf("got %d cluster compliance, want 1", len(info.ClusterCompliance))
	}
	if info.ClusterCompliance[0].ClusterName != "infraops1" {
		t.Errorf("cluster = %q, want infraops1", info.ClusterCompliance[0].ClusterName)
	}
	if info.ClusterCompliance[0].ComplianceState != "NonCompliant" {
		t.Errorf("state = %q, want NonCompliant", info.ClusterCompliance[0].ComplianceState)
	}
}

func TestSetRemediation(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("my-policy")
	pol.SetNamespace(DefaultNamespace)
	pol.Object["spec"] = map[string]interface{}{
		"remediationAction": "inform",
	}

	c := fakeClient(pol)
	mgr := New(c, config.Config{})

	if err := mgr.SetRemediation(context.Background(), "my-policy", DefaultNamespace, "enforce"); err != nil {
		t.Fatalf("SetRemediation failed: %v", err)
	}

	obj, _ := c.Get(context.Background(), client.GVRPolicy, DefaultNamespace, "my-policy")
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec["remediationAction"] != "enforce" {
		t.Errorf("remediationAction = %v, want enforce", spec["remediationAction"])
	}
}

func TestSetDisabled(t *testing.T) {
	pol := &unstructured.Unstructured{}
	pol.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "policy.open-cluster-management.io", Version: "v1", Kind: "Policy",
	})
	pol.SetName("my-policy")
	pol.SetNamespace(DefaultNamespace)
	pol.Object["spec"] = map[string]interface{}{
		"disabled": false,
	}

	c := fakeClient(pol)
	mgr := New(c, config.Config{})

	if err := mgr.SetDisabled(context.Background(), "my-policy", DefaultNamespace, true); err != nil {
		t.Fatalf("SetDisabled failed: %v", err)
	}

	obj, _ := c.Get(context.Background(), client.GVRPolicy, DefaultNamespace, "my-policy")
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec["disabled"] != true {
		t.Errorf("disabled = %v, want true", spec["disabled"])
	}
}

func TestAggregateComplianceMixed(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "test", "namespace": "ns"},
		"spec":     map[string]interface{}{"remediationAction": "inform"},
		"status": map[string]interface{}{
			"status": []interface{}{
				map[string]interface{}{"clustername": "c1", "compliant": "Compliant"},
				map[string]interface{}{"clustername": "c2", "compliant": "NonCompliant"},
			},
		},
	}
	info := parsePolicyInfo(obj)
	if info.Compliant != "NonCompliant" {
		t.Errorf("compliant = %q, want NonCompliant (one cluster non-compliant)", info.Compliant)
	}
}

func TestAggregateComplianceAllCompliant(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "test", "namespace": "ns"},
		"spec":     map[string]interface{}{"remediationAction": "inform"},
		"status": map[string]interface{}{
			"status": []interface{}{
				map[string]interface{}{"clustername": "c1", "compliant": "Compliant"},
				map[string]interface{}{"clustername": "c2", "compliant": "Compliant"},
			},
		},
	}
	info := parsePolicyInfo(obj)
	if info.Compliant != "Compliant" {
		t.Errorf("compliant = %q, want Compliant", info.Compliant)
	}
}

func TestAggregateCompliancePending(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "test", "namespace": "ns"},
		"spec":     map[string]interface{}{"remediationAction": "inform"},
		"status": map[string]interface{}{
			"status": []interface{}{
				map[string]interface{}{"clustername": "c1"},
			},
		},
	}
	info := parsePolicyInfo(obj)
	if info.Compliant != "Pending" {
		t.Errorf("compliant = %q, want Pending", info.Compliant)
	}
}

func TestParsePolicyInfoNoStatus(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "ns",
		},
		"spec": map[string]interface{}{
			"remediationAction": "inform",
			"disabled":          false,
		},
	}
	info := parsePolicyInfo(obj)
	if info.Compliant != "" {
		t.Errorf("compliant = %q, want empty", info.Compliant)
	}
	if len(info.ClusterCompliance) != 0 {
		t.Errorf("got %d cluster compliance, want 0", len(info.ClusterCompliance))
	}
}
