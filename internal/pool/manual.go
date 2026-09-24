package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/lifecycle"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

const (
	labelPool        = "acmlab.redhat.com/pool"
	labelPoolClaimed = "acmlab.redhat.com/pool-claimed"
	labelPoolIndex   = "acmlab.redhat.com/pool-index"

	poolConfigNS = "open-cluster-management"
)

type ManualPoolOpts struct {
	Name          string
	Size          int
	ProvisionOpts provisioning.ClusterOpts
	CredentialRef string // name of the Secret holding cloud credentials
}

func (m *Manager) CreateManualPool(ctx context.Context, opts ManualPoolOpts) error {
	m.logger.Info("pool.CreateManualPool", "name", opts.Name, "size", opts.Size)
	if m.provisioning == nil {
		return fmt.Errorf("provisioning manager required for manual pool (use NewWithManagers)")
	}
	if opts.Size <= 0 {
		opts.Size = 2
	}

	if err := m.savePoolConfig(ctx, opts); err != nil {
		return fmt.Errorf("persisting pool config: %w", err)
	}

	for i := 1; i <= opts.Size; i++ {
		clusterName := fmt.Sprintf("%s-%d", opts.Name, i)
		clusterOpts := opts.ProvisionOpts
		clusterOpts.Name = clusterName

		if err := m.provisioning.Create(ctx, clusterOpts); err != nil {
			return fmt.Errorf("provisioning pool cluster %s: %w", clusterName, err)
		}

		if err := m.labelPoolCluster(ctx, clusterName, opts.Name); err != nil {
			return fmt.Errorf("labeling pool cluster %s: %w", clusterName, err)
		}
	}

	m.logger.Info("pool.CreateManualPool: all clusters created, waiting for install and hibernate separately",
		"pool", opts.Name, "count", opts.Size)
	return nil
}

func (m *Manager) WaitManualPoolReady(ctx context.Context, poolName string, timeout time.Duration) error {
	m.logger.Info("pool.WaitManualPoolReady", "pool", poolName, "timeout", timeout)
	if m.provisioning == nil || m.lifecycle == nil {
		return fmt.Errorf("provisioning and lifecycle managers required (use NewWithManagers)")
	}
	if timeout == 0 {
		timeout = 50 * time.Minute
	}

	clusters, err := m.listPoolClusters(ctx, poolName)
	if err != nil {
		return err
	}
	if len(clusters) == 0 {
		return fmt.Errorf("no clusters found for pool %s", poolName)
	}

	for _, name := range clusters {
		m.logger.Info("pool.WaitManualPoolReady: waiting for install", "cluster", name)
		if err := m.provisioning.WaitForProvision(ctx, name, timeout); err != nil {
			return fmt.Errorf("waiting for cluster %s: %w", name, err)
		}

		m.logger.Info("pool.WaitManualPoolReady: hibernating", "cluster", name)
		if err := m.lifecycle.Hibernate(ctx, name, name); err != nil {
			return fmt.Errorf("hibernating cluster %s: %w", name, err)
		}
	}

	return nil
}

func (m *Manager) ClaimManualPool(ctx context.Context, poolName, claimName string) (*ClaimInfo, error) {
	m.logger.Info("pool.ClaimManualPool", "pool", poolName, "claim", claimName)
	if m.lifecycle == nil {
		return nil, fmt.Errorf("lifecycle manager required for manual pool (use NewWithManagers)")
	}

	clusters, err := m.listPoolClusters(ctx, poolName)
	if err != nil {
		return nil, err
	}

	for _, name := range clusters {
		cd, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
		if err != nil {
			continue
		}

		labels, _, _ := unstructured.NestedStringMap(cd.Object, "metadata", "labels")
		if labels[labelPoolClaimed] == "true" {
			continue
		}

		powerState, _, _ := unstructured.NestedString(cd.Object, "spec", "powerState")
		if powerState != string(lifecycle.PowerStateHibernating) {
			continue
		}

		// Atomic claim: conditional patch using resourceVersion
		rv := cd.GetResourceVersion()
		if !m.tryAtomicClaim(ctx, name, rv) {
			continue
		}

		if err := m.lifecycle.Resume(ctx, name, name); err != nil {
			m.logger.Error("pool.ClaimManualPool: resume failed after claim, leaving claimed to avoid race", "cluster", name, "error", err)
		}

		if m.provisioning != nil {
			go m.replenishPool(ctx, poolName)
		}

		return &ClaimInfo{
			Name:    claimName,
			Pool:    poolName,
			Cluster: name,
			Status:  "Resuming",
		}, nil
	}

	return nil, fmt.Errorf("no available (hibernated, unclaimed) cluster in pool %s", poolName)
}

