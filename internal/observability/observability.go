package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

const (
	Namespace      = "open-cluster-management-observability"
	MinIOName      = "minio"
	MinIOPort      = 9000
	ThanosCfgKey   = "thanos.yaml"
	SecretName     = "thanos-object-storage"
	MCOName        = "observability"
	StorageClass   = "ibmc-vpc-block-10iops-tier"
	MinIOPVCSize   = "20Gi"
	MinIOAccessKey = "minio"
	MinIOSecretKey = "minio123"
	MinioBucket    = "thanos"

	PullSecretName      = "multiclusterhub-operator-pull-secret"
	PullSecretSourceNS  = "openshift-config"
	MetricsAllowlistCM  = "observability-metrics-custom-allowlist"
	CustomRulesCM       = "thanos-ruler-custom-rules"
	DashboardLabelKey   = "grafana-custom-dashboard"
	DashboardLabelValue = "true"
	OBCName             = "observability-obc"
)

type Manager struct {
	client *client.Client
	cfg    config.Config
	logger *slog.Logger
}

func New(c *client.Client, cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{client: c, cfg: cfg, logger: logger}
}

func (m *Manager) createOrUpdate(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
	name := obj.GetName()
	existing, err := m.client.Get(ctx, gvr, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, createErr := m.client.Create(ctx, gvr, namespace, obj)
			return createErr
		}
		return err
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	_, err = m.client.Update(ctx, gvr, namespace, obj)
	return err
}

func (m *Manager) Setup(ctx context.Context) error {
	m.logger.Info("observability.Setup")
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"namespace", m.ensureNamespace},
		{"minio-pvc", m.ensureMinioPVC},
		{"minio-deployment", m.ensureMinioDeployment},
		{"minio-service", m.ensureMinioService},
		{"thanos-secret", m.ensureThanosSecret},
		{"multiclusterobservability", m.ensureMCO},
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("setup %s: %w", s.name, err)
		}
	}
	return nil
}

func (m *Manager) Teardown(ctx context.Context) error {
	m.logger.Info("observability.Teardown")
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"multiclusterobservability", m.deleteMCO},
		{"thanos-secret", m.deleteSecret},
		{"minio-service", m.deleteMinioService},
		{"minio-deployment", m.deleteMinioDeployment},
		{"minio-pvc", m.deleteMinioPVC},
		{"namespace", m.deleteNamespace},
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("teardown %s: %w", s.name, err)
		}
	}
	return nil
}

func (m *Manager) Status(ctx context.Context) (string, error) {
	m.logger.Info("observability.Status")
	obj, err := m.client.Get(ctx, client.GVRMultiClusterObservability, "", MCOName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "NotInstalled", nil
		}
		return "", fmt.Errorf("getting MCO: %w", err)
	}
	status, _ := obj.Object["status"].(map[string]interface{})
	if status == nil {
		return "Pending", nil
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "Ready" && cond["status"] == "True" {
			return "Ready", nil
		}
	}
	return "Progressing", nil
}

type StorageOpts struct {
	Type         string
	BucketName   string
	StorageClass string
	Endpoint     string
}

type CustomRuleOpts struct {
	Name  string
	Rules string
}

type DashboardOpts struct {
	Name string
	JSON string
}

type MetricsOpts struct {
	Metrics []string
}

type RetentionOpts struct {
	RetentionInLocal string
	BlockDuration    string
	DeleteDelay      string
}

type AddonHealth struct {
	Cluster   string `json:"cluster"`
	Available bool   `json:"available"`
	Degraded  bool   `json:"degraded"`
}

func (m *Manager) ConfigurePullSecret(ctx context.Context) error {
	m.logger.Info("observability.ConfigurePullSecret")
	src, err := m.client.Get(ctx, client.GVRSecret, PullSecretSourceNS, "pull-secret")
	if err != nil {
		return fmt.Errorf("getting source pull secret: %w", err)
	}
	dst := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      PullSecretName,
				"namespace": Namespace,
			},
			"type": src.Object["type"],
			"data": src.Object["data"],
		},
	}
	return m.client.CreateIfNotExists(ctx, client.GVRSecret, Namespace, dst)
}

