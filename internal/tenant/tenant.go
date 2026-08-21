package tenant

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

type TenantOpts struct {
	Name        string
	Cluster     string
	Team        string
	CPULimit    string
	MemoryLimit string
	PodLimit    int64
}

type TenantInfo struct {
	Name    string
	Cluster string
	Status  string
}

type ManifestStatus struct {
	Name       string
	Cluster    string
	Applied    bool
	Conditions []string
	Resources  []ResourceStatus
}

type ResourceStatus struct {
	Kind      string
	Name      string
	Namespace string
	Status    string
}

type Manager struct {
	client *client.Client
	cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Manager {
	return &Manager{client: c, cfg: cfg}
}

func (m *Manager) Deploy(ctx context.Context, opts TenantOpts) error {
	name := manifestWorkName(opts.Name)
	manifests := buildTenantManifests(opts)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "work.open-cluster-management.io/v1",
			"kind":       "ManifestWork",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": opts.Cluster,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/tenant": opts.Name,
				},
			},
			"spec": map[string]interface{}{
				"workload": map[string]interface{}{
					"manifests": manifests,
				},
			},
		},
	}
	return m.createIfNotExists(ctx, client.GVRManifestWork, opts.Cluster, obj)
}

func (m *Manager) Remove(ctx context.Context, tenantName, cluster string) error {
	name := manifestWorkName(tenantName)
	return m.deleteIfExists(ctx, client.GVRManifestWork, cluster, name)
}

func (m *Manager) List(ctx context.Context, cluster string) ([]TenantInfo, error) {
	list, err := m.client.List(ctx, client.GVRManifestWork, cluster, "acmlab.redhat.com/tenant")
	if err != nil {
		return nil, fmt.Errorf("listing tenant manifestworks: %w", err)
	}
	tenants := make([]TenantInfo, 0, len(list.Items))
	for _, item := range list.Items {
		tenants = append(tenants, parseTenantInfo(item.Object))
	}
	return tenants, nil
}

func (m *Manager) Status(ctx context.Context, tenantName, cluster string) (*ManifestStatus, error) {
	name := manifestWorkName(tenantName)
	obj, err := m.client.Get(ctx, client.GVRManifestWork, cluster, name)
	if err != nil {
		return nil, fmt.Errorf("getting tenant manifestwork %s: %w", name, err)
	}
	return parseManifestStatus(obj.Object), nil
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

func manifestWorkName(tenantName string) string {
	return "tenant-" + tenantName
}

func parseTenantInfo(obj map[string]interface{}) TenantInfo {
	info := TenantInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Cluster, _ = meta["namespace"].(string)
		if labels, ok := meta["labels"].(map[string]interface{}); ok {
			info.Name, _ = labels["acmlab.redhat.com/tenant"].(string)
		}
	}

	info.Status = "Pending"
	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return info
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "Applied" && cond["status"] == "True" {
			info.Status = "Applied"
		}
	}
	return info
}

func parseManifestStatus(obj map[string]interface{}) *ManifestStatus {
	ms := &ManifestStatus{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		ms.Name, _ = meta["name"].(string)
		ms.Cluster, _ = meta["namespace"].(string)
	}

	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return ms
	}

	conditions, _ := status["conditions"].([]interface{})
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		ms.Conditions = append(ms.Conditions, fmt.Sprintf("%s=%s", condType, condStatus))
		if condType == "Applied" && condStatus == "True" {
			ms.Applied = true
		}
	}

	resourceStatus, _ := status["resourceStatus"].(map[string]interface{})
	if resourceStatus != nil {
		manifests, _ := resourceStatus["manifests"].([]interface{})
		for _, raw := range manifests {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			rs := ResourceStatus{}
			if ref, ok := m["resourceMeta"].(map[string]interface{}); ok {
				rs.Kind, _ = ref["kind"].(string)
				rs.Name, _ = ref["name"].(string)
				rs.Namespace, _ = ref["namespace"].(string)
			}
			conditions, _ := m["conditions"].([]interface{})
			for _, c := range conditions {
				cd, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cd["type"] == "Applied" {
					rs.Status, _ = cd["status"].(string)
				}
			}
			ms.Resources = append(ms.Resources, rs)
		}
	}

	return ms
}
