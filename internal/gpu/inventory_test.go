package gpu

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestListGPUClustersEmpty(t *testing.T) {
	mgr := newTestManager()
	clusters, err := mgr.ListGPUClusters(context.Background())
	if err != nil {
		t.Fatalf("ListGPUClusters failed: %v", err)
	}
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}

func TestListGPUClustersReturnsGPUOnly(t *testing.T) {
	gpu := gpuReadyCluster("gpu1", "H100")
	plain := managedCluster("worker1", map[string]string{"env": "prod"})
	mgr := newTestManager(gpu, plain)

	clusters, err := mgr.ListGPUClusters(context.Background())
	if err != nil {
		t.Fatalf("ListGPUClusters failed: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 GPU cluster, got %d", len(clusters))
	}
	if clusters[0].Cluster != "gpu1" {
		t.Errorf("cluster = %q, want gpu1", clusters[0].Cluster)
	}
	if clusters[0].GPUType != "H100" {
		t.Errorf("gpuType = %q, want H100", clusters[0].GPUType)
	}
	if !clusters[0].Fit {
		t.Errorf("expected Fit=true, got reasons: %v", clusters[0].Reasons)
	}
}

func TestListGPUClustersUnavailableMarkedUnfit(t *testing.T) {
	mc := managedCluster("gpu1", map[string]string{"gpu-type": "H100", "gpu-available": "true"})
	mgr := newTestManager(mc)

	clusters, err := mgr.ListGPUClusters(context.Background())
	if err != nil {
		t.Fatalf("ListGPUClusters failed: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Fit {
		t.Error("expected Fit=false for unavailable cluster")
	}
	if !containsReason(clusters[0].Reasons, FitnessReasonNotAvailable) {
		t.Errorf("expected NotAvailable reason, got %v", clusters[0].Reasons)
	}
}

func TestListGPUClustersSaturatedMarkedUnfit(t *testing.T) {
	mc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": "gpu1",
				"labels": map[string]interface{}{
					"gpu-type":      "H100",
					"gpu-available": "false",
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "ManagedClusterConditionAvailable",
						"status": "True",
					},
				},
			},
		},
	}
	mgr := newTestManager(mc)

	clusters, err := mgr.ListGPUClusters(context.Background())
	if err != nil {
		t.Fatalf("ListGPUClusters failed: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Fit {
		t.Error("expected Fit=false for saturated cluster")
	}
	if !containsReason(clusters[0].Reasons, FitnessReasonSaturated) {
		t.Errorf("expected Saturated reason, got %v", clusters[0].Reasons)
	}
}

func TestListGPUClustersMultipleMixedFitness(t *testing.T) {
	fit := gpuReadyCluster("gpu1", "H100")
	unfit := managedCluster("gpu2", map[string]string{"gpu-type": "A100"})
	mgr := newTestManager(fit, unfit)

	clusters, err := mgr.ListGPUClusters(context.Background())
	if err != nil {
		t.Fatalf("ListGPUClusters failed: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
	}

	fitCount := 0
	for _, c := range clusters {
		if c.Fit {
			fitCount++
		}
	}
	if fitCount != 1 {
		t.Errorf("expected 1 fit cluster, got %d", fitCount)
	}
}

func TestGetGPUFitnessSuccess(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if !result.Fit {
		t.Errorf("expected Fit=true, got reasons: %v", result.Reasons)
	}
	if result.Cluster != "gpu1" {
		t.Errorf("cluster = %q, want gpu1", result.Cluster)
	}
}

func TestGetGPUFitnessClusterNotFound(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.GetGPUFitness(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing cluster")
	}
}

func TestGetGPUFitnessNoGPULabel(t *testing.T) {
	mc := managedCluster("gpu1", map[string]string{"env": "prod"})
	mgr := newTestManager(mc)

	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if result.Fit {
		t.Error("expected Fit=false for cluster without gpu-type label")
	}
	if !containsReason(result.Reasons, FitnessReasonNoGPUType) {
		t.Errorf("expected NoGPUType reason, got %v", result.Reasons)
	}
}

func TestGetGPUFitnessNotAvailable(t *testing.T) {
	mc := managedCluster("gpu1", map[string]string{"gpu-type": "H100"})
	mgr := newTestManager(mc)

	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if result.Fit {
		t.Error("expected Fit=false for unavailable cluster")
	}
	if !containsReason(result.Reasons, FitnessReasonNotAvailable) {
		t.Errorf("expected NotAvailable reason, got %v", result.Reasons)
	}
}

func TestGetGPUFitnessSaturated(t *testing.T) {
	mc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": "gpu1",
				"labels": map[string]interface{}{
					"gpu-type":      "H100",
					"gpu-available": "false",
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "ManagedClusterConditionAvailable",
						"status": "True",
					},
				},
			},
		},
	}
	mgr := newTestManager(mc)

	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if result.Fit {
		t.Error("expected Fit=false for saturated cluster")
	}
	if !containsReason(result.Reasons, FitnessReasonSaturated) {
		t.Errorf("expected Saturated reason, got %v", result.Reasons)
	}
}

func TestGetGPUFitnessMultipleReasons(t *testing.T) {
	mc := managedCluster("gpu1", nil)
	mgr := newTestManager(mc)

	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if result.Fit {
		t.Error("expected Fit=false")
	}
	if !containsReason(result.Reasons, FitnessReasonNoGPUType) {
		t.Errorf("expected NoGPUType reason, got %v", result.Reasons)
	}
	if !containsReason(result.Reasons, FitnessReasonNotAvailable) {
		t.Errorf("expected NotAvailable reason, got %v", result.Reasons)
	}
}

func TestGetGPUFitnessAvailabilityUnknown(t *testing.T) {
	mc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": "gpu1",
				"labels": map[string]interface{}{
					"gpu-type": "H100",
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "ManagedClusterConditionAvailable",
						"status": "True",
					},
				},
			},
		},
	}
	mgr := newTestManager(mc)

	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if result.Fit {
		t.Error("expected Fit=false when gpu-available label is absent")
	}
	if !containsReason(result.Reasons, FitnessReasonAvailabilityUnknown) {
		t.Errorf("expected AvailabilityUnknown reason, got %v", result.Reasons)
	}
}

func TestGetGPUFitnessUnknownLabelValue(t *testing.T) {
	mc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": "gpu1",
				"labels": map[string]interface{}{
					"gpu-type":      "H100",
					"gpu-available": "unknown",
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "ManagedClusterConditionAvailable",
						"status": "True",
					},
				},
			},
		},
	}
	mgr := newTestManager(mc)

	result, err := mgr.GetGPUFitness(context.Background(), "gpu1")
	if err != nil {
		t.Fatalf("GetGPUFitness failed: %v", err)
	}
	if result.Fit {
		t.Error("expected Fit=false when gpu-available has unrecognised value")
	}
	if !containsReason(result.Reasons, FitnessReasonAvailabilityUnknown) {
		t.Errorf("expected AvailabilityUnknown reason, got %v", result.Reasons)
	}
}

func containsReason(reasons []FitnessReason, target FitnessReason) bool {
	for _, r := range reasons {
		if r == target {
			return true
		}
	}
	return false
}