// tryAtomicClaim attempts to set the claim label with a resourceVersion precondition.
// Returns true if this caller won the claim, false if another caller got it first (conflict).
func (m *Manager) tryAtomicClaim(ctx context.Context, clusterName, resourceVersion string) bool {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"resourceVersion": resourceVersion,
			"labels": map[string]interface{}{
				labelPoolClaimed: "true",
			},
		},
	}
	data, _ := json.Marshal(patch)

	_, err := m.client.Patch(ctx, client.GVRClusterDeployment, clusterName, clusterName,
		types.MergePatchType, data)
	if err != nil {
		if apierrors.IsConflict(err) {
			m.logger.Info("pool.tryAtomicClaim: conflict, another caller claimed first", "cluster", clusterName)
		} else {
			m.logger.Error("pool.tryAtomicClaim: patch failed", "cluster", clusterName, "error", err)
		}
		return false
	}
	return true
}

func (m *Manager) ReleaseManualClaim(ctx context.Context, clusterName string) error {
	m.logger.Info("pool.ReleaseManualClaim", "cluster", clusterName)
	if m.lifecycle == nil {
		return fmt.Errorf("lifecycle manager required for manual pool (use NewWithManagers)")
	}

	if err := m.lifecycle.Hibernate(ctx, clusterName, clusterName); err != nil {
		return fmt.Errorf("hibernating cluster %s: %w", clusterName, err)
	}

	return m.setClaimLabel(ctx, clusterName, "false")
}

func (m *Manager) ListManualPool(ctx context.Context, poolName string) (*PoolInfo, error) {
	m.logger.Info("pool.ListManualPool", "pool", poolName)

	list, err := m.client.List(ctx, client.GVRClusterDeployment, "", labelPool+"="+poolName)
	if err != nil {
		return nil, fmt.Errorf("listing pool clusters: %w", err)
	}

	info := &PoolInfo{
		Name: poolName,
		Size: len(list.Items),
	}

	for _, item := range list.Items {
		labels, _, _ := unstructured.NestedStringMap(item.Object, "metadata", "labels")
		installed, _, _ := unstructured.NestedBool(item.Object, "spec", "installed")
		powerState, _, _ := unstructured.NestedString(item.Object, "spec", "powerState")

		switch {
		case labels[labelPoolClaimed] == "true":
			info.Claimed++
		case installed && powerState == string(lifecycle.PowerStateHibernating):
			info.Standby++
			info.Ready++
		case installed:
			info.Ready++
		}
	}

	return info, nil
}

func (m *Manager) DeleteManualPool(ctx context.Context, poolName string) error {
	m.logger.Info("pool.DeleteManualPool", "pool", poolName)
	if m.provisioning == nil {
		return fmt.Errorf("provisioning manager required for manual pool (use NewWithManagers)")
	}

	clusters, err := m.listPoolClusters(ctx, poolName)
	if err != nil {
		return err
	}

	var lastErr error
	for _, name := range clusters {
		if err := m.provisioning.Destroy(ctx, name); err != nil {
			m.logger.Error("pool.DeleteManualPool: failed to destroy cluster", "cluster", name, "error", err)
			lastErr = err
		}
	}

	if err := m.deletePoolConfig(ctx, poolName); err != nil {
		m.logger.Error("pool.DeleteManualPool: failed to delete pool config", "pool", poolName, "error", err)
		if lastErr == nil {
			lastErr = err
		}
	}

	return lastErr
}

func (m *Manager) replenishPool(parentCtx context.Context, poolName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Minute)
	defer cancel()

	_ = parentCtx // replenish runs independently of the claim context

	opts, err := m.loadPoolConfig(ctx, poolName)
	if err != nil {
		m.logger.Error("pool.replenish: failed to load pool config", "pool", poolName, "error", err)
		return
	}

	clusterName := fmt.Sprintf("%s-%s", poolName, randomSuffix())
	m.logger.Info("pool.replenish: provisioning replacement cluster", "pool", poolName, "cluster", clusterName)

	clusterOpts := opts
	clusterOpts.Name = clusterName

	if err := m.provisioning.Create(ctx, clusterOpts); err != nil {
		m.logger.Error("pool.replenish: failed to provision", "cluster", clusterName, "error", err)
		return
	}

	if err := m.labelPoolCluster(ctx, clusterName, poolName); err != nil {
		m.logger.Error("pool.replenish: failed to label", "cluster", clusterName, "error", err)
		return
	}

	m.logger.Info("pool.replenish: waiting for install", "cluster", clusterName)
	if err := m.provisioning.WaitForProvision(ctx, clusterName, 60*time.Minute); err != nil {
		m.logger.Error("pool.replenish: install failed or timed out", "cluster", clusterName, "error", err)
		return
	}

	m.logger.Info("pool.replenish: hibernating replacement", "cluster", clusterName)
	if err := m.lifecycle.Hibernate(ctx, clusterName, clusterName); err != nil {
		m.logger.Error("pool.replenish: failed to hibernate", "cluster", clusterName, "error", err)
		return
	}

	m.logger.Info("pool.replenish: replacement cluster ready", "cluster", clusterName, "pool", poolName)
}

func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// --- Pool config persistence via ConfigMap ---

func poolConfigName(poolName string) string {
	return "pool-config-" + poolName
}

