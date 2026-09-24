package provisioning

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

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
	SSHKey             string
	SSHPrivateKey      string
	IBMCloudAPIKey     string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	PullSecret         string
	ManifestsDir       string
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
	logger *slog.Logger

	iamURL string // override for testing; defaults to https://iam.cloud.ibm.com
	vpcURL string // override for testing; defaults to https://{region}.iaas.cloud.ibm.com
}

func New(c *client.Client, cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{client: c, cfg: cfg, logger: logger}
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
		if opts.Platform == "aws" {
			opts.Region = m.cfg.AWSRegion
		} else {
			opts.Region = m.cfg.IBMCloudRegion
		}
	}
	if opts.BaseDomain == "" {
		if opts.Platform == "aws" && m.cfg.AWSBaseDomain != "" {
			opts.BaseDomain = m.cfg.AWSBaseDomain
		} else {
			opts.BaseDomain = m.cfg.BaseDomain
		}
	}
	if opts.ImageSet == "" {
		opts.ImageSet = m.cfg.ClusterImageSet
	}
	if opts.WorkerType == "" {
		if opts.Platform == "aws" {
			opts.WorkerType = "m5.large"
		} else {
			opts.WorkerType = m.cfg.DefaultWorkerType
		}
	}
	if opts.MasterType == "" {
		if opts.Platform == "aws" {
			opts.MasterType = "m5.xlarge"
		} else {
			opts.MasterType = m.cfg.DefaultMasterType
		}
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
	m.logger.Info("provisioning.Create", "cluster", opts.Name)
	m.applyDefaults(&opts)

	if !supportedPlatforms[opts.Platform] {
		return fmt.Errorf("unsupported platform %q (supported: ibmcloud, aws, gcp, azure)", opts.Platform)
	}
	if opts.Platform == "ibmcloud" && opts.IBMCloudAPIKey == "" {
		return fmt.Errorf("IBM Cloud API key is required (set IBMCLOUD_API_KEY or pass --api-key)")
	}
	if opts.Platform == "aws" && (opts.AWSAccessKeyID == "" || opts.AWSSecretAccessKey == "") {
		return fmt.Errorf("AWS credentials are required (set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY or place in ~/.aws/credentials)")
	}
	if opts.PullSecret == "" {
		return fmt.Errorf("pull secret is required (set ACM_PULL_SECRET_PATH or pass --pull-secret)")
	}

	ns := buildNamespace(opts.Name)
	if err := m.client.CreateIfNotExists(ctx, client.GVRNamespace, "", ns); err != nil {
		return fmt.Errorf("creating namespace %s: %w", opts.Name, err)
	}

	creds := buildCredentialsSecret(opts.Name, opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Name, creds); err != nil {
		return fmt.Errorf("creating credentials secret: %w", err)
	}

	pull := buildPullSecret(opts.Name, opts.PullSecret)
	if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Name, pull); err != nil {
		return fmt.Errorf("creating pull secret: %w", err)
	}

	installCfg := buildInstallConfigSecret(opts.Name, opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Name, installCfg); err != nil {
		return fmt.Errorf("creating install-config secret: %w", err)
	}

	if opts.SSHPrivateKey != "" {
		sshKey := buildSSHPrivateKeySecret(opts.Name, opts.SSHPrivateKey)
		if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Name, sshKey); err != nil {
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
		if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Name, manifestsObj); err != nil {
			return fmt.Errorf("creating manifests secret: %w", err)
		}
	}

	cd := buildClusterDeployment(opts)
	if err := m.client.CreateIfNotExists(ctx, client.GVRClusterDeployment, opts.Name, cd); err != nil {
		return fmt.Errorf("creating ClusterDeployment %s: %w", opts.Name, err)
	}

	mc := buildManagedCluster(opts.Name, opts.Platform)
	if err := m.client.CreateIfNotExists(ctx, client.GVRManagedCluster, "", mc); err != nil {
		return fmt.Errorf("creating ManagedCluster %s: %w", opts.Name, err)
	}

	kac := buildKlusterletAddonConfig(opts.Name)
	if err := m.client.CreateIfNotExists(ctx, client.GVRKlusterletAddonConfig, opts.Name, kac); err != nil {
		return fmt.Errorf("creating KlusterletAddonConfig %s: %w", opts.Name, err)
	}

	return nil
}

