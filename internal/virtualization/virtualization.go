package virtualization

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

const (
	LabelVM     = "acmlab.redhat.com/vm"
	LabelVMName = "acmlab.redhat.com/vm-name"

	DefaultCPU      = "2"
	DefaultMemory   = "4Gi"
	DefaultImage    = "registry.redhat.io/rhel9/rhel-guest-image:latest"
	DefaultDiskSize = "20Gi"

	PolicyNamespace    = "open-cluster-management-global-set"
	CNVPolicyPrefix    = "install-cnv"
	CNVOperatorName    = "kubevirt-hyperconverged"
	CNVOperatorChannel = "stable"
	CNVNamespace       = "openshift-cnv"
)

type VMOpts struct {
	Name     string
	Cluster  string
	CPU      string
	Memory   string
	Image    string
	DiskSize string
}

type VMInfo struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	Status  string `json:"status"`
	IP      string `json:"ip,omitempty"`
}

type VMDetail struct {
	VMInfo
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
	Image    string `json:"image"`
	DiskSize string `json:"diskSize"`
	Node     string `json:"node,omitempty"`
}

type Manager struct {
	client *client.Client
	cfg    config.Config
	logger *slog.Logger
}

func New(c *client.Client, cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{client: c, cfg: cfg, logger: logger}
}

func mwName(name, cluster string) string {
	return fmt.Sprintf("vm-%s-%s", name, cluster)
}

func (m *Manager) Deploy(ctx context.Context, opts VMOpts) error {
	m.logger.Info("virtualization.Deploy", "name", opts.Name, "cluster", opts.Cluster)
	if opts.CPU == "" {
		opts.CPU = DefaultCPU
	}
	if opts.Memory == "" {
		opts.Memory = DefaultMemory
	}
	if opts.Image == "" {
		opts.Image = DefaultImage
	}
	if opts.DiskSize == "" {
		opts.DiskSize = DefaultDiskSize
	}
	mw := buildVMManifestWork(opts.Cluster, opts.Name, opts)
	return m.client.CreateIfNotExists(ctx, client.GVRManifestWork, opts.Cluster, mw)
}

func (m *Manager) Remove(ctx context.Context, name, cluster string) error {
	m.logger.Info("virtualization.Remove", "name", name, "cluster", cluster)
	return m.client.DeleteIfExists(ctx, client.GVRManifestWork, cluster, mwName(name, cluster))
}

func (m *Manager) Start(ctx context.Context, name, cluster string) error {
	m.logger.Info("virtualization.Start", "name", name, "cluster", cluster)
	return m.patchRunning(ctx, name, cluster, true)
}

func (m *Manager) Stop(ctx context.Context, name, cluster string) error {
	m.logger.Info("virtualization.Stop", "name", name, "cluster", cluster)
	return m.patchRunning(ctx, name, cluster, false)
}

func (m *Manager) patchRunning(ctx context.Context, name, cluster string, running bool) error {
	obj, err := m.client.Get(ctx, client.GVRManifestWork, cluster, mwName(name, cluster))
	if err != nil {
		return fmt.Errorf("getting ManifestWork: %w", err)
	}
	manifests, _ := obj.Object["spec"].(map[string]interface{})["workload"].(map[string]interface{})["manifests"].([]interface{})
	if len(manifests) == 0 {
		return fmt.Errorf("no manifests found in ManifestWork")
	}
	vm, _ := manifests[0].(map[string]interface{})
	spec, _ := vm["spec"].(map[string]interface{})
	if spec == nil {
		return fmt.Errorf("VM spec not found in manifest")
	}
	spec["running"] = running
	_, err = m.client.Update(ctx, client.GVRManifestWork, cluster, obj)
	return err
}

func (m *Manager) Migrate(ctx context.Context, name, cluster string) error {
	m.logger.Info("virtualization.Migrate", "name", name, "cluster", cluster)
	obj, err := m.client.Get(ctx, client.GVRManifestWork, cluster, mwName(name, cluster))
	if err != nil {
		return fmt.Errorf("getting ManifestWork: %w", err)
	}
	manifests, _ := obj.Object["spec"].(map[string]interface{})["workload"].(map[string]interface{})["manifests"].([]interface{})
	migration := map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachineInstanceMigration",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("migrate-%s", name),
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"vmiName": name,
		},
	}
	manifests = append(manifests, migration)
	if err := unstructured.SetNestedSlice(obj.Object, manifests, "spec", "workload", "manifests"); err != nil {
		return fmt.Errorf("setting manifests: %w", err)
	}
	_, err = m.client.Update(ctx, client.GVRManifestWork, cluster, obj)
	return err
}

