package provisioning

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type CAPIClusterOpts struct {
	Name                 string
	Namespace            string
	InfraProvider        string
	KubernetesVersion    string
	WorkerReplicas       int64
	ControlPlaneReplicas int64
	PullSecret           string

	Region         string
	SSHKeyName     string
	InstanceType   string
	AMI            string
	RootVolumeSize int64
}

type CAPIClusterInfo struct {
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace"`
	Phase             string   `json:"phase"`
	Ready             bool     `json:"ready"`
	KubernetesVersion string   `json:"kubernetesVersion,omitempty"`
	Conditions        []string `json:"conditions,omitempty"`
}

func (m *Manager) applyCAPIDefaults(opts *CAPIClusterOpts) {
	if opts.Namespace == "" {
		opts.Namespace = opts.Name
	}
	if opts.InfraProvider == "" {
		if m.cfg.Platform == "aws" {
			opts.InfraProvider = "aws"
		} else {
			opts.InfraProvider = "docker"
		}
	}
	if opts.KubernetesVersion == "" {
		opts.KubernetesVersion = "v1.34.8"
	}
	if opts.WorkerReplicas == 0 {
		opts.WorkerReplicas = 2
	}
	if opts.ControlPlaneReplicas == 0 {
		opts.ControlPlaneReplicas = 1
	}
	if opts.InfraProvider == "aws" {
		if opts.Region == "" {
			opts.Region = m.cfg.AWSRegion
		}
		if opts.InstanceType == "" {
			opts.InstanceType = "t3.large"
		}
		if opts.RootVolumeSize == 0 {
			opts.RootVolumeSize = 80
		}
	}
}

func (m *Manager) CreateCAPI(ctx context.Context, opts CAPIClusterOpts) error {
	m.logger.Info("provisioning.CreateCAPI", "cluster", opts.Name)
	m.applyCAPIDefaults(&opts)

	if opts.Name == "" {
		return fmt.Errorf("cluster name is required")
	}

	ns := buildNamespace(opts.Namespace)
	if err := m.client.CreateIfNotExists(ctx, client.GVRNamespace, "", ns); err != nil {
		return fmt.Errorf("creating namespace %s: %w", opts.Namespace, err)
	}

	if opts.InfraProvider == "aws" {
		return m.createCAPIAWS(ctx, opts)
	}

	cluster := buildCAPICluster(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRCAPICluster, opts.Namespace, cluster); err != nil {
		return fmt.Errorf("creating CAPI Cluster %s: %w", opts.Name, err)
	}

	md := buildCAPIMachineDeployment(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRCAPIMachineDeployment, opts.Namespace, md); err != nil {
		return fmt.Errorf("creating CAPI MachineDeployment %s-workers: %w", opts.Name, err)
	}

	mc := buildCAPIManagedCluster(opts.Name)
	if err := m.client.CreateIfNotExists(ctx, client.GVRManagedCluster, "", mc); err != nil {
		return fmt.Errorf("creating ManagedCluster %s: %w", opts.Name, err)
	}

	return nil
}

