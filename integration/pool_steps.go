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
	"github.com/pablofelix/acm-caas-poc/internal/pool"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

func registerPoolSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^I create a ClusterPool "([^"]*)" on platform "([^"]*)" with size (\d+)$`, s.iCreateClusterPool)
	sc.Step(`^the ClusterPool "([^"]*)" exists with size (\d+)$`, s.clusterPoolExistsWithSize)
	sc.Step(`^the pool "([^"]*)" eventually has (\d+) ready clusters$`, s.poolHasReadyClusters)
	sc.Step(`^the pool "([^"]*)" has at least (\d+) ready cluster$`, s.poolHasAtLeastReadyClusters)
	sc.Step(`^I claim a cluster from pool "([^"]*)" as "([^"]*)" with TTL "([^"]*)"$`, s.iClaimFromPool)
	sc.Step(`^the ClusterClaim "([^"]*)" is bound to a cluster$`, s.claimIsBound)
	sc.Step(`^the pool "([^"]*)" provisions a replacement cluster$`, s.poolProvisionsReplacement)
	sc.Step(`^I list cluster pools$`, s.iListClusterPools)
	sc.Step(`^I see pool "([^"]*)" with size, ready, and claimed counts$`, s.iSeePoolWithCounts)
	sc.Step(`^I list claims in pool "([^"]*)"$`, s.iListClaimsInPool)
	sc.Step(`^I see claim "([^"]*)" with its assigned cluster$`, s.iSeeClaimWithCluster)
	sc.Step(`^I have a claim "([^"]*)" in pool "([^"]*)"$`, s.iHaveClaimInPool)
	sc.Step(`^I release claim "([^"]*)" from pool "([^"]*)"$`, s.iReleaseClaimFromPool)
	sc.Step(`^the ClusterClaim "([^"]*)" is removed$`, s.clusterClaimIsRemoved)
	sc.Step(`^I delete pool "([^"]*)"$`, s.iDeletePool)
	sc.Step(`^the ClusterPool "([^"]*)" is removed$`, s.clusterPoolIsRemoved)
	sc.Step(`^all pool clusters are destroyed$`, s.allPoolClustersDestroyed)
}

func (s *suiteContext) iCreateClusterPool(ctx context.Context, name, platform string, size int) error {
	pullSecret, err := s.fetchPullSecret(ctx)
	if err != nil {
		return fmt.Errorf("fetching pull secret: %w", err)
	}

	opts := pool.PoolOpts{
		Name:       name,
		Namespace:  name,
		Size:       size,
		Platform:   platform,
		PullSecret: pullSecret,
		ImageSet:   s.cfg.ClusterImageSet,
		BaseDomain: s.cfg.BaseDomain,
	}

	switch platform {
	case "aws":
		opts.Region = s.cfg.AWSRegion
		if opts.Region == "" {
			opts.Region = "us-east-1"
		}
		awsCreds, err := provisioning.LoadAWSCredentials("")
		if err != nil {
			return fmt.Errorf("loading AWS credentials: %w", err)
		}
		opts.AWSAccessKeyID = awsCreds.AccessKeyID
		opts.AWSSecretAccessKey = awsCreds.SecretAccessKey
	case "ibmcloud":
		opts.Region = s.cfg.IBMCloudRegion
		if opts.Region == "" {
			opts.Region = "us-south"
		}
		apiKey, err := s.fetchACMCredentialKey("ibm-caas-creds", "ibmcloud_api_key")
		if err != nil {
			return fmt.Errorf("reading IBM Cloud API key from ACM credential: %w", err)
		}
		opts.IBMCloudAPIKey = apiKey
	}

	if opts.BaseDomain == "" {
		switch platform {
		case "aws":
			opts.BaseDomain = s.cfg.AWSBaseDomain
		case "ibmcloud":
			obj, err := s.client.Get(ctx, client.GVRSecret, "open-cluster-management", "ibm-caas-creds")
			if err == nil {
				data, _, _ := unstructured.NestedMap(obj.Object, "data")
				if encoded, ok := data["baseDomain"].(string); ok {
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err == nil {
						opts.BaseDomain = string(decoded)
					} else {
						opts.BaseDomain = encoded
					}
				}
			}
		}
	}

	fmt.Printf("\n  ┌─ Creating ClusterPool %q\n", name)
	fmt.Printf("  │  platform=%s  region=%s  size=%d\n", platform, opts.Region, size)
	fmt.Printf("  │  imageSet=%s  baseDomain=%s\n", opts.ImageSet, opts.BaseDomain)
	fmt.Printf("  └─\n")

	s.lastPoolName = name
	return s.poolManager.CreatePool(ctx, opts)
}