func (m *Manager) ConfigureOBCStorage(ctx context.Context, opts StorageOpts) error {
	m.logger.Info("observability.ConfigureOBCStorage")
	if opts.BucketName == "" {
		opts.BucketName = MinioBucket
	}
	if opts.StorageClass == "" {
		opts.StorageClass = StorageClass
	}
	obc := buildOBC(OBCName, Namespace, opts.StorageClass, opts.BucketName)
	return m.client.CreateIfNotExists(ctx, client.GVRObjectBucketClaim, Namespace, obc)
}

func (m *Manager) DeployCustomRules(ctx context.Context, opts CustomRuleOpts) error {
	m.logger.Info("observability.DeployCustomRules")
	if err := ValidateRulesYAML(opts.Rules); err != nil {
		return fmt.Errorf("rules validation: %w", err)
	}
	cm := buildCustomRulesConfigMap(Namespace, opts.Rules)
	return m.createOrUpdate(ctx, client.GVRConfigMap, Namespace, cm)
}

func (m *Manager) RemoveCustomRules(ctx context.Context) error {
	m.logger.Info("observability.RemoveCustomRules")
	return m.client.DeleteIfExists(ctx, client.GVRConfigMap, Namespace, CustomRulesCM)
}

func (m *Manager) DeployDashboard(ctx context.Context, opts DashboardOpts) error {
	m.logger.Info("observability.DeployDashboard", "name", opts.Name)
	if err := ValidateDashboardJSON(opts.JSON); err != nil {
		return fmt.Errorf("dashboard validation: %w", err)
	}
	cm := buildDashboardConfigMap(Namespace, opts.Name, opts.JSON)
	return m.createOrUpdate(ctx, client.GVRConfigMap, Namespace, cm)
}

func (m *Manager) RemoveDashboard(ctx context.Context, name string) error {
	m.logger.Info("observability.RemoveDashboard", "name", name)
	return m.client.DeleteIfExists(ctx, client.GVRConfigMap, Namespace, name)
}

func (m *Manager) ConfigureMetricsAllowlist(ctx context.Context, opts MetricsOpts) error {
	m.logger.Info("observability.ConfigureMetricsAllowlist")
	return m.mergeMetricsKey(ctx, "metrics_list.yaml", opts.Metrics)
}

func (m *Manager) ListAddonHealth(ctx context.Context) ([]AddonHealth, error) {
	m.logger.Info("observability.ListAddonHealth")
	list, err := m.client.Dynamic.Resource(client.GVRManagedClusterAddOn).
		Namespace("").
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing addons: %w", err)
	}
	var result []AddonHealth
	for _, item := range list.Items {
		if item.GetName() != "observability-controller" {
			continue
		}
		h := AddonHealth{Cluster: item.GetNamespace()}
		status, _ := item.Object["status"].(map[string]interface{})
		if status == nil {
			result = append(result, h)
			continue
		}
		conditions, _ := status["conditions"].([]interface{})
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			switch cond["type"] {
			case "Available":
				h.Available = cond["status"] == "True"
			case "Degraded":
				h.Degraded = cond["status"] == "True"
			}
		}
		result = append(result, h)
	}
	return result, nil
}

func (m *Manager) ConfigureRetention(ctx context.Context, opts RetentionOpts) error {
	m.logger.Info("observability.ConfigureRetention")
	obj, err := m.client.Get(ctx, client.GVRMultiClusterObservability, "", MCOName)
	if err != nil {
		return fmt.Errorf("getting MCO: %w", err)
	}
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		spec = map[string]interface{}{}
		obj.Object["spec"] = spec
	}
	retention, _ := spec["retentionConfig"].(map[string]interface{})
	if retention == nil {
		retention = map[string]interface{}{}
	}
	if opts.RetentionInLocal != "" {
		retention["retentionInLocal"] = opts.RetentionInLocal
	}
	if opts.BlockDuration != "" {
		retention["blockDuration"] = opts.BlockDuration
	}
	if opts.DeleteDelay != "" {
		retention["deleteDelay"] = opts.DeleteDelay
	}
	spec["retentionConfig"] = retention
	_, err = m.client.Update(ctx, client.GVRMultiClusterObservability, "", obj)
	return err
}

