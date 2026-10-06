package provisioning

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

	go m.watchAndAutoImport(ctx, opts.Name, opts.Namespace)

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

	cpMachineTemplate := buildAWSMachineTemplate(opts.Name+"-control-plane", opts, "control-plane.cluster-api-provider-aws.sigs.k8s.io")
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

	workerMachineTemplate := buildAWSMachineTemplate(opts.Name+"-workers", opts, "nodes.cluster-api-provider-aws.sigs.k8s.io")
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

	go m.watchAndAutoImport(ctx, opts.Name, opts.Namespace)

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

func (m *Manager) watchAndAutoImport(ctx context.Context, name, namespace string) {
	m.logger.Info("provisioning.watchAndAutoImport", "cluster", name, "namespace", namespace)
	secretName := name + "-kubeconfig"
	timeout := 15 * time.Minute
	interval := 10 * time.Second
	deadline := time.After(timeout)

	for {
		obj, err := m.client.Get(ctx, client.GVRSecret, namespace, secretName)
		if err == nil {
			kubeconfig, found, _ := unstructured.NestedString(obj.Object, "data", "value")
			if found && kubeconfig != "" {
				decoded, err := base64.StdEncoding.DecodeString(kubeconfig)
				if err != nil {
					m.logger.Error("failed to decode CAPI kubeconfig", "cluster", name, "error", err)
					return
				}
				if err := m.createCAPIAutoImportSecret(ctx, namespace, decoded); err != nil {
					m.logger.Error("failed to create auto-import secret", "cluster", name, "error", err)
					return
				}
				m.logger.Info("auto-import secret created from CAPI kubeconfig", "cluster", name)
				return
			}
		}
		select {
		case <-deadline:
			m.logger.Warn("timed out waiting for CAPI kubeconfig secret", "cluster", name)
			return
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (m *Manager) createCAPIAutoImportSecret(ctx context.Context, namespace string, kubeconfig []byte) error {
	secret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "auto-import-secret",
				"namespace": namespace,
			},
			"type": "Opaque",
			"stringData": map[string]interface{}{
				"kubeconfig": string(kubeconfig),
			},
		},
	}
	_, err := m.client.Create(ctx, client.GVRSecret, namespace, secret)
	if apierrors.IsAlreadyExists(err) {
		existing, getErr := m.client.Get(ctx, client.GVRSecret, namespace, "auto-import-secret")
		if getErr != nil {
			return getErr
		}
		if err := unstructured.SetNestedField(existing.Object, base64.StdEncoding.EncodeToString(kubeconfig), "data", "kubeconfig"); err != nil {
			return err
		}
		_, err = m.client.Update(ctx, client.GVRSecret, namespace, existing)
		return err
	}
	return err
}

// EnsureCAPIAddons creates ClusterResourceSets on the hub that auto-install
// Calico CNI and AWS CCM on new CAPI clusters matching the label selector.
// The ConfigMaps are fetched once and stored on the hub; CAPI applies them
// to each matching cluster when the kubeconfig becomes available.
func (m *Manager) EnsureCAPIAddons(ctx context.Context, infraProvider string) error {
	m.logger.Info("provisioning.EnsureCAPIAddons", "infraProvider", infraProvider)

	cniCM := buildAddonConfigMap("capi-addon-calico", "capi-addons", calicoManifestURL)
	if err := m.client.CreateIfNotExists(ctx, client.GVRConfigMap, "capi-addons", cniCM); err != nil {
		return fmt.Errorf("creating Calico addon ConfigMap: %w", err)
	}

	resources := []interface{}{
		map[string]interface{}{
			"kind": "ConfigMap", "name": "capi-addon-calico",
		},
	}

	if infraProvider == "aws" {
		ccmCM := buildAddonConfigMap("capi-addon-aws-ccm", "capi-addons", awsCCMKustomizeURL)
		if err := m.client.CreateIfNotExists(ctx, client.GVRConfigMap, "capi-addons", ccmCM); err != nil {
			return fmt.Errorf("creating AWS CCM addon ConfigMap: %w", err)
		}
		resources = append(resources, map[string]interface{}{
			"kind": "ConfigMap", "name": "capi-addon-aws-ccm",
		})
	}

	crs := buildClusterResourceSet("capi-addons-"+infraProvider, "capi-addons", infraProvider, resources)
	if err := m.client.CreateIfNotExists(ctx, client.GVRClusterResourceSet, "capi-addons", crs); err != nil {
		return fmt.Errorf("creating ClusterResourceSet: %w", err)
	}

	m.logger.Info("CAPI addons ensured", "infraProvider", infraProvider)
	return nil
}

const (
	calicoManifestURL  = "https://raw.githubusercontent.com/projectcalico/calico/v3.29.4/manifests/calico.yaml"
	awsCCMKustomizeURL = "https://github.com/kubernetes/cloud-provider-aws/examples/existing-cluster/base/?ref=master"
)

func buildAddonConfigMap(name, namespace, sourceURL string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"annotations": map[string]interface{}{
					"acmlab.redhat.com/source-url": sourceURL,
				},
			},
			"data": map[string]interface{}{
				"source-url": sourceURL,
			},
		},
	}
}

func buildClusterResourceSet(name, namespace, infraProvider string, resources []interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "addons.cluster.x-k8s.io/v1beta2",
			"kind":       "ClusterResourceSet",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"strategy": "ApplyOnce",
				"clusterSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"acmlab.redhat.com/managed":        "true",
						"acmlab.redhat.com/infra-provider": infraProvider,
					},
				},
				"resources": resources,
			},
		},
	}
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
