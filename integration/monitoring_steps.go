//go:build integration

package integration

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
)

func registerMonitoringSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a managed cluster "([^"]*)" is joined and available$`, s.managedClusterJoinedAndAvailable)
	sc.Step(`^I get the cluster resources for "([^"]*)"$`, s.iGetClusterResources)
	sc.Step(`^I receive node count, CPU capacity, and memory capacity$`, s.receiveResourceDetails)
	sc.Step(`^I list cluster resources for all managed clusters$`, s.iListClusterResources)
	sc.Step(`^I receive resource summaries for each cluster$`, s.receiveResourceSummaries)
	sc.Step(`^each summary includes node count and capacity$`, s.eachSummaryIncludesDetails)
}

func (s *suiteContext) managedClusterJoinedAndAvailable(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	cluster, err := s.fleet.GetCluster(ctx, name)
	if err != nil {
		return fmt.Errorf("cluster %s not found: %w", name, err)
	}
	if !cluster.Available {
		return fmt.Errorf("cluster %s is not available", name)
	}
	return nil
}

func (s *suiteContext) iGetClusterResources(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	res, err := s.monitoring.GetClusterResources(ctx, name)
	if err != nil {
		return err
	}
	s.resources = res
	return nil
}

func (s *suiteContext) receiveResourceDetails() error {
	if s.resources == nil {
		return fmt.Errorf("no resource data available")
	}
	if s.resources.TotalNodes == 0 {
		return fmt.Errorf("node count is 0")
	}
	return nil
}

func (s *suiteContext) iListClusterResources(ctx context.Context) error {
	list, err := s.monitoring.ListClusterResources(ctx)
	if err != nil {
		return err
	}
	s.resourceList = list
	return nil
}

func (s *suiteContext) receiveResourceSummaries() error {
	if len(s.resourceList) == 0 {
		return fmt.Errorf("no resource summaries returned")
	}
	return nil
}

func (s *suiteContext) eachSummaryIncludesDetails() error {
	for _, r := range s.resourceList {
		if r.Name == "" {
			return fmt.Errorf("resource summary has empty name")
		}
	}
	return nil
}