func (m *Manager) createCAPIAWS(ctx context.Context, opts CAPIClusterOpts) error {
	if opts.Region == "" {
		return fmt.Errorf("AWS region is required for CAPI AWS provider")
	}

	awsCluster := buildAWSCluster(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRAWSCluster, opts.Namespace, awsCluster); err != nil {
		return fmt.Errorf("creating AWSCluster %s: %w", opts.Name, err)
	}

	cpMachineTemplate := buildAWSMachineTemplate(opts.Name+"-control-plane", opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRAWSMachineTemplate, opts.Namespace, cpMachineTemplate); err != nil {
		return fmt.Errorf("creating AWSMachineTemplate %s-control-plane: %w", opts.Name, err)
	}

	kcp := buildKubeadmControlPlane(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRKubeadmControlPlane, opts.Namespace, kcp); err != nil {
		return fmt.Errorf("creating KubeadmControlPlane %s-control-plane: %w", opts.Name, err)
	}

	cluster := buildCAPIClusterForAWS(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRCAPICluster, opts.Namespace, cluster); err != nil {
		return fmt.Errorf("creating CAPI Cluster %s: %w", opts.Name, err)
	}

	workerMachineTemplate := buildAWSMachineTemplate(opts.Name+"-workers", opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRAWSMachineTemplate, opts.Namespace, workerMachineTemplate); err != nil {
		return fmt.Errorf("creating AWSMachineTemplate %s-workers: %w", opts.Name, err)
	}

	bootstrapTemplate := buildKubeadmConfigTemplate(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRKubeadmConfigTemplate, opts.Namespace, bootstrapTemplate); err != nil {
		return fmt.Errorf("creating KubeadmConfigTemplate %s-workers: %w", opts.Name, err)
	}

	md := buildCAPIMachineDeployment(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRCAPIMachineDeployment, opts.Namespace, md); err != nil {
		return fmt.Errorf("creating CAPI MachineDeployment %s-workers: %w", opts.Name, err)
	}

	mc := buildCAPIManagedCluster(opts.Name)
	if err := m.client.CreateIfNotExists(ctx, client.GVRManagedCluster, "", mc); err != nil {
		return fmt.Errorf("creating ManagedCluster %s: %w", opts.Name, err)
	}

	return nil
}

func (m *Manager) DestroyCAPI(ctx context.Context, name string) error {
	m.logger.Info("provisioning.DestroyCAPI", "cluster", name)

	if err := m.client.DeleteIfExists(ctx, client.GVRCAPIMachineDeployment, name, name+"-workers"); err != nil {
		return fmt.Errorf("deleting CAPI MachineDeployment %s-workers: %w", name, err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRKubeadmConfigTemplate, name, name+"-workers"); err != nil {
		m.logger.Warn("failed to delete KubeadmConfigTemplate", "name", name+"-workers", "error", err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRAWSMachineTemplate, name, name+"-workers"); err != nil {
		m.logger.Warn("failed to delete AWSMachineTemplate", "name", name+"-workers", "error", err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRKubeadmControlPlane, name, name+"-control-plane"); err != nil {
		m.logger.Warn("failed to delete KubeadmControlPlane", "name", name+"-control-plane", "error", err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRAWSMachineTemplate, name, name+"-control-plane"); err != nil {
		m.logger.Warn("failed to delete AWSMachineTemplate", "name", name+"-control-plane", "error", err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRCAPICluster, name, name); err != nil {
		return fmt.Errorf("deleting CAPI Cluster %s: %w", name, err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRAWSCluster, name, name); err != nil {
		m.logger.Warn("failed to delete AWSCluster", "name", name, "error", err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRManagedCluster, "", name); err != nil {
		return fmt.Errorf("deleting ManagedCluster %s: %w", name, err)
	}

	return nil
}

func (m *Manager) StatusCAPI(ctx context.Context, name string) (*CAPIClusterInfo, error) {
	m.logger.Info("provisioning.StatusCAPI", "cluster", name)
	obj, err := m.client.Get(ctx, client.GVRCAPICluster, name, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("CAPI cluster %s not found", name)
		}
		return nil, fmt.Errorf("getting CAPI Cluster %s: %w", name, err)
	}
	return parseCAPIClusterInfo(obj.Object), nil
}

func (m *Manager) ListCAPI(ctx context.Context) ([]CAPIClusterInfo, error) {
	m.logger.Info("provisioning.ListCAPI")
	list, err := m.client.List(ctx, client.GVRCAPICluster, "", "acmlab.redhat.com/managed")
	if err != nil {
		return nil, fmt.Errorf("listing CAPI Clusters: %w", err)
	}
	clusters := make([]CAPIClusterInfo, 0, len(list.Items))
	for _, item := range list.Items {
		clusters = append(clusters, *parseCAPIClusterInfo(item.Object))
	}
	return clusters, nil
}
