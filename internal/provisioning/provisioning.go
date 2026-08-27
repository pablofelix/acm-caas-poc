package provisioning

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

type ClusterOpts struct {
	Name           string
	Platform       string
	BaseDomain     string
	Region         string
	ImageSet       string
	WorkerType     string
	MasterType     string
	WorkerReplicas int64
	MasterReplicas int64
	SSHKey         string
	SSHPrivateKey  string
	IBMCloudAPIKey string
	PullSecret     string
	ManifestsDir   string
}

type ClusterInfo struct {
	Name          string
	Namespace     string
	BaseDomain    string
	Region        string
	ImageSet      string
	Installed     bool
	Provisioned   bool
	FailureReason string
	Conditions    []string
}

type Manager struct {
	client *client.Client
	cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Manager {
	return &Manager{client: c, cfg: cfg}
}

var supportedPlatforms = map[string]bool{
	"ibmcloud": true,
	"aws":      true,
	"gcp":      true,
	"azure":    true,
}

func (m *Manager) applyDefaults(opts *ClusterOpts) {
	if opts.Platform == "" {
		opts.Platform = m.cfg.Platform
	}
	if opts.Region == "" {
		opts.Region = m.cfg.IBMCloudRegion
	}
	if opts.BaseDomain == "" {
		opts.BaseDomain = m.cfg.BaseDomain
	}
	if opts.ImageSet == "" {
		opts.ImageSet = m.cfg.ClusterImageSet
	}
	if opts.WorkerType == "" {
		opts.WorkerType = m.cfg.DefaultWorkerType
	}
	if opts.MasterType == "" {
		opts.MasterType = m.cfg.DefaultMasterType
	}
	if opts.WorkerReplicas == 0 {
		opts.WorkerReplicas = int64(m.cfg.DefaultWorkerReplicas)
	}
	if opts.MasterReplicas == 0 {
		opts.MasterReplicas = int64(m.cfg.DefaultMasterReplicas)
	}
	if opts.IBMCloudAPIKey == "" {
		opts.IBMCloudAPIKey = m.cfg.IBMCloudAPIKey
	}
}

func (m *Manager) Create(ctx context.Context, opts ClusterOpts) error {
	m.applyDefaults(&opts)

	if !supportedPlatforms[opts.Platform] {
		return fmt.Errorf("unsupported platform %q (supported: ibmcloud, aws, gcp, azure)", opts.Platform)
	}
	if opts.Platform == "ibmcloud" && opts.IBMCloudAPIKey == "" {
		return fmt.Errorf("IBM Cloud API key is required (set IBMCLOUD_API_KEY or pass --api-key)")
	}
	if opts.PullSecret == "" {
		return fmt.Errorf("pull secret is required (set ACM_PULL_SECRET_PATH or pass --pull-secret)")
	}

	ns := buildNamespace(opts.Name)
	if err := m.createIfNotExists(ctx, client.GVRNamespace, "", ns); err != nil {
		return fmt.Errorf("creating namespace %s: %w", opts.Name, err)
	}

	creds := buildCredentialsSecret(opts.Name, opts.IBMCloudAPIKey)
	if err := m.createIfNotExists(ctx, client.GVRSecret, opts.Name, creds); err != nil {
		return fmt.Errorf("creating credentials secret: %w", err)
	}

	pull := buildPullSecret(opts.Name, opts.PullSecret)
	if err := m.createIfNotExists(ctx, client.GVRSecret, opts.Name, pull); err != nil {
		return fmt.Errorf("creating pull secret: %w", err)
	}

	installCfg := buildInstallConfigSecret(opts.Name, opts)
	if err := m.createIfNotExists(ctx, client.GVRSecret, opts.Name, installCfg); err != nil {
		return fmt.Errorf("creating install-config secret: %w", err)
	}

	if opts.SSHPrivateKey != "" {
		sshKey := buildSSHPrivateKeySecret(opts.Name, opts.SSHPrivateKey)
		if err := m.createIfNotExists(ctx, client.GVRSecret, opts.Name, sshKey); err != nil {
			return fmt.Errorf("creating ssh private key secret: %w", err)
		}
	}

	if opts.Platform == "ibmcloud" {
		var manifestsObj *unstructured.Unstructured
		if opts.ManifestsDir != "" {
			obj, err := buildManifestsSecret(opts.Name, opts.ManifestsDir)
			if err != nil {
				return fmt.Errorf("building manifests secret: %w", err)
			}
			manifestsObj = obj
		} else {
			componentCreds, err := generateIBMCloudCredentials(opts.IBMCloudAPIKey, opts.Name)
			if err != nil {
				return fmt.Errorf("generating IBM Cloud IAM credentials: %w", err)
			}
			yamls := buildManifestYAMLs(componentCreds)
			manifestsObj = buildManifestsSecretFromYAMLs(opts.Name, yamls)
		}
		if err := m.createIfNotExists(ctx, client.GVRSecret, opts.Name, manifestsObj); err != nil {
			return fmt.Errorf("creating manifests secret: %w", err)
		}
	}

	cd := buildClusterDeployment(opts)
	if err := m.createIfNotExists(ctx, client.GVRClusterDeployment, opts.Name, cd); err != nil {
		return fmt.Errorf("creating ClusterDeployment %s: %w", opts.Name, err)
	}

	return nil
}

func (m *Manager) Destroy(ctx context.Context, name string) error {
	err := m.client.Delete(ctx, client.GVRClusterDeployment, name, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting ClusterDeployment %s: %w", name, err)
	}
	if m.cfg.IBMCloudAPIKey != "" {
		cleanupIBMCloudCredentials(m.cfg.IBMCloudAPIKey, name)
	}
	return nil
}

func (m *Manager) Status(ctx context.Context, name string) (*ClusterInfo, error) {
	obj, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err != nil {
		return nil, fmt.Errorf("getting ClusterDeployment %s: %w", name, err)
	}
	return parseClusterInfo(obj.Object), nil
}

func (m *Manager) List(ctx context.Context) ([]ClusterInfo, error) {
	list, err := m.client.List(ctx, client.GVRClusterDeployment, "", "acmlab.redhat.com/managed")
	if err != nil {
		return nil, fmt.Errorf("listing ClusterDeployments: %w", err)
	}
	clusters := make([]ClusterInfo, 0, len(list.Items))
	for _, item := range list.Items {
		clusters = append(clusters, *parseClusterInfo(item.Object))
	}
	return clusters, nil
}

func (m *Manager) WaitForProvision(ctx context.Context, name string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = m.cfg.ProvisionTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watcher, err := m.client.Watch(ctx, client.GVRClusterDeployment, name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		return fmt.Errorf("watching ClusterDeployment %s: %w", name, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for cluster %s to provision", name)
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed for cluster %s", name)
			}
			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			info := parseClusterInfo(obj.Object)
			if info.FailureReason != "" {
				return fmt.Errorf("cluster %s provisioning failed: %s", name, info.FailureReason)
			}
			if info.Installed {
				return nil
			}
		}
	}
}