func (m *Manager) ConfigureMetricsAllowlistFromYAML(ctx context.Context, metricsYAML string) error {
	m.logger.Info("observability.ConfigureMetricsAllowlistFromYAML")
	return m.mergeMetricsData(ctx, "metrics_list.yaml", metricsYAML)
}

type MetricsScope string

const (
	MetricsScopeGlobal   MetricsScope = "global"
	MetricsScopeWorkload MetricsScope = "workload"
	MetricsScopeCluster  MetricsScope = "cluster"
)

type MetricsConfigOpts struct {
	Scope   MetricsScope
	Cluster string
	Metrics []string
}

func (m *Manager) ConfigureMetrics(ctx context.Context, opts MetricsConfigOpts) error {
	m.logger.Info("observability.ConfigureMetrics", "scope", opts.Scope)
	switch opts.Scope {
	case MetricsScopeCluster:
		return fmt.Errorf("per-cluster metrics configuration requires direct access to the managed cluster %q", opts.Cluster)
	case MetricsScopeWorkload:
		return fmt.Errorf("user workload metrics require enabling the user-workload-monitoring stack on each managed cluster; hub-only configuration via this tool is not yet supported")
	default:
		return m.mergeMetricsKey(ctx, "metrics_list.yaml", opts.Metrics)
	}
}

func (m *Manager) mergeMetricsKey(ctx context.Context, key string, metrics []string) error {
	return m.mergeMetricsData(ctx, key, "names:\n"+metricsToYAML(metrics))
}

func (m *Manager) mergeMetricsData(ctx context.Context, key, value string) error {
	existing, err := m.client.Get(ctx, client.GVRConfigMap, Namespace, MetricsAllowlistCM)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		cm := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      MetricsAllowlistCM,
					"namespace": Namespace,
				},
				"data": map[string]interface{}{
					key: value,
				},
			},
		}
		_, err = m.client.Create(ctx, client.GVRConfigMap, Namespace, cm)
		return err
	}
	data, _ := existing.Object["data"].(map[string]interface{})
	if data == nil {
		data = map[string]interface{}{}
	}
	data[key] = value
	existing.Object["data"] = data
	_, err = m.client.Update(ctx, client.GVRConfigMap, Namespace, existing)
	return err
}

func (m *Manager) DisableCluster(ctx context.Context, name string) error {
	m.logger.Info("observability.DisableCluster", "cluster", name)
	obj, err := m.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err != nil {
		return fmt.Errorf("getting managed cluster %s: %w", name, err)
	}
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["observability"] = "disabled"
	obj.SetLabels(labels)
	_, err = m.client.Update(ctx, client.GVRManagedCluster, "", obj)
	return err
}

func (m *Manager) EnableCluster(ctx context.Context, name string) error {
	m.logger.Info("observability.EnableCluster", "cluster", name)
	obj, err := m.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err != nil {
		return fmt.Errorf("getting managed cluster %s: %w", name, err)
	}
	labels := obj.GetLabels()
	if labels == nil {
		return nil
	}
	delete(labels, "observability")
	obj.SetLabels(labels)
	_, err = m.client.Update(ctx, client.GVRManagedCluster, "", obj)
	return err
}

