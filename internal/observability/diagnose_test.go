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

func diagnoseFakeClient(objs ...runtime.Object) *client.Client {
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
			client.GVRMultiClusterHub:           "MultiClusterHubList",
			client.GVRMultiClusterEngine:        "MultiClusterEngineList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestDiagnoseHealthyStack(t *testing.T) {
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]interface{}{"name": "spoke1"},
	}}
	addon := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "addon.open-cluster-management.io/v1alpha1",
		"kind":       "ManagedClusterAddOn",
		"metadata":   map[string]interface{}{"name": "observability-controller", "namespace": "spoke1"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}

	c := diagnoseFakeClient(mco, pullSecret, mch, mce, cluster, addon)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if !result.Healthy {
		for _, check := range result.Checks {
			if check.Status != "pass" {
				t.Errorf("check %s: %s — %s", check.Name, check.Status, check.Message)
			}
		}
	}
}

func TestDiagnoseMissingPullSecret(t *testing.T) {
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}

	c := diagnoseFakeClient(mco, mch, mce)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if result.Healthy {
		t.Error("expected unhealthy result when pull secret is missing")
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "pull-secret" && check.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected pull-secret check to fail")
	}
}

func TestDiagnoseDisabledCluster(t *testing.T) {
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata": map[string]interface{}{
			"name":   "spoke1",
			"labels": map[string]interface{}{"observability": "disabled"},
		},
	}}

	c := diagnoseFakeClient(mco, pullSecret, mch, mce, cluster)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if result.Healthy {
		t.Error("expected unhealthy result when cluster is disabled")
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "disabled-clusters" && check.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected disabled-clusters check to fail")
	}
}

func TestDiagnoseMCHNotComplete(t *testing.T) {
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "False", "message": "Not all hub components ready."},
			},
		},
	}}
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}

	c := diagnoseFakeClient(mco, pullSecret, mch, mce)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if result.Healthy {
		t.Error("expected unhealthy when MCH is not complete")
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "mch-complete" && check.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected mch-complete check to fail")
	}
}

func TestRepairCreatesPullSecretAndEnablesCluster(t *testing.T) {
	srcSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": "pull-secret", "namespace": PullSecretSourceNS},
		"type":     "kubernetes.io/dockerconfigjson",
		"data":     map[string]interface{}{".dockerconfigjson": "eyJ0ZXN0IjogdHJ1ZX0="},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata": map[string]interface{}{
			"name":   "spoke1",
			"labels": map[string]interface{}{"observability": "disabled", "vendor": "OpenShift"},
		},
	}}

	c := diagnoseFakeClient(srcSecret, cluster)
	mgr := New(c, config.Config{}, discardLogger)
	actions, err := mgr.Repair(context.Background())
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d: %v", len(actions), actions)
	}

	ctx := context.Background()
	if _, err := c.Get(ctx, client.GVRSecret, Namespace, PullSecretName); err != nil {
		t.Error("pull secret not created")
	}
	updated, err := c.Get(ctx, client.GVRManagedCluster, "", "spoke1")
	if err != nil {
		t.Fatalf("getting cluster: %v", err)
	}
	labels := updated.GetLabels()
	if labels["observability"] == "disabled" {
		t.Error("cluster still has observability=disabled")
	}
	if labels["vendor"] != "OpenShift" {
		t.Error("vendor label was lost")
	}
}

func TestDiagnoseMCENotAvailable(t *testing.T) {
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "False", "message": "Not all components available"},
			},
		},
	}}
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}

	c := diagnoseFakeClient(mce, mco, pullSecret)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	if result.Healthy {
		t.Error("expected unhealthy when MCE is not available")
	}
}

func TestDiagnoseMCONotInstalled(t *testing.T) {
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}

	c := diagnoseFakeClient(mce, mch, pullSecret)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "mco-status" && check.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected mco-status fail when MCO not installed")
	}
}

func TestDiagnoseMCOProgressing(t *testing.T) {
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Progressing", "status": "True"},
			},
		},
	}}

	c := diagnoseFakeClient(mce, mch, pullSecret, mco)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "mco-status" && check.Status == "warn" {
			found = true
		}
	}
	if !found {
		t.Error("expected mco-status warn when MCO is progressing")
	}
}

