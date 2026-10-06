package observability

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type DiagnoseCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type DiagnoseResult struct {
	Healthy bool           `json:"healthy"`
	Checks  []DiagnoseCheck `json:"checks"`
}

func (r *DiagnoseResult) addCheck(name, status, message string) {
	r.Checks = append(r.Checks, DiagnoseCheck{Name: name, Status: status, Message: message})
	if status == "fail" {
		r.Healthy = false
	}
}

func (m *Manager) Diagnose(ctx context.Context) (*DiagnoseResult, error) {
	m.logger.Info("observability.Diagnose")
	result := &DiagnoseResult{Healthy: true}

	m.checkMCEAvailable(ctx, result)
	m.checkMCHComplete(ctx, result)
	m.checkPullSecret(ctx, result)
	m.checkMCOStatus(ctx, result)
	m.checkDisabledClusters(ctx, result)
	m.checkAddonHealth(ctx, result)

	return result, nil
}

func (m *Manager) checkMCEAvailable(ctx context.Context, result *DiagnoseResult) {
	list, err := m.client.Dynamic.Resource(client.GVRMultiClusterEngine).
		List(ctx, metav1.ListOptions{})
	if err != nil || len(list.Items) == 0 {
		result.addCheck("mce-available", "skip", "MultiClusterEngine not found")
		return
	}
	available, reason := conditionStatus(list.Items[0].Object, "Available")
	if available == "True" {
		result.addCheck("mce-available", "pass", "MultiClusterEngine is available")
		return
	}
	result.addCheck("mce-available", "fail",
		fmt.Sprintf("MultiClusterEngine not available (Available=%s: %s); this blocks MCH and MCO addon deployment", available, reason))
}

func (m *Manager) checkMCHComplete(ctx context.Context, result *DiagnoseResult) {
	list, err := m.client.Dynamic.Resource(client.GVRMultiClusterHub).
		List(ctx, metav1.ListOptions{})
	if err != nil || len(list.Items) == 0 {
		result.addCheck("mch-complete", "skip", "MultiClusterHub not found")
		return
	}
	complete, reason := conditionStatus(list.Items[0].Object, "Complete")
	if complete == "True" {
		result.addCheck("mch-complete", "pass", "MultiClusterHub is complete")
		return
	}
	result.addCheck("mch-complete", "fail",
		fmt.Sprintf("MultiClusterHub not complete (Complete=%s: %s); MCO cannot populate image list until MCH completes", complete, reason))
}

func (m *Manager) checkPullSecret(ctx context.Context, result *DiagnoseResult) {
	_, err := m.client.Get(ctx, client.GVRSecret, Namespace, PullSecretName)
	if err == nil {
		result.addCheck("pull-secret", "pass", "pull secret present")
		return
	}
	result.addCheck("pull-secret", "fail",
		fmt.Sprintf("%s not found in %s; MCO cannot resolve addon images without it", PullSecretName, Namespace))
}

func (m *Manager) checkMCOStatus(ctx context.Context, result *DiagnoseResult) {
	status, err := m.Status(ctx)
	if err != nil {
		result.addCheck("mco-status", "fail", fmt.Sprintf("error getting MCO status: %v", err))
		return
	}
	if status == "Ready" {
		result.addCheck("mco-status", "pass", "MCO is ready")
		return
	}
	if status == "NotInstalled" {
		result.addCheck("mco-status", "fail", "MCO not installed; run 'acmlab observability setup' first")
		return
	}
	result.addCheck("mco-status", "warn", fmt.Sprintf("MCO status: %s", status))
}

func (m *Manager) checkDisabledClusters(ctx context.Context, result *DiagnoseResult) {
	clusters, err := m.client.Dynamic.Resource(client.GVRManagedCluster).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		result.addCheck("disabled-clusters", "fail", fmt.Sprintf("error listing clusters: %v", err))
		return
	}
	var disabled []string
	for _, c := range clusters.Items {
		labels := c.GetLabels()
		if labels["observability"] == "disabled" {
			disabled = append(disabled, c.GetName())
		}
	}
	if len(disabled) > 0 {
		result.addCheck("disabled-clusters", "fail",
			fmt.Sprintf("clusters with observability disabled: %v; run 'acmlab observability enable-cluster <name>' to fix", disabled))
		return
	}
	result.addCheck("disabled-clusters", "pass", "no clusters have observability disabled")
}

func (m *Manager) checkAddonHealth(ctx context.Context, result *DiagnoseResult) {
	health, err := m.ListAddonHealth(ctx)
	if err != nil {
		result.addCheck("addon-health", "fail", fmt.Sprintf("error listing addon health: %v", err))
		return
	}
	clusters, err := m.client.Dynamic.Resource(client.GVRManagedCluster).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		result.addCheck("addon-health", "fail", fmt.Sprintf("error listing clusters: %v", err))
		return
	}

	addonClusters := map[string]bool{}
	for _, h := range health {
		addonClusters[h.Cluster] = true
	}

	var missing []string
	var degraded []string
	var unavailable []string
	for _, c := range clusters.Items {
		name := c.GetName()
		if name == "local-cluster" {
			continue
		}
		if !addonClusters[name] {
			missing = append(missing, name)
		}
	}
	for _, h := range health {
		if h.Cluster == Namespace {
			continue
		}
		if h.Degraded {
			degraded = append(degraded, h.Cluster)
		}
		if !h.Available {
			unavailable = append(unavailable, h.Cluster)
		}
	}

	if len(missing) > 0 {
		result.addCheck("addon-health", "fail",
			fmt.Sprintf("clusters without observability addon: %v; check MCH/MCE health or pull secret", missing))
		return
	}
	if len(degraded) > 0 {
		result.addCheck("addon-health", "warn",
			fmt.Sprintf("degraded addons: %v", degraded))
		return
	}
	if len(unavailable) > 0 {
		result.addCheck("addon-health", "warn",
			fmt.Sprintf("addons not yet available (may still be deploying): %v", unavailable))
		return
	}
	result.addCheck("addon-health", "pass", fmt.Sprintf("all %d cluster addons healthy", len(health)))
}

func (m *Manager) Repair(ctx context.Context) ([]string, error) {
	m.logger.Info("observability.Repair")
	var actions []string

	_, err := m.client.Get(ctx, client.GVRSecret, Namespace, PullSecretName)
	if err != nil && apierrors.IsNotFound(err) {
		if err := m.ConfigurePullSecret(ctx); err != nil {
			return actions, fmt.Errorf("configuring pull secret: %w", err)
		}
		actions = append(actions, "created pull secret "+PullSecretName)
	}

	clusters, err := m.client.Dynamic.Resource(client.GVRManagedCluster).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return actions, fmt.Errorf("listing clusters: %w", err)
	}
	for _, c := range clusters.Items {
		labels := c.GetLabels()
		if labels["observability"] == "disabled" {
			name := c.GetName()
			if err := m.EnableCluster(ctx, name); err != nil {
				return actions, fmt.Errorf("enabling cluster %s: %w", name, err)
			}
			actions = append(actions, "enabled observability for cluster "+name)
		}
	}

	return actions, nil
}

func conditionStatus(obj map[string]interface{}, condType string) (string, string) {
	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return "Unknown", ""
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == condType {
			s, _ := cond["status"].(string)
			msg, _ := cond["message"].(string)
			return s, msg
		}
	}
	return "Unknown", "condition not found"
}
