//go:build integration

package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/lifecycle"
	"github.com/pablofelix/acm-caas-poc/internal/pool"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

func registerManualPoolSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^I create a manual pool "([^"]*)" on platform "([^"]*)" in region "([^"]*)" with size (\d+)$`, s.iCreateManualPool)
	sc.Step(`^the manual pool "([^"]*)" has (\d+) clusters provisioning$`, s.manualPoolHasClustersProvisioning)
	sc.Step(`^the manual pool "([^"]*)" exists$`, s.manualPoolExists)
	sc.Step(`^I wait for manual pool "([^"]*)" to be ready$`, s.iWaitForManualPoolReady)
	sc.Step(`^the manual pool "([^"]*)" has (\d+) standby clusters$`, s.manualPoolHasStandbyClusters)
	sc.Step(`^the manual pool "([^"]*)" has at least (\d+) standby cluster$`, s.manualPoolHasAtLeastStandby)
	sc.Step(`^I claim a cluster from manual pool "([^"]*)" as "([^"]*)"$`, s.iClaimFromManualPool)
	sc.Step(`^the claimed cluster is running$`, s.claimedClusterIsRunning)
	sc.Step(`^the manual pool "([^"]*)" has (\d+) claimed and (\d+) standby$`, s.manualPoolHasClaimedAndStandby)
	sc.Step(`^I list manual pool "([^"]*)"$`, s.iListManualPool)
	sc.Step(`^I see pool "([^"]*)" with size (\d+), showing claimed and standby counts$`, s.iSeeManualPoolCounts)
	sc.Step(`^I have a claimed cluster "([^"]*)" in pool "([^"]*)"$`, s.iHaveClaimedCluster)
	sc.Step(`^I release manual claim "([^"]*)"$`, s.iReleaseManualClaim)
	sc.Step(`^the cluster is hibernated$`, s.clusterIsHibernated)
}

func (s *suiteContext) iCreateManualPool(ctx context.Context, name, platform, region string, size int) error {
	provOpts := provisioning.ClusterOpts{
		Platform:   platform,
		Region:     region,
		ImageSet:   s.cfg.ClusterImageSet,
		BaseDomain: s.cfg.BaseDomain,
	}

	if platform == "ibmcloud" {
		apiKey, err := s.fetchACMCredentialKey("ibm-caas-creds", "ibmcloud_api_key")
		if err != nil {
			return fmt.Errorf("reading IBM Cloud API key: %w", err)
		}
		provOpts.IBMCloudAPIKey = apiKey

		if provOpts.BaseDomain == "" {
			obj, err := s.client.Get(ctx, client.GVRSecret, "open-cluster-management", "ibm-caas-creds")
			if err == nil {
				data, _, _ := unstructured.NestedMap(obj.Object, "data")
				if encoded, ok := data["baseDomain"].(string); ok {
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err == nil {
						provOpts.BaseDomain = string(decoded)
					} else {
						provOpts.BaseDomain = encoded
					}
				}
			}
		}
	}

	pullSecret, err := s.fetchPullSecret(ctx)
	if err != nil {
		return fmt.Errorf("fetching pull secret: %w", err)
	}
	provOpts.PullSecret = pullSecret

	opts := pool.ManualPoolOpts{
		Name:          name,
		Size:          size,
		ProvisionOpts: provOpts,
	}

	fmt.Printf("\n  ┌─ Creating manual pool %q\n", name)
	fmt.Printf("  │  platform=%s  region=%s  size=%d\n", platform, region, size)
	fmt.Printf("  │  imageSet=%s  baseDomain=%s\n", provOpts.ImageSet, provOpts.BaseDomain)
	fmt.Printf("  └─\n")

	s.lastPoolName = name
	return s.poolManager.CreateManualPool(ctx, opts)
}

func (s *suiteContext) manualPoolHasClustersProvisioning(ctx context.Context, name string, count int) error {
	info, err := s.poolManager.ListManualPool(ctx, name)
	if err != nil {
		return err
	}
	fmt.Printf("\n  ┌─ Manual pool %q: size=%d (expecting %d provisioning)\n", name, info.Size, count)
	fmt.Printf("  └─\n")
	if info.Size != count {
		return fmt.Errorf("expected %d clusters, got %d", count, info.Size)
	}
	return nil
}

func (s *suiteContext) manualPoolExists(ctx context.Context, name string) error {
	info, err := s.poolManager.ListManualPool(ctx, name)
	if err != nil {
		return err
	}
	if info.Size == 0 {
		return fmt.Errorf("manual pool %q has no clusters", name)
	}
	return nil
}

func (s *suiteContext) iWaitForManualPoolReady(ctx context.Context, name string) error {
	fmt.Printf("\n  ┌─ Waiting for manual pool %q to be ready (clusters install + hibernate)\n", name)
	fmt.Printf("  │  This may take 40-50 minutes...\n")
	fmt.Printf("  └─\n")
	return s.poolManager.WaitManualPoolReady(ctx, name, 60*time.Minute)
}

func (s *suiteContext) manualPoolHasStandbyClusters(ctx context.Context, name string, count int) error {
	info, err := s.poolManager.ListManualPool(ctx, name)
	if err != nil {
		return err
	}
	fmt.Printf("\n  ┌─ Manual pool %q: standby=%d ready=%d claimed=%d\n", name, info.Standby, info.Ready, info.Claimed)
	fmt.Printf("  └─\n")
	if info.Standby < count {
		return fmt.Errorf("expected %d standby, got %d", count, info.Standby)
	}
	return nil
}