func (m *Manager) Destroy(ctx context.Context, name string) error {
	m.logger.Info("provisioning.Destroy", "cluster", name)

	_, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("cluster %s not found (no ClusterDeployment)", name)
		}
		return fmt.Errorf("checking ClusterDeployment %s: %w", name, err)
	}

	if err := m.client.Delete(ctx, client.GVRClusterDeployment, name, name); err != nil {
		return fmt.Errorf("deleting ClusterDeployment %s: %w", name, err)
	}

	if err := m.client.DeleteIfExists(ctx, client.GVRManagedCluster, "", name); err != nil {
		m.logger.Error("failed to delete ManagedCluster (manual cleanup may be needed)", "cluster", name, "error", err)
	}

	if m.cfg.IBMCloudAPIKey != "" {
		cleanupIBMCloudCredentials(m.cfg.IBMCloudAPIKey, name)
	}
	return nil
}

func (m *Manager) DestroyIfFailed(ctx context.Context, name string) (bool, error) {
	info, err := m.Status(ctx, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if info.FailureReason != "" {
		m.logger.Info("provisioning.DestroyIfFailed: cleaning up failed cluster", "cluster", name, "reason", info.FailureReason)
		return true, m.Destroy(ctx, name)
	}
	return false, nil
}

func (m *Manager) Status(ctx context.Context, name string) (*ClusterInfo, error) {
	m.logger.Info("provisioning.Status", "cluster", name)
	obj, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err != nil {
		return nil, fmt.Errorf("getting ClusterDeployment %s: %w", name, err)
	}
	return parseClusterInfo(obj.Object), nil
}

func (m *Manager) List(ctx context.Context) ([]ClusterInfo, error) {
	m.logger.Info("provisioning.List")
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
	m.logger.Info("provisioning.WaitForProvision", "cluster", name)
	if timeout == 0 {
		timeout = m.cfg.ProvisionTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("timed out waiting for cluster %s to provision", name)
		}

		// Check current state before watching (handles already-installed case and gets resourceVersion)
		current, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
		if err != nil {
			return fmt.Errorf("getting ClusterDeployment %s: %w", name, err)
		}
		info := parseClusterInfo(current.Object)
		if info.FailureReason != "" {
			return fmt.Errorf("cluster %s provisioning failed: %s", name, info.FailureReason)
		}
		if info.Installed {
			return nil
		}

		rv := current.GetResourceVersion()
		watcher, err := m.client.Watch(ctx, client.GVRClusterDeployment, name, metav1.ListOptions{
			FieldSelector:   "metadata.name=" + name,
			ResourceVersion: rv,
		})
		if err != nil {
			return fmt.Errorf("watching ClusterDeployment %s: %w", name, err)
		}

		done, watchErr := m.drainProvisionWatch(ctx, watcher, name)
		watcher.Stop()
		if done {
			return watchErr
		}
		// Watch channel closed (server-side timeout) — reconnect
		m.logger.Info("provisioning.WaitForProvision: watch reconnecting", "cluster", name)
	}
}

// drainProvisionWatch reads events until installed, failed, context cancelled, or channel closed.
// Returns (true, err) when a terminal state is reached, (false, nil) when the channel closed and reconnection is needed.
func (m *Manager) drainProvisionWatch(ctx context.Context, watcher watch.Interface, name string) (bool, error) {
	for {
		select {
		case <-ctx.Done():
			return true, fmt.Errorf("timed out waiting for cluster %s to provision", name)
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return false, nil
			}
			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			info := parseClusterInfo(obj.Object)
			if info.FailureReason != "" {
				return true, fmt.Errorf("cluster %s provisioning failed: %s", name, info.FailureReason)
			}
			if info.Installed {
				return true, nil
			}
		}
	}
}

func (m *Manager) ListImageSets(ctx context.Context) ([]ImageSetInfo, error) {
	m.logger.Info("provisioning.ListImageSets")
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
		if !info.Installed {
			_, hasTimestamp := status["installedTimestamp"]
			info.Installed = hasTimestamp
		}
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
