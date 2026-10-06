package gpu

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type FitnessReason string

const (
	FitnessReasonFit                FitnessReason = "Fit"
	FitnessReasonNoGPUType          FitnessReason = "NoGPUType"
	FitnessReasonNotAvailable       FitnessReason = "NotAvailable"
	FitnessReasonSaturated          FitnessReason = "Saturated"
	FitnessReasonAvailabilityUnknown FitnessReason = "AvailabilityUnknown"
)

type GPUClusterInfo struct {
	Cluster      string          `json:"cluster"`
	GPUType      string          `json:"gpuType"`
	Available    bool            `json:"available"`
	GPUAvailable bool            `json:"gpuAvailable"`
	Fit          bool            `json:"fit"`
	Reasons      []FitnessReason `json:"reasons,omitempty"`
}

type FitnessResult struct {
	Cluster string          `json:"cluster"`
	Fit     bool            `json:"fit"`
	Reasons []FitnessReason `json:"reasons,omitempty"`
}

func (m *Manager) ListGPUClusters(ctx context.Context) ([]GPUClusterInfo, error) {
	m.logger.Info("gpu.ListGPUClusters")

	list, err := m.client.List(ctx, client.GVRManagedCluster, "", "")
	if err != nil {
		return nil, fmt.Errorf("listing ManagedClusters: %w", err)
	}

	var clusters []GPUClusterInfo
	for _, item := range list.Items {
		labels, _, _ := unstructured.NestedStringMap(item.Object, "metadata", "labels")
		gpuType := labels["gpu-type"]
		if gpuType == "" {
			continue
		}

		info := evaluateCluster(item.GetName(), labels, item.Object)
		clusters = append(clusters, info)
	}

	if clusters == nil {
		clusters = []GPUClusterInfo{}
	}
	return clusters, nil
}

func (m *Manager) GetGPUFitness(ctx context.Context, cluster string) (*FitnessResult, error) {
	m.logger.Info("gpu.GetGPUFitness", "cluster", cluster)

	mc, err := m.client.Get(ctx, client.GVRManagedCluster, "", cluster)
	if err != nil {
		return nil, fmt.Errorf("cluster %s not found: %w", cluster, err)
	}

	labels, _, _ := unstructured.NestedStringMap(mc.Object, "metadata", "labels")
	info := evaluateCluster(cluster, labels, mc.Object)

	return &FitnessResult{
		Cluster: cluster,
		Fit:     info.Fit,
		Reasons: info.Reasons,
	}, nil
}

func evaluateCluster(name string, labels map[string]string, obj map[string]interface{}) GPUClusterInfo {
	gpuType := labels["gpu-type"]
	gpuAvailLabel, gpuAvailSet := labels["gpu-available"]
	gpuAvailable := gpuAvailSet && gpuAvailLabel == "true"
	available := clusterAvailable(obj)

	var reasons []FitnessReason
	if gpuType == "" {
		reasons = append(reasons, FitnessReasonNoGPUType)
	}
	if !available {
		reasons = append(reasons, FitnessReasonNotAvailable)
	}
	if !gpuAvailSet {
		reasons = append(reasons, FitnessReasonAvailabilityUnknown)
	} else if gpuAvailLabel == "false" {
		reasons = append(reasons, FitnessReasonSaturated)
	} else if gpuAvailLabel != "true" {
		reasons = append(reasons, FitnessReasonAvailabilityUnknown)
	}

	return GPUClusterInfo{
		Cluster:      name,
		GPUType:      gpuType,
		Available:    available,
		GPUAvailable: gpuAvailable,
		Fit:          len(reasons) == 0,
		Reasons:      reasons,
	}
}

func clusterAvailable(obj map[string]interface{}) bool {
	conditions, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		if condType == "ManagedClusterConditionAvailable" && condStatus == "True" {
			return true
		}
	}
	return false
}