func (m *Manager) savePoolConfig(ctx context.Context, opts ManualPoolOpts) error {
	credRef := opts.CredentialRef
	if credRef == "" {
		if opts.ProvisionOpts.Platform == "ibmcloud" {
			credRef = "ibm-caas-creds"
		} else if opts.ProvisionOpts.Platform == "aws" {
			credRef = "aws-creds"
		}
	}

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      poolConfigName(opts.Name),
				"namespace": poolConfigNS,
				"labels": map[string]interface{}{
					labelPool: opts.Name,
				},
			},
			"data": map[string]interface{}{
				"platform":       opts.ProvisionOpts.Platform,
				"region":         opts.ProvisionOpts.Region,
				"imageSet":       opts.ProvisionOpts.ImageSet,
				"baseDomain":     opts.ProvisionOpts.BaseDomain,
				"workerType":     opts.ProvisionOpts.WorkerType,
				"masterType":     opts.ProvisionOpts.MasterType,
				"pullSecretRef":  "pull-secret",
				"credentialRef":  credRef,
			},
		},
	}

	return m.client.CreateIfNotExists(ctx, client.GVRConfigMap, poolConfigNS, cm)
}

func (m *Manager) loadPoolConfig(ctx context.Context, poolName string) (provisioning.ClusterOpts, error) {
	obj, err := m.client.Get(ctx, client.GVRConfigMap, poolConfigNS, poolConfigName(poolName))
	if err != nil {
		return provisioning.ClusterOpts{}, fmt.Errorf("loading pool config for %s: %w", poolName, err)
	}

	data, _, _ := unstructured.NestedStringMap(obj.Object, "data")

	opts := provisioning.ClusterOpts{
		Platform:   data["platform"],
		Region:     data["region"],
		ImageSet:   data["imageSet"],
		BaseDomain: data["baseDomain"],
		WorkerType: data["workerType"],
		MasterType: data["masterType"],
	}

	credRef := data["credentialRef"]
	if credRef != "" {
		credSecret, err := m.client.Get(ctx, client.GVRSecret, poolConfigNS, credRef)
		if err != nil {
			return opts, fmt.Errorf("loading credential secret %s: %w", credRef, err)
		}
		secretData, _, _ := unstructured.NestedMap(credSecret.Object, "data")
		if v, ok := secretData["ibmcloud_api_key"].(string); ok {
			opts.IBMCloudAPIKey = v
		}
		if v, ok := secretData["aws_access_key_id"].(string); ok {
			opts.AWSAccessKeyID = v
		}
		if v, ok := secretData["aws_secret_access_key"].(string); ok {
			opts.AWSSecretAccessKey = v
		}
	}

	pullRef := data["pullSecretRef"]
	if pullRef != "" {
		pullSecret, err := m.client.Get(ctx, client.GVRSecret, poolConfigNS, pullRef)
		if err == nil {
			pullData, _, _ := unstructured.NestedMap(pullSecret.Object, "data")
			if v, ok := pullData[".dockerconfigjson"].(string); ok {
				opts.PullSecret = v
			}
		}
	}

	return opts, nil
}

func (m *Manager) deletePoolConfig(ctx context.Context, poolName string) error {
	return m.client.DeleteIfExists(ctx, client.GVRConfigMap, poolConfigNS, poolConfigName(poolName))
}

// --- Helper methods ---

func (m *Manager) listPoolClusters(ctx context.Context, poolName string) ([]string, error) {
	list, err := m.client.List(ctx, client.GVRClusterDeployment, "", labelPool+"="+poolName)
	if err != nil {
		return nil, fmt.Errorf("listing pool clusters: %w", err)
	}

	names := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		name, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (m *Manager) isClusterClaimed(ctx context.Context, name string) (bool, error) {
	cd, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err != nil {
		return false, err
	}
	labels, _, _ := unstructured.NestedStringMap(cd.Object, "metadata", "labels")
	return labels[labelPoolClaimed] == "true", nil
}

func (m *Manager) labelPoolCluster(ctx context.Context, clusterName, poolName string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				labelPool:        poolName,
				labelPoolClaimed: "false",
			},
		},
	}
	data, _ := json.Marshal(patch)

	_, err := m.client.Patch(ctx, client.GVRClusterDeployment, clusterName, clusterName,
		types.MergePatchType, data)
	if err != nil {
		return fmt.Errorf("patching ClusterDeployment labels: %w", err)
	}

	mcPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				labelPool: poolName,
			},
		},
	}
	mcData, _ := json.Marshal(mcPatch)

	_, err = m.client.Patch(ctx, client.GVRManagedCluster, "", clusterName,
		types.MergePatchType, mcData)
	return err
}

func (m *Manager) setClaimLabel(ctx context.Context, clusterName, value string) error {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				labelPoolClaimed: value,
			},
		},
	}
	data, _ := json.Marshal(patch)

	_, err := m.client.Patch(ctx, client.GVRClusterDeployment, clusterName, clusterName,
		types.MergePatchType, data)
	return err
}
