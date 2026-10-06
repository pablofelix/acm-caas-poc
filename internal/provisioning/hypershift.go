package provisioning

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

const DefaultHyperShiftNamespace = "clusters"

type HyperShiftOpts struct {
	Name             string
	Namespace        string
	Platform         string
	Region           string
	BaseDomain       string
	NodePoolReplicas int64
	ReleaseImage     string
	PullSecret       string
	InfraID          string
}

type HyperShiftInfo struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Available  bool     `json:"available"`
	Version    string   `json:"version,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
}

func (m *Manager) applyHyperShiftDefaults(opts *HyperShiftOpts) {
	if opts.Namespace == "" {
		opts.Namespace = DefaultHyperShiftNamespace
	}
	if opts.Platform == "" {
		opts.Platform = m.cfg.Platform
	}
	if opts.Region == "" {
		opts.Region = m.cfg.IBMCloudRegion
	}
	if opts.BaseDomain == "" {
		opts.BaseDomain = m.cfg.BaseDomain
	}
	if opts.NodePoolReplicas == 0 {
		opts.NodePoolReplicas = 2
	}
	if opts.InfraID == "" {
		opts.InfraID = opts.Name
	}
}

func (m *Manager) CreateHyperShift(ctx context.Context, opts HyperShiftOpts) error {
	m.logger.Info("provisioning.CreateHyperShift", "cluster", opts.Name)
	m.applyHyperShiftDefaults(&opts)

	if opts.PullSecret == "" {
		return fmt.Errorf("pull secret is required for HyperShift provisioning")
	}
	if opts.ReleaseImage == "" {
		return fmt.Errorf("release image is required for HyperShift provisioning")
	}

	if err := m.resolveHyperShiftReleaseImage(ctx, &opts); err != nil {
		return err
	}

	ns := buildNamespace(opts.Namespace)
	if err := m.client.CreateIfNotExists(ctx, client.GVRNamespace, "", ns); err != nil {
		return fmt.Errorf("creating namespace %s: %w", opts.Namespace, err)
	}

	pull := buildHyperShiftPullSecret(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Namespace, pull); err != nil {
		return fmt.Errorf("creating pull secret: %w", err)
	}

	hc := buildHostedCluster(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRHostedCluster, opts.Namespace, hc); err != nil {
		return fmt.Errorf("creating HostedCluster %s: %w", opts.Name, err)
	}

	np := buildNodePool(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRNodePool, opts.Namespace, np); err != nil {
		return fmt.Errorf("creating NodePool %s: %w", opts.Name, err)
	}

	mc := buildHyperShiftManagedCluster(opts.Name)
	if err := m.client.CreateIfNotExists(ctx, client.GVRManagedCluster, "", mc); err != nil {
		return fmt.Errorf("creating ManagedCluster %s: %w", opts.Name, err)
	}

	return nil
}

func (m *Manager) DestroyHyperShift(ctx context.Context, namespace, name string) error {
	m.logger.Info("provisioning.DestroyHyperShift", "cluster", name)
	if namespace == "" {
		namespace = DefaultHyperShiftNamespace
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRNodePool, namespace, name); err != nil {
		return fmt.Errorf("deleting NodePool %s: %w", name, err)
	}

	err := m.client.Delete(ctx, client.GVRHostedCluster, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting HostedCluster %s: %w", name, err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRManagedCluster, "", name); err != nil {
		m.logger.Warn("failed to delete ManagedCluster during HyperShift destroy", "cluster", name, "error", err)
	}

	return nil
}

func (m *Manager) StatusHyperShift(ctx context.Context, namespace, name string) (*HyperShiftInfo, error) {
	m.logger.Info("provisioning.StatusHyperShift", "cluster", name)
	if namespace == "" {
		namespace = DefaultHyperShiftNamespace
	}

	obj, err := m.client.Get(ctx, client.GVRHostedCluster, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("getting HostedCluster %s/%s: %w", namespace, name, err)
	}
	return parseHyperShiftInfo(obj.Object), nil
}

func (m *Manager) ListHyperShift(ctx context.Context) ([]HyperShiftInfo, error) {
	m.logger.Info("provisioning.ListHyperShift")
	list, err := m.client.List(ctx, client.GVRHostedCluster, "", "acmlab.redhat.com/managed")
	if err != nil {
		return nil, fmt.Errorf("listing HostedClusters: %w", err)
	}
	infos := make([]HyperShiftInfo, 0, len(list.Items))
	for _, item := range list.Items {
		infos = append(infos, *parseHyperShiftInfo(item.Object))
	}
	return infos, nil
}

func parseHyperShiftInfo(obj map[string]interface{}) *HyperShiftInfo {
	info := &HyperShiftInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		info.Namespace, _ = meta["namespace"].(string)
	}
	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return info
	}
	if v, ok := status["version"].(map[string]interface{}); ok {
		info.Version, _ = v["history"].(string)
		if desired, ok := v["desired"].(map[string]interface{}); ok {
			info.Version, _ = desired["image"].(string)
		}
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		info.Conditions = append(info.Conditions, fmt.Sprintf("%s=%s", condType, condStatus))
		if condType == "Available" && condStatus == "True" {
			info.Available = true
		}
	}
	return info
}

// resolveHyperShiftReleaseImage resolves a ClusterImageSet name to a release image URL.
// If ReleaseImage already looks like an image reference (contains / or @), it's used as-is.
func (m *Manager) resolveHyperShiftReleaseImage(ctx context.Context, opts *HyperShiftOpts) error {
	if strings.Contains(opts.ReleaseImage, "/") || strings.Contains(opts.ReleaseImage, "@") {
		return nil
	}
	obj, err := m.client.Get(ctx, client.GVRClusterImageSet, "", opts.ReleaseImage)
	if err != nil {
		return fmt.Errorf("resolving ClusterImageSet %s: %w", opts.ReleaseImage, err)
	}
	image, _, _ := unstructured.NestedString(obj.Object, "spec", "releaseImage")
	if image == "" {
		return fmt.Errorf("ClusterImageSet %s has no spec.releaseImage", opts.ReleaseImage)
	}
	m.logger.Info("provisioning.resolveHyperShiftReleaseImage", "imageSet", opts.ReleaseImage, "releaseImage", image)
	opts.ReleaseImage = image
	return nil
}
