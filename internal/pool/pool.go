package pool

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
	"github.com/pablofelix/acm-caas-poc/internal/lifecycle"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

type PoolOpts struct {
	Name               string
	Namespace          string
	Size               int
	Platform           string
	Region             string
	ImageSet           string
	BaseDomain         string
	PullSecret         string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	IBMCloudAPIKey     string
	WorkerType         string
	MasterType         string
}

type PoolInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Size      int    `json:"size"`
	Ready     int    `json:"ready"`
	Claimed   int    `json:"claimed"`
	Standby   int    `json:"standby"`
}

type ClaimInfo struct {
	Name      string `json:"name"`
	Pool      string `json:"pool"`
	Namespace string `json:"namespace"`
	Cluster   string `json:"cluster"`
	Status    string `json:"status"`
}

type Manager struct {
	client       *client.Client
	cfg          config.Config
	logger       *slog.Logger
	provisioning *provisioning.Manager
	lifecycle    *lifecycle.Manager
}

func New(c *client.Client, cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{client: c, cfg: cfg, logger: logger}
}

func NewWithManagers(c *client.Client, cfg config.Config, logger *slog.Logger, provMgr *provisioning.Manager, lcMgr *lifecycle.Manager) *Manager {
	return &Manager{
		client:       c,
		cfg:          cfg,
		logger:       logger,
		provisioning: provMgr,
		lifecycle:    lcMgr,
	}
}

func (m *Manager) CreatePool(ctx context.Context, opts PoolOpts) error {
	m.logger.Info("pool.CreatePool", "name", opts.Name)
	if opts.Namespace == "" {
		opts.Namespace = opts.Name
	}
	if opts.Size <= 0 {
		opts.Size = 1
	}

	ns := buildNamespace(opts.Namespace)
	if err := m.client.CreateIfNotExists(ctx, client.GVRNamespace, "", ns); err != nil {
		return fmt.Errorf("ensuring namespace %s: %w", opts.Namespace, err)
	}

	if opts.PullSecret != "" {
		ps := buildPullSecret(opts.Namespace, opts.PullSecret)
		if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Namespace, ps); err != nil {
			return fmt.Errorf("creating pull secret: %w", err)
		}
	}

	if opts.Platform == "aws" && opts.AWSAccessKeyID != "" {
		creds := buildAWSCredentialsSecret(opts.Namespace, opts.AWSAccessKeyID, opts.AWSSecretAccessKey)
		if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Namespace, creds); err != nil {
			return fmt.Errorf("creating AWS credentials: %w", err)
		}
	}

	if opts.Platform == "ibmcloud" && opts.IBMCloudAPIKey != "" {
		creds := buildIBMCredentialsSecret(opts.Namespace, opts.IBMCloudAPIKey)
		if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, opts.Namespace, creds); err != nil {
			return fmt.Errorf("creating IBM credentials: %w", err)
		}
	}

	obj := buildClusterPool(opts)
	return m.client.CreateIfNotExists(ctx, client.GVRClusterPool, opts.Namespace, obj)
}

func (m *Manager) ListPools(ctx context.Context) ([]PoolInfo, error) {
	m.logger.Info("pool.ListPools")
	list, err := m.client.List(ctx, client.GVRClusterPool, "", "")
	if err != nil {
		return nil, fmt.Errorf("listing ClusterPools: %w", err)
	}
	infos := make([]PoolInfo, 0, len(list.Items))
	for _, item := range list.Items {
		infos = append(infos, parsePoolInfo(item.Object))
	}
	return infos, nil
}

func (m *Manager) GetPool(ctx context.Context, name, namespace string) (*PoolInfo, error) {
	m.logger.Info("pool.GetPool", "name", name)
	if namespace == "" {
		namespace = name
	}
	obj, err := m.client.Get(ctx, client.GVRClusterPool, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("getting ClusterPool %s: %w", name, err)
	}
	info := parsePoolInfo(obj.Object)
	return &info, nil
}

func (m *Manager) DeletePool(ctx context.Context, name, namespace string) error {
	m.logger.Info("pool.DeletePool", "name", name)
	if namespace == "" {
		namespace = name
	}
	_, err := m.client.Get(ctx, client.GVRClusterPool, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("ClusterPool %s not found", name)
		}
		return fmt.Errorf("checking ClusterPool %s: %w", name, err)
	}
	return m.client.DeleteIfExists(ctx, client.GVRClusterPool, namespace, name)
}

func (m *Manager) Claim(ctx context.Context, poolName, namespace, claimName, ttl string) (*ClaimInfo, error) {
	m.logger.Info("pool.Claim", "pool", poolName, "claim", claimName)
	if namespace == "" {
		namespace = poolName
	}
	if claimName == "" {
		claimName = poolName + "-claim"
	}

	obj := buildClusterClaim(poolName, claimName, namespace, ttl)
	if err := m.client.CreateIfNotExists(ctx, client.GVRClusterClaim, namespace, obj); err != nil {
		return nil, fmt.Errorf("creating ClusterClaim %s: %w", claimName, err)
	}

	created, err := m.client.Get(ctx, client.GVRClusterClaim, namespace, claimName)
	if err != nil {
		return nil, fmt.Errorf("reading ClusterClaim %s: %w", claimName, err)
	}
	info := parseClaimInfo(created.Object)
	return &info, nil
}