func (m *Manager) ListImageSets(ctx context.Context) ([]ImageSetInfo, error) {
	list, err := m.client.List(ctx, client.GVRClusterImageSet, "", "")
	if err != nil {
		return nil, fmt.Errorf("listing ClusterImageSets: %w", err)
	}
	sets := make([]ImageSetInfo, 0, len(list.Items))
	for _, item := range list.Items {
		sets = append(sets, parseImageSetInfo(item.Object))
	}
	return sets, nil
}

type ImageSetInfo struct {
	Name         string
	ReleaseImage string
}

func parseImageSetInfo(obj map[string]interface{}) ImageSetInfo {
	info := ImageSetInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		info.ReleaseImage, _ = spec["releaseImage"].(string)
	}
	return info
}

func (m *Manager) createIfNotExists(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
	name := obj.GetName()
	_, err := m.client.Get(ctx, gvr, namespace, name)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = m.client.Create(ctx, gvr, namespace, obj)
	return err
}

func parseClusterInfo(obj map[string]interface{}) *ClusterInfo {
	info := &ClusterInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		info.Namespace, _ = meta["namespace"].(string)
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		info.BaseDomain, _ = spec["baseDomain"].(string)
		if platform, ok := spec["platform"].(map[string]interface{}); ok {
			if ibm, ok := platform["ibmcloud"].(map[string]interface{}); ok {
				info.Region, _ = ibm["region"].(string)
			}
		}
		if prov, ok := spec["provisioning"].(map[string]interface{}); ok {
			if ref, ok := prov["imageSetRef"].(map[string]interface{}); ok {
				info.ImageSet, _ = ref["name"].(string)
			}
		}
	}
	if status, ok := obj["status"].(map[string]interface{}); ok {
		info.Installed, _ = status["installed"].(bool)
		conditions, _ := status["conditions"].([]interface{})
		for _, raw := range conditions {
			cond, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			reason, _ := cond["reason"].(string)
			info.Conditions = append(info.Conditions, fmt.Sprintf("%s=%s", condType, condStatus))
			if condType == "Provisioned" && condStatus == "True" {
				info.Provisioned = true
			}
			if condType == "ProvisionFailed" && condStatus == "True" {
				info.FailureReason = reason
			}
		}
	}
	return info
}