func (m *Manager) GrafanaURL(ctx context.Context) (string, error) {
	m.logger.Info("observability.GrafanaURL")
	list, err := m.client.Dynamic.Resource(client.GVRRoute).
		Namespace(Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing routes: %w", err)
	}
	for _, item := range list.Items {
		if !strings.Contains(item.GetName(), "grafana") {
			continue
		}
		spec, _ := item.Object["spec"].(map[string]interface{})
		if spec == nil {
			continue
		}
		host, _ := spec["host"].(string)
		if host != "" {
			return "https://" + host, nil
		}
	}
	return "", fmt.Errorf("grafana route not found in namespace %s; check that the observability stack is fully deployed and the MCO status is Ready", Namespace)
}

func ValidateRulesYAML(rulesYAML string) error {
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(rulesYAML), &parsed); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	groupsRaw, ok := parsed["groups"]
	if !ok {
		return fmt.Errorf("rules YAML must contain a top-level 'groups' field; PromQL semantics are not validated server-side")
	}
	groups, ok := groupsRaw.([]interface{})
	if !ok {
		return fmt.Errorf("'groups' must be a list; PromQL semantics are not validated server-side")
	}
	for i, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			return fmt.Errorf("group[%d] must be a map", i)
		}
		if _, ok := group["name"]; !ok {
			return fmt.Errorf("group[%d] missing 'name'", i)
		}
	}
	return nil
}

func ValidateDashboardJSON(dashJSON string) error {
	if !json.Valid([]byte(dashJSON)) {
		return fmt.Errorf("invalid JSON for dashboard")
	}
	return nil
}

type AlertmanagerOpts struct {
	Config string
}

func ValidateAlertmanagerConfig(cfgYAML string) error {
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(cfgYAML), &parsed); err != nil {
		return fmt.Errorf("invalid alertmanager YAML: %w", err)
	}
	if _, ok := parsed["route"]; !ok {
		return fmt.Errorf("alertmanager config must contain 'route'")
	}
	if _, ok := parsed["receivers"]; !ok {
		return fmt.Errorf("alertmanager config must contain 'receivers'")
	}
	return nil
}

const AlertmanagerSecretName = "alertmanager-config"

func (m *Manager) ConfigureAlertmanager(ctx context.Context, opts AlertmanagerOpts) error {
	m.logger.Info("observability.ConfigureAlertmanager")
	if err := ValidateAlertmanagerConfig(opts.Config); err != nil {
		return err
	}
	secret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      AlertmanagerSecretName,
				"namespace": Namespace,
			},
			"type": "Opaque",
			"stringData": map[string]interface{}{
				"alertmanager.yaml": opts.Config,
			},
		},
	}
	return m.createOrUpdate(ctx, client.GVRSecret, Namespace, secret)
}

func (m *Manager) DisableAlertForwarding(ctx context.Context) error {
	m.logger.Info("observability.DisableAlertForwarding")
	obj, err := m.client.Get(ctx, client.GVRMultiClusterObservability, "", MCOName)
	if err != nil {
		return fmt.Errorf("getting MCO: %w", err)
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations["mco-disable-alerting"] = "true"
	obj.SetAnnotations(annotations)
	_, err = m.client.Update(ctx, client.GVRMultiClusterObservability, "", obj)
	return err
}

func (m *Manager) EnableAlertForwarding(ctx context.Context) error {
	m.logger.Info("observability.EnableAlertForwarding")
	obj, err := m.client.Get(ctx, client.GVRMultiClusterObservability, "", MCOName)
	if err != nil {
		return fmt.Errorf("getting MCO: %w", err)
	}
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}
	delete(annotations, "mco-disable-alerting")
	obj.SetAnnotations(annotations)
	_, err = m.client.Update(ctx, client.GVRMultiClusterObservability, "", obj)
	return err
}

type AdvancedOpts struct {
	ReceiveReplicas    *int64
	CollectionInterval *int64
	Downsampling       *bool
}