func (m *Manager) ReleaseClaim(ctx context.Context, claimName, namespace string) error {
	m.logger.Info("pool.ReleaseClaim", "claim", claimName)
	if namespace == "" {
		return fmt.Errorf("namespace is required to release a claim")
	}
	_, err := m.client.Get(ctx, client.GVRClusterClaim, namespace, claimName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("ClusterClaim %s not found in namespace %s", claimName, namespace)
		}
		return fmt.Errorf("checking ClusterClaim %s: %w", claimName, err)
	}
	return m.client.DeleteIfExists(ctx, client.GVRClusterClaim, namespace, claimName)
}

func (m *Manager) ListClaims(ctx context.Context, namespace string) ([]ClaimInfo, error) {
	m.logger.Info("pool.ListClaims", "namespace", namespace)
	list, err := m.client.List(ctx, client.GVRClusterClaim, namespace, "")
	if err != nil {
		return nil, fmt.Errorf("listing ClusterClaims: %w", err)
	}
	infos := make([]ClaimInfo, 0, len(list.Items))
	for _, item := range list.Items {
		infos = append(infos, parseClaimInfo(item.Object))
	}
	return infos, nil
}

func parsePoolInfo(obj map[string]interface{}) PoolInfo {
	info := PoolInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		info.Namespace, _ = meta["namespace"].(string)
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if size, ok := spec["size"].(int64); ok {
			info.Size = int(size)
		}
	}
	status, _ := obj["status"].(map[string]interface{})
	if status != nil {
		if ready, ok := status["ready"].(int64); ok {
			info.Ready = int(ready)
		}
		if standby, ok := status["standby"].(int64); ok {
			info.Standby = int(standby)
		}
		if claimed, ok := status["claimedClusterCount"].(int64); ok {
			info.Claimed = int(claimed)
		}
	}
	return info
}

func parseClaimInfo(obj map[string]interface{}) ClaimInfo {
	info := ClaimInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		info.Namespace, _ = meta["namespace"].(string)
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		info.Pool, _ = spec["clusterPoolName"].(string)
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
		if cond["type"] == "ClusterRunning" && cond["status"] == "True" {
			info.Status = "Running"
		}
		if cond["type"] == "Pending" && cond["status"] == "True" {
			info.Status = "Pending"
		}
	}
	if cd, ok := status["clusterDeploymentRef"].(map[string]interface{}); ok {
		info.Cluster, _ = cd["name"].(string)
	}
	return info
}

func buildNamespace(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

func buildClusterPool(opts PoolOpts) *unstructured.Unstructured {
	platform := opts.Platform
	if platform == "" {
		platform = "ibmcloud"
	}
	region := opts.Region
	if region == "" {
		region = "us-south"
	}
	baseDomain := opts.BaseDomain
	if baseDomain == "" {
		baseDomain = "example.com"
	}

	platformBlock := map[string]interface{}{
		"region": region,
	}
	credsSecretName := opts.Namespace + "-aws-creds"
	if platform == "ibmcloud" {
		credsSecretName = opts.Namespace + "-ibmcloud-creds"
	}
	platformBlock["credentialsSecretRef"] = map[string]interface{}{
		"name": credsSecretName,
	}

	spec := map[string]interface{}{
		"size":       int64(opts.Size),
		"baseDomain": baseDomain,
		"imageSetRef": map[string]interface{}{
			"name": opts.ImageSet,
		},
		"platform": map[string]interface{}{
			platform: platformBlock,
		},
		"installAttemptsLimit": int64(6),
		"hibernationConfig": map[string]interface{}{
			"resumeTimeout": "20m",
		},
		"pullSecretRef": map[string]interface{}{
			"name": opts.Namespace + "-pull-secret",
		},
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hive.openshift.io/v1",
			"kind":       "ClusterPool",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": spec,
		},
	}
}

func buildPullSecret(namespace, pullSecret string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-pull-secret",
				"namespace": namespace,
			},
			"type": "kubernetes.io/dockerconfigjson",
			"stringData": map[string]interface{}{
				".dockerconfigjson": pullSecret,
			},
		},
	}
}

func buildAWSCredentialsSecret(namespace, accessKeyID, secretAccessKey string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-aws-creds",
				"namespace": namespace,
			},
			"type": "Opaque",
			"stringData": map[string]interface{}{
				"aws_access_key_id":     accessKeyID,
				"aws_secret_access_key": secretAccessKey,
			},
		},
	}
}

func buildIBMCredentialsSecret(namespace, apiKey string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-ibmcloud-creds",
				"namespace": namespace,
			},
			"type": "Opaque",
			"stringData": map[string]interface{}{
				"ibmcloud_api_key": apiKey,
			},
		},
	}
}

func buildClusterClaim(poolName, claimName, namespace, ttl string) *unstructured.Unstructured {
	spec := map[string]interface{}{
		"clusterPoolName": poolName,
	}
	if ttl != "" {
		spec["lifetime"] = ttl
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hive.openshift.io/v1",
			"kind":       "ClusterClaim",
			"metadata": map[string]interface{}{
				"name":      claimName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": spec,
		},
	}
}
