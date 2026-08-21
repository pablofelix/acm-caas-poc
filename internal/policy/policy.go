package policy

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

const (
	DefaultNamespace = "open-cluster-management-global-set"
)

type ComplianceInfo struct {
	ClusterName      string
	ComplianceState  string
}

type PolicyInfo struct {
	Name              string
	Namespace         string
	RemediationAction string
	Disabled          bool
	Compliant         string
	ClusterCompliance []ComplianceInfo
}

type Manager struct {
	client *client.Client
	cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Manager {
	return &Manager{client: c, cfg: cfg}
}

func (m *Manager) Apply(ctx context.Context, opts PolicyOpts) error {
	ns := opts.Namespace
	if ns == "" {
		ns = DefaultNamespace
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"placement", func() error { return m.ensurePlacement(ctx, ns, opts) }},
		{"policy", func() error { return m.ensurePolicy(ctx, ns, opts) }},
		{"placement-binding", func() error { return m.ensurePlacementBinding(ctx, ns, opts) }},
	}
	for _, s := range steps {
		if err := s.fn(); err != nil {
			return fmt.Errorf("apply %s: %w", s.name, err)
		}
	}
	return nil
}

func (m *Manager) Remove(ctx context.Context, name, namespace string) error {
	if namespace == "" {
		namespace = DefaultNamespace
	}

	steps := []struct {
		label string
		gvr   schema.GroupVersionResource
		name  string
	}{
		{"placement-binding", client.GVRPlacementBinding, name + "-placement-binding"},
		{"policy", client.GVRPolicy, name},
		{"placement", client.GVRPlacement, name + "-placement"},
	}
	for _, s := range steps {
		if err := m.deleteIfExists(ctx, s.gvr, namespace, s.name); err != nil {
			return fmt.Errorf("remove %s: %w", s.label, err)
		}
	}
	return nil
}

func (m *Manager) List(ctx context.Context, namespace string) ([]PolicyInfo, error) {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	list, err := m.client.List(ctx, client.GVRPolicy, namespace, "")
	if err != nil {
		return nil, fmt.Errorf("listing policies: %w", err)
	}
	policies := make([]PolicyInfo, 0, len(list.Items))
	for _, item := range list.Items {
		policies = append(policies, parsePolicyInfo(item.Object))
	}
	return policies, nil
}

func (m *Manager) Get(ctx context.Context, name, namespace string) (*PolicyInfo, error) {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	obj, err := m.client.Get(ctx, client.GVRPolicy, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("getting policy %s: %w", name, err)
	}
	info := parsePolicyInfo(obj.Object)
	return &info, nil
}

func (m *Manager) SetRemediation(ctx context.Context, name, namespace, action string) error {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"remediationAction": action,
		},
	}
	data, _ := json.Marshal(patch)
	_, err := m.client.Patch(ctx, client.GVRPolicy, namespace, name, types.MergePatchType, data)
	if err != nil {
		return fmt.Errorf("patching policy %s remediation to %s: %w", name, action, err)
	}
	return nil
}

func (m *Manager) SetDisabled(ctx context.Context, name, namespace string, disabled bool) error {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"disabled": disabled,
		},
	}
	data, _ := json.Marshal(patch)
	_, err := m.client.Patch(ctx, client.GVRPolicy, namespace, name, types.MergePatchType, data)
	if err != nil {
		return fmt.Errorf("patching policy %s disabled=%v: %w", name, disabled, err)
	}
	return nil
}

func (m *Manager) createIfNotExists(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
	_, err := m.client.Get(ctx, gvr, namespace, obj.GetName())
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = m.client.Create(ctx, gvr, namespace, obj)
	return err
}

func (m *Manager) deleteIfExists(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	err := m.client.Delete(ctx, gvr, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func parsePolicyInfo(obj map[string]interface{}) PolicyInfo {
	info := PolicyInfo{}

	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		info.Namespace, _ = meta["namespace"].(string)
	}

	spec, _ := obj["spec"].(map[string]interface{})
	if spec != nil {
		info.RemediationAction, _ = spec["remediationAction"].(string)
		info.Disabled, _ = spec["disabled"].(bool)
	}

	status, _ := obj["status"].(map[string]interface{})
	if status != nil {
		info.Compliant, _ = status["compliant"].(string)
		if cds, ok := status["status"].([]interface{}); ok {
			for _, raw := range cds {
				cs, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				ci := ComplianceInfo{}
				ci.ClusterName, _ = cs["clustername"].(string)
				ci.ComplianceState, _ = cs["compliant"].(string)
				info.ClusterCompliance = append(info.ClusterCompliance, ci)
			}
		}
		if info.Compliant == "" && len(info.ClusterCompliance) > 0 {
			info.Compliant = aggregateCompliance(info.ClusterCompliance)
		}
	}

	return info
}

func aggregateCompliance(clusters []ComplianceInfo) string {
	for _, c := range clusters {
		if c.ComplianceState == "NonCompliant" {
			return "NonCompliant"
		}
	}
	for _, c := range clusters {
		if c.ComplianceState == "Compliant" {
			return "Compliant"
		}
	}
	return "Pending"
}