func TestDiagnoseMissingAddon(t *testing.T) {
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]interface{}{"name": "spoke1"},
	}}

	c := diagnoseFakeClient(mce, mch, pullSecret, mco, cluster)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "addon-health" && check.Status == "fail" {
			found = true
		}
	}
	if !found {
		t.Error("expected addon-health fail when cluster has no addon")
	}
}

func TestDiagnoseDegradedAddon(t *testing.T) {
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]interface{}{"name": "spoke1"},
	}}
	addon := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "addon.open-cluster-management.io/v1alpha1",
		"kind":       "ManagedClusterAddOn",
		"metadata":   map[string]interface{}{"name": "observability-controller", "namespace": "spoke1"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
				map[string]interface{}{"type": "Degraded", "status": "True"},
			},
		},
	}}

	c := diagnoseFakeClient(mce, mch, pullSecret, mco, cluster, addon)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "addon-health" && check.Status == "warn" {
			found = true
		}
	}
	if !found {
		t.Error("expected addon-health warn when addon is degraded")
	}
}

func TestDiagnoseNoMCEFound(t *testing.T) {
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}

	c := diagnoseFakeClient(mco, mch, pullSecret)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	for _, check := range result.Checks {
		if check.Name == "mce-available" && check.Status != "skip" {
			t.Errorf("expected skip for mce-available when no MCE, got %s", check.Status)
		}
	}
}

func TestDiagnoseNoMCHFound(t *testing.T) {
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}

	c := diagnoseFakeClient(mco, mce, pullSecret)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	for _, check := range result.Checks {
		if check.Name == "mch-complete" && check.Status != "skip" {
			t.Errorf("expected skip for mch-complete when no MCH, got %s", check.Status)
		}
	}
}

func TestDiagnoseAddonUnavailable(t *testing.T) {
	mce := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "multicluster.openshift.io/v1",
		"kind":       "MultiClusterEngine",
		"metadata":   map[string]interface{}{"name": "multiclusterengine"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "True"},
			},
		},
	}}
	mch := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operator.open-cluster-management.io/v1",
		"kind":       "MultiClusterHub",
		"metadata":   map[string]interface{}{"name": "multiclusterhub", "namespace": "open-cluster-management"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Complete", "status": "True"},
			},
		},
	}}
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	mco := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "observability.open-cluster-management.io/v1beta2",
		"kind":       "MultiClusterObservability",
		"metadata":   map[string]interface{}{"name": MCOName},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]interface{}{"name": "spoke1"},
	}}
	addon := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "addon.open-cluster-management.io/v1alpha1",
		"kind":       "ManagedClusterAddOn",
		"metadata":   map[string]interface{}{"name": "observability-controller", "namespace": "spoke1"},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Available", "status": "False"},
			},
		},
	}}

	c := diagnoseFakeClient(mce, mch, pullSecret, mco, cluster, addon)
	mgr := New(c, config.Config{}, discardLogger)
	result, err := mgr.Diagnose(context.Background())
	if err != nil {
		t.Fatalf("Diagnose failed: %v", err)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "addon-health" && check.Status == "warn" {
			found = true
		}
	}
	if !found {
		t.Error("expected addon-health warn when addon is unavailable")
	}
}

func TestConditionStatusMissingCondition(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Other", "status": "True"},
			},
		},
	}
	status, _ := conditionStatus(obj, "Ready")
	if status != "Unknown" {
		t.Errorf("expected Unknown for missing condition, got %s", status)
	}
}

func TestConditionStatusNilStatus(t *testing.T) {
	obj := map[string]interface{}{}
	status, _ := conditionStatus(obj, "Ready")
	if status != "Unknown" {
		t.Errorf("expected Unknown for nil status, got %s", status)
	}
}

func TestRepairNoActionWhenHealthy(t *testing.T) {
	pullSecret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": PullSecretName, "namespace": Namespace},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata":   map[string]interface{}{"name": "spoke1"},
	}}

	c := diagnoseFakeClient(pullSecret, cluster)
	mgr := New(c, config.Config{}, discardLogger)
	actions, err := mgr.Repair(context.Background())
	if err != nil {
		t.Fatalf("Repair failed: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d: %v", len(actions), actions)
	}
}
