//go:build integration

package integration

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
)

func registerFleetSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^I list ManagedCluster resources$`, s.iListManagedClusterResources)
	sc.Step(`^each cluster has a name and condition list$`, s.eachClusterHasNameAndConditions)
	sc.Step(`^available clusters report Available = True$`, s.availableClustersReportTrue)
	sc.Step(`^a ManagedCluster "([^"]*)" exists$`, s.aManagedClusterExists)
	sc.Step(`^I get ManagedCluster "([^"]*)"$`, s.iGetManagedCluster)
	sc.Step(`^the cluster info includes name, labels, and conditions$`, s.clusterInfoIncludesDetails)
}

func (s *suiteContext) iListManagedClusterResources(ctx context.Context) error {
	clusters, err := s.fleet.ListClusters(ctx, "")
	if err != nil {
		return err
	}
	s.clusters = clusters
	return nil
}

func (s *suiteContext) eachClusterHasNameAndConditions() error {
	fmt.Printf("\n  ┌─ ManagedClusters: %d\n", len(s.clusters))
	for _, c := range s.clusters {
		if c.Name == "" {
			return fmt.Errorf("found cluster with empty name")
		}
		if len(c.Conditions) == 0 {
			return fmt.Errorf("cluster %s has no conditions", c.Name)
		}
		avail := "No"
		if c.Available {
			avail = "Yes"
		}
		fmt.Printf("  │  %-20s available=%-5s joined=%-5v conditions=%d\n", c.Name, avail, c.Joined, len(c.Conditions))
	}
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) availableClustersReportTrue() error {
	for _, c := range s.clusters {
		if c.Available && c.Name != "" {
			return nil
		}
	}
	return fmt.Errorf("no available clusters found")
}

func (s *suiteContext) aManagedClusterExists(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	cluster, err := s.fleet.GetCluster(ctx, name)
	if err != nil {
		return fmt.Errorf("ManagedCluster %s does not exist: %w", name, err)
	}
	s.cluster = cluster
	return nil
}

func (s *suiteContext) iGetManagedCluster(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	cluster, err := s.fleet.GetCluster(ctx, name)
	if err != nil {
		return err
	}
	s.cluster = cluster
	return nil
}

func (s *suiteContext) clusterInfoIncludesDetails() error {
	if s.cluster == nil {
		return fmt.Errorf("no cluster info available")
	}
	if s.cluster.Name == "" {
		return fmt.Errorf("cluster name is empty")
	}
	if len(s.cluster.Conditions) == 0 {
		return fmt.Errorf("cluster has no conditions")
	}
	fmt.Printf("\n  ┌─ Cluster: %s\n", s.cluster.Name)
	fmt.Printf("  │  Available: %v, Joined: %v, Version: %s\n", s.cluster.Available, s.cluster.Joined, s.cluster.Version)
	if len(s.cluster.Labels) > 0 {
		fmt.Printf("  │  Labels: %d entries\n", len(s.cluster.Labels))
	}
	fmt.Printf("  │  Conditions: %d\n", len(s.cluster.Conditions))
	for _, cond := range s.cluster.Conditions {
		fmt.Printf("  │    %-40s = %s\n", cond.Type, cond.Status)
	}
	fmt.Printf("  └─\n")
	return nil
}