func (s *suiteContext) manualPoolHasAtLeastStandby(ctx context.Context, name string, count int) error {
	return s.manualPoolHasStandbyClusters(ctx, name, count)
}

func (s *suiteContext) iClaimFromManualPool(ctx context.Context, poolName, claimName string) error {
	fmt.Printf("\n  ┌─ Claiming cluster from manual pool %q as %q\n", poolName, claimName)
	fmt.Printf("  └─\n")

	info, err := s.poolManager.ClaimManualPool(ctx, poolName, claimName)
	if err != nil {
		return err
	}
	s.lastClaimInfo = &pool.ClaimInfo{
		Name:    info.Name,
		Pool:    info.Pool,
		Cluster: info.Cluster,
		Status:  info.Status,
	}
	s.lastPoolName = poolName

	fmt.Printf("\n  ┌─ Claimed cluster %q from pool %q\n", info.Cluster, poolName)
	fmt.Printf("  │  status=%s\n", info.Status)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) claimedClusterIsRunning(ctx context.Context) error {
	if s.lastClaimInfo == nil {
		return fmt.Errorf("no claim in context")
	}

	timeout := 15 * time.Minute
	interval := 15 * time.Second
	deadline := time.After(timeout)

	fmt.Printf("\n  ┌─ Waiting for claimed cluster %q to be running (timeout %v)\n", s.lastClaimInfo.Cluster, timeout)
	fmt.Printf("  └─\n")

	for {
		state, err := s.lifecycle.GetPowerState(ctx, s.lastClaimInfo.Cluster, s.lastClaimInfo.Cluster)
		if err == nil && state == lifecycle.PowerStateRunning {
			fmt.Printf("\n  ┌─ Cluster %q is running\n", s.lastClaimInfo.Cluster)
			fmt.Printf("  └─\n")
			return nil
		}
		if err == nil {
			fmt.Printf("  │  cluster %s: powerState=%s (waiting for Running)\n", s.lastClaimInfo.Cluster, state)
		}

		select {
		case <-deadline:
			return fmt.Errorf("cluster %q did not reach Running within %v", s.lastClaimInfo.Cluster, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) manualPoolHasClaimedAndStandby(ctx context.Context, name string, claimed, standby int) error {
	info, err := s.poolManager.ListManualPool(ctx, name)
	if err != nil {
		return err
	}
	fmt.Printf("\n  ┌─ Manual pool %q: claimed=%d standby=%d\n", name, info.Claimed, info.Standby)
	fmt.Printf("  └─\n")
	if info.Claimed != claimed {
		return fmt.Errorf("expected %d claimed, got %d", claimed, info.Claimed)
	}
	if info.Standby != standby {
		return fmt.Errorf("expected %d standby, got %d", standby, info.Standby)
	}
	return nil
}

func (s *suiteContext) iListManualPool(ctx context.Context, name string) error {
	info, err := s.poolManager.ListManualPool(ctx, name)
	if err != nil {
		return err
	}
	s.poolList = []pool.PoolInfo{*info}

	fmt.Printf("\n  ┌─ Manual pool %q\n", name)
	fmt.Printf("  │  size=%d  ready=%d  standby=%d  claimed=%d\n", info.Size, info.Ready, info.Standby, info.Claimed)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) iSeeManualPoolCounts(ctx context.Context, name string, size int) error {
	if len(s.poolList) == 0 {
		return fmt.Errorf("no pool data in context")
	}
	info := s.poolList[0]
	if info.Name != name {
		return fmt.Errorf("expected pool %q, got %q", name, info.Name)
	}
	if info.Size != size {
		return fmt.Errorf("expected size %d, got %d", size, info.Size)
	}
	return nil
}

func (s *suiteContext) iHaveClaimedCluster(ctx context.Context, claimName, poolName string) error {
	info, err := s.poolManager.ListManualPool(ctx, poolName)
	if err != nil {
		return err
	}
	if info.Claimed == 0 {
		return fmt.Errorf("no claimed clusters in pool %q", poolName)
	}
	s.lastPoolName = poolName
	return nil
}

func (s *suiteContext) iReleaseManualClaim(ctx context.Context, claimName string) error {
	if s.lastClaimInfo == nil {
		return fmt.Errorf("no claim in context")
	}
	fmt.Printf("\n  ┌─ Releasing manual claim %q (cluster %s)\n", claimName, s.lastClaimInfo.Cluster)
	fmt.Printf("  └─\n")
	return s.poolManager.ReleaseManualClaim(ctx, s.lastClaimInfo.Cluster)
}

func (s *suiteContext) clusterIsHibernated(ctx context.Context) error {
	if s.lastClaimInfo == nil {
		return fmt.Errorf("no claim in context")
	}

	timeout := 10 * time.Minute
	interval := 15 * time.Second
	deadline := time.After(timeout)

	for {
		state, err := s.lifecycle.GetPowerState(ctx, s.lastClaimInfo.Cluster, s.lastClaimInfo.Cluster)
		if err == nil && state == lifecycle.PowerStateHibernating {
			fmt.Printf("\n  ┌─ Cluster %q is hibernated\n", s.lastClaimInfo.Cluster)
			fmt.Printf("  └─\n")
			return nil
		}

		select {
		case <-deadline:
			return fmt.Errorf("cluster %q did not hibernate within %v", s.lastClaimInfo.Cluster, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