func (m *Manager) Status(ctx context.Context, name, cluster string) (VMDetail, error) {
	m.logger.Info("virtualization.Status", "name", name, "cluster", cluster)
	obj, err := m.client.Get(ctx, client.GVRManifestWork, cluster, mwName(name, cluster))
	if err != nil {
		return VMDetail{}, fmt.Errorf("getting ManifestWork: %w", err)
	}

	detail := VMDetail{
		VMInfo: VMInfo{
			Name:    name,
			Cluster: cluster,
			Status:  "Unknown",
		},
	}

	manifests, _ := obj.Object["spec"].(map[string]interface{})["workload"].(map[string]interface{})["manifests"].([]interface{})
	if len(manifests) > 0 {
		vm, _ := manifests[0].(map[string]interface{})
		spec, _ := vm["spec"].(map[string]interface{})
		if spec != nil {
			if running, ok := spec["running"].(bool); ok {
				if running {
					detail.Status = "Running"
				} else {
					detail.Status = "Stopped"
				}
			}
			tmpl, _ := spec["template"].(map[string]interface{})
			domain, _ := tmpl["spec"].(map[string]interface{})["domain"].(map[string]interface{})
			if cpu, ok := domain["cpu"].(map[string]interface{}); ok {
				if cores, ok := cpu["cores"].(int64); ok {
					detail.CPU = fmt.Sprintf("%d", cores)
				}
			}
			if mem, ok := domain["memory"].(map[string]interface{}); ok {
				if guest, ok := mem["guest"].(string); ok {
					detail.Memory = guest
				}
			}
		}
	}

	statusMap, _ := obj.Object["status"].(map[string]interface{})
	var conditions []interface{}
	if statusMap != nil {
		conditions, _ = statusMap["conditions"].([]interface{})
	}
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "Applied" && cond["status"] == "True" {
			detail.Status = "Applied"
		}
	}

	annotations := obj.GetAnnotations()
	detail.Image = annotations["acmlab.redhat.com/vm-image"]
	detail.DiskSize = annotations["acmlab.redhat.com/vm-disk"]

	return detail, nil
}

func cnvPolicyName(cluster string) string {
	return fmt.Sprintf("%s-%s", CNVPolicyPrefix, cluster)
}

type CNVInstallStatus struct {
	Cluster   string `json:"cluster"`
	Compliant string `json:"compliant"`
}

func (m *Manager) EnsureCNVOperator(ctx context.Context, cluster string) error {
	m.logger.Info("virtualization.EnsureCNVOperator", "cluster", cluster)

	placement := buildCNVPlacement(cluster)
	if err := m.client.CreateIfNotExists(ctx, client.GVRPlacement, PolicyNamespace, placement); err != nil {
		return fmt.Errorf("creating CNV placement: %w", err)
	}

	policy := buildCNVPolicy(cluster)
	if err := m.client.CreateIfNotExists(ctx, client.GVRPolicy, PolicyNamespace, policy); err != nil {
		return fmt.Errorf("creating CNV policy: %w", err)
	}

	binding := buildCNVPlacementBinding(cluster)
	if err := m.client.CreateIfNotExists(ctx, client.GVRPlacementBinding, PolicyNamespace, binding); err != nil {
		return fmt.Errorf("creating CNV placement binding: %w", err)
	}

	return nil
}

func (m *Manager) CNVStatus(ctx context.Context, cluster string) (*CNVInstallStatus, error) {
	m.logger.Info("virtualization.CNVStatus", "cluster", cluster)
	name := cnvPolicyName(cluster)
	obj, err := m.client.Get(ctx, client.GVRPolicy, PolicyNamespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &CNVInstallStatus{Cluster: cluster, Compliant: "NotInstalled"}, nil
		}
		return nil, fmt.Errorf("getting CNV policy: %w", err)
	}
	status, _ := obj.Object["status"].(map[string]interface{})
	compliant := "Pending"
	if status != nil {
		if c, ok := status["compliant"].(string); ok {
			compliant = c
		}
	}
	return &CNVInstallStatus{Cluster: cluster, Compliant: compliant}, nil
}

func (m *Manager) RemoveCNVOperator(ctx context.Context, cluster string) error {
	m.logger.Info("virtualization.RemoveCNVOperator", "cluster", cluster)
	name := cnvPolicyName(cluster)
	_ = m.client.DeleteIfExists(ctx, client.GVRPlacementBinding, PolicyNamespace, name+"-placement-binding")
	_ = m.client.DeleteIfExists(ctx, client.GVRPolicy, PolicyNamespace, name)
	_ = m.client.DeleteIfExists(ctx, client.GVRPlacement, PolicyNamespace, name+"-placement")
	return nil
}

func (m *Manager) List(ctx context.Context) ([]VMInfo, error) {
	m.logger.Info("virtualization.List")
	list, err := m.client.Dynamic.Resource(client.GVRManifestWork).
		Namespace("").
		List(ctx, metav1.ListOptions{
			LabelSelector: LabelVM + "=true",
		})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing VM ManifestWorks: %w", err)
	}
	var vms []VMInfo
	for _, item := range list.Items {
		labels := item.GetLabels()
		status := "Unknown"
		statusMap, _ := item.Object["status"].(map[string]interface{})
		var conditions []interface{}
		if statusMap != nil {
			conditions, _ = statusMap["conditions"].([]interface{})
		}
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cond["type"] == "Applied" && cond["status"] == "True" {
				status = "Applied"
			}
		}
		vms = append(vms, VMInfo{
			Name:    labels[LabelVMName],
			Cluster: item.GetNamespace(),
			Status:  status,
		})
	}
	return vms, nil
}