func (m *Manager) ConfigureAdvanced(ctx context.Context, opts AdvancedOpts) error {
	m.logger.Info("observability.ConfigureAdvanced")
	obj, err := m.client.Get(ctx, client.GVRMultiClusterObservability, "", MCOName)
	if err != nil {
		return fmt.Errorf("getting MCO: %w", err)
	}
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		spec = map[string]interface{}{}
		obj.Object["spec"] = spec
	}
	if opts.ReceiveReplicas != nil {
		advanced, _ := spec["advanced"].(map[string]interface{})
		if advanced == nil {
			advanced = map[string]interface{}{}
		}
		receive, _ := advanced["receive"].(map[string]interface{})
		if receive == nil {
			receive = map[string]interface{}{}
		}
		receive["replicas"] = *opts.ReceiveReplicas
		advanced["receive"] = receive
		spec["advanced"] = advanced
	}
	if opts.CollectionInterval != nil {
		addon, _ := spec["observabilityAddonSpec"].(map[string]interface{})
		if addon == nil {
			addon = map[string]interface{}{}
		}
		addon["interval"] = *opts.CollectionInterval
		spec["observabilityAddonSpec"] = addon
	}
	if opts.Downsampling != nil {
		spec["enableDownsampling"] = *opts.Downsampling
	}
	_, err = m.client.Update(ctx, client.GVRMultiClusterObservability, "", obj)
	return err
}

type WorkloadStatus struct {
	Name      string `json:"name"`
	Ready     bool   `json:"ready"`
	Replicas  int64  `json:"replicas"`
	Available int64  `json:"available"`
}

type VerifyResult struct {
	MCOStatus   string           `json:"mcoStatus"`
	Workloads   []WorkloadStatus `json:"workloads"`
	PVCsBound   bool             `json:"pvcsBound"`
	AddonHealth []AddonHealth    `json:"addonHealth"`
}

func workloadFromUnstructured(obj unstructured.Unstructured) WorkloadStatus {
	ws := WorkloadStatus{Name: obj.GetName()}
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec != nil {
		ws.Replicas, _ = spec["replicas"].(int64)
	}
	status, _ := obj.Object["status"].(map[string]interface{})
	if status != nil {
		if avail, ok := status["availableReplicas"].(int64); ok {
			ws.Available = avail
		} else if ready, ok := status["readyReplicas"].(int64); ok {
			ws.Available = ready
		}
	}
	ws.Ready = ws.Replicas > 0 && ws.Available >= ws.Replicas
	generation := obj.GetGeneration()
	if generation > 0 {
		observedGen, _, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
		if observedGen < generation {
			ws.Ready = false
		}
	}
	if ws.Ready && status != nil {
		updated, ok := status["updatedReplicas"].(int64)
		if ok && updated < ws.Replicas {
			ws.Ready = false
		}
	}
	return ws
}

func (m *Manager) Verify(ctx context.Context) (*VerifyResult, error) {
	m.logger.Info("observability.Verify")
	result := &VerifyResult{PVCsBound: true}

	mcoStatus, err := m.Status(ctx)
	if err != nil {
		return nil, err
	}
	result.MCOStatus = mcoStatus

	deps, err := m.client.Dynamic.Resource(client.GVRDeployment).
		Namespace(Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}
	for _, d := range deps.Items {
		ws := workloadFromUnstructured(d)
		result.Workloads = append(result.Workloads, ws)
	}

	sts, err := m.client.Dynamic.Resource(client.GVRStatefulSet).
		Namespace(Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets: %w", err)
	}
	for _, s := range sts.Items {
		ws := workloadFromUnstructured(s)
		result.Workloads = append(result.Workloads, ws)
	}

	pvcs, err := m.client.Dynamic.Resource(client.GVRPersistentVolumeClaim).
		Namespace(Namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing PVCs: %w", err)
	}
	for _, p := range pvcs.Items {
		status, _ := p.Object["status"].(map[string]interface{})
		if status == nil {
			result.PVCsBound = false
			continue
		}
		phase, _ := status["phase"].(string)
		if phase != "Bound" {
			result.PVCsBound = false
		}
	}

	health, err := m.ListAddonHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing addon health: %w", err)
	}
	result.AddonHealth = health

	return result, nil
}