func (s *suiteContext) clusterPoolExistsWithSize(ctx context.Context, name string, size int) error {
	info, err := s.poolManager.GetPool(ctx, name, name)
	if err != nil {
		return err
	}
	fmt.Printf("\n  ┌─ ClusterPool %q verified\n", name)
	fmt.Printf("  │  size=%d  ready=%d  claimed=%d\n", info.Size, info.Ready, info.Claimed)
	fmt.Printf("  └─\n")
	if info.Size != size {
		return fmt.Errorf("expected size %d, got %d", size, info.Size)
	}
	return nil
}

func (s *suiteContext) poolHasReadyClusters(ctx context.Context, name string, count int) error {
	timeout := 50 * time.Minute
	interval := 30 * time.Second
	deadline := time.After(timeout)

	fmt.Printf("\n  ┌─ Waiting for pool %q to have %d ready clusters (timeout %v)\n", name, count, timeout)
	fmt.Printf("  └─\n")

	for {
		info, err := s.poolManager.GetPool(ctx, name, name)
		if err == nil && info.Ready >= count {
			fmt.Printf("\n  ┌─ Pool %q has %d ready clusters\n", name, info.Ready)
			fmt.Printf("  └─\n")
			return nil
		}
		if err == nil {
			fmt.Printf("  │  pool %s: size=%d ready=%d standby=%d (waiting for %d ready)\n",
				name, info.Size, info.Ready, info.Standby, count)
		}

		select {
		case <-deadline:
			return fmt.Errorf("pool %q did not reach %d ready clusters within %v", name, count, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) poolHasAtLeastReadyClusters(ctx context.Context, name string, count int) error {
	info, err := s.poolManager.GetPool(ctx, name, name)
	if err != nil {
		return err
	}
	if info.Ready < count {
		return fmt.Errorf("pool %q has %d ready clusters, need at least %d", name, info.Ready, count)
	}
	fmt.Printf("\n  ┌─ Pool %q has %d ready clusters (need %d)\n", name, info.Ready, count)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) iClaimFromPool(ctx context.Context, poolName, claimName, ttl string) error {
	fmt.Printf("\n  ┌─ Claiming cluster from pool %q as %q (TTL=%s)\n", poolName, claimName, ttl)
	fmt.Printf("  └─\n")

	info, err := s.poolManager.Claim(ctx, poolName, poolName, claimName, ttl)
	if err != nil {
		return err
	}
	s.lastClaimInfo = info
	fmt.Printf("\n  ┌─ ClusterClaim %q created\n", claimName)
	fmt.Printf("  │  pool=%s  status=%s\n", info.Pool, info.Status)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) claimIsBound(ctx context.Context, claimName string) error {
	timeout := 15 * time.Minute
	interval := 15 * time.Second
	deadline := time.After(timeout)

	fmt.Printf("\n  ┌─ Waiting for claim %q to bind (timeout %v)\n", claimName, timeout)
	fmt.Printf("  └─\n")

	for {
		claims, err := s.poolManager.ListClaims(ctx, s.lastClaimInfo.Namespace)
		if err == nil {
			for _, c := range claims {
				if c.Name == claimName && c.Cluster != "" {
					fmt.Printf("\n  ┌─ Claim %q bound to cluster %q (status=%s)\n", claimName, c.Cluster, c.Status)
					fmt.Printf("  └─\n")
					s.lastClaimInfo = &c
					return nil
				}
			}
		}

		select {
		case <-deadline:
			return fmt.Errorf("claim %q did not bind within %v", claimName, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) poolProvisionsReplacement(ctx context.Context, poolName string) error {
	fmt.Printf("\n  ┌─ Checking pool %q is provisioning a replacement\n", poolName)

	info, err := s.poolManager.GetPool(ctx, poolName, poolName)
	if err != nil {
		return err
	}
	fmt.Printf("  │  size=%d  ready=%d  claimed=%d\n", info.Size, info.Ready, info.Claimed)
	fmt.Printf("  └─ Pool will auto-provision to maintain size=%d\n", info.Size)
	return nil
}

func (s *suiteContext) iListClusterPools(ctx context.Context) error {
	pools, err := s.poolManager.ListPools(ctx)
	if err != nil {
		return err
	}
	s.poolList = pools
	fmt.Printf("\n  ┌─ Cluster pools: %d\n", len(pools))
	for _, p := range pools {
		fmt.Printf("  │  %-20s size=%d  ready=%d  claimed=%d\n", p.Name, p.Size, p.Ready, p.Claimed)
	}
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) iSeePoolWithCounts(ctx context.Context, name string) error {
	for _, p := range s.poolList {
		if p.Name == name {
			fmt.Printf("\n  ┌─ Pool %q: size=%d ready=%d claimed=%d\n", name, p.Size, p.Ready, p.Claimed)
			fmt.Printf("  └─\n")
			return nil
		}
	}
	return fmt.Errorf("pool %q not found in list", name)
}

func (s *suiteContext) iListClaimsInPool(ctx context.Context, poolName string) error {
	claims, err := s.poolManager.ListClaims(ctx, poolName)
	if err != nil {
		return err
	}
	s.claimList = claims
	fmt.Printf("\n  ┌─ Claims in pool %q: %d\n", poolName, len(claims))
	for _, c := range claims {
		fmt.Printf("  │  %-20s cluster=%s  status=%s\n", c.Name, c.Cluster, c.Status)
	}
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) iSeeClaimWithCluster(ctx context.Context, claimName string) error {
	for _, c := range s.claimList {
		if c.Name == claimName {
			if c.Cluster == "" {
				return fmt.Errorf("claim %q has no assigned cluster", claimName)
			}
			fmt.Printf("\n  ┌─ Claim %q → cluster %q\n", claimName, c.Cluster)
			fmt.Printf("  └─\n")
			return nil
		}
	}
	return fmt.Errorf("claim %q not found in list", claimName)
}

func (s *suiteContext) iHaveClaimInPool(ctx context.Context, claimName, poolName string) error {
	claims, err := s.poolManager.ListClaims(ctx, poolName)
	if err != nil {
		return err
	}
	for _, c := range claims {
		if c.Name == claimName {
			s.lastClaimInfo = &c
			return nil
		}
	}
	return fmt.Errorf("claim %q not found in pool %q", claimName, poolName)
}

func (s *suiteContext) iReleaseClaimFromPool(ctx context.Context, claimName, poolName string) error {
	fmt.Printf("\n  ┌─ Releasing claim %q from pool %q\n", claimName, poolName)
	fmt.Printf("  └─\n")
	return s.poolManager.ReleaseClaim(ctx, claimName, poolName)
}

func (s *suiteContext) clusterClaimIsRemoved(ctx context.Context, claimName string) error {
	ns := ""
	if s.lastClaimInfo != nil {
		ns = s.lastClaimInfo.Namespace
	}
	if ns == "" {
		return fmt.Errorf("no namespace context for claim %q", claimName)
	}
	_, err := s.client.Get(ctx, client.GVRClusterClaim, ns, claimName)
	if err != nil {
		fmt.Printf("\n  ┌─ ClusterClaim %q confirmed removed\n", claimName)
		fmt.Printf("  └─\n")
		return nil
	}
	return fmt.Errorf("ClusterClaim %q still exists", claimName)
}

func (s *suiteContext) iDeletePool(ctx context.Context, name string) error {
	fmt.Printf("\n  ┌─ Deleting ClusterPool %q\n", name)
	fmt.Printf("  └─\n")
	return s.poolManager.DeletePool(ctx, name, name)
}

func (s *suiteContext) clusterPoolIsRemoved(ctx context.Context, name string) error {
	_, err := s.client.Get(ctx, client.GVRClusterPool, name, name)
	if err != nil {
		fmt.Printf("\n  ┌─ ClusterPool %q confirmed removed\n", name)
		fmt.Printf("  └─\n")
		return nil
	}
	return fmt.Errorf("ClusterPool %q still exists", name)
}

func (s *suiteContext) allPoolClustersDestroyed(ctx context.Context) error {
	fmt.Printf("\n  ┌─ Waiting for pool cluster deprovision (this may take several minutes)\n")
	fmt.Printf("  └─\n")

	timeout := 30 * time.Minute
	interval := 30 * time.Second
	deadline := time.After(timeout)

	selector := fmt.Sprintf("hive.openshift.io/cluster-pool-name=%s", s.lastPoolName)
	for {
		list, err := s.client.List(ctx, client.GVRClusterDeployment, "", selector)
		if err != nil || len(list.Items) == 0 {
			fmt.Printf("\n  ┌─ All pool clusters destroyed\n")
			fmt.Printf("  └─\n")
			return nil
		}
		fmt.Printf("  │  %d pool clusters still deprovisioning...\n", len(list.Items))

		select {
		case <-deadline:
			return fmt.Errorf("pool clusters not destroyed within %v", timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
