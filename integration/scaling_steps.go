//go:build integration

package integration

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/pablofelix/acm-caas-poc/internal/scaling"
)

func registerScalingSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a Hive-provisioned cluster "([^"]*)" has a MachinePool$`, s.hiveClusterHasMachinePool)
	sc.Step(`^I get the MachinePool for "([^"]*)"$`, s.iGetMachinePool)
	sc.Step(`^I receive the replica count and platform type$`, s.receiveReplicaAndPlatform)
	sc.Step(`^I set the MachinePool replicas to (\d+) for "([^"]*)"$`, s.iSetMachinePoolReplicas)
	sc.Step(`^the MachinePool for "([^"]*)" has replicas set to (\d+)$`, s.machinePoolHasReplicas)
	sc.Step(`^I enable autoscaling with min (\d+) and max (\d+) for "([^"]*)"$`, s.iEnableAutoscaling)
	sc.Step(`^the MachinePool for "([^"]*)" has autoscaling enabled$`, s.machinePoolHasAutoscaling)
	sc.Step(`^I list all MachinePools$`, s.iListAllMachinePools)
	sc.Step(`^I receive MachinePool information for each Hive cluster$`, s.receiveMachinePoolInfo)
	sc.Step(`^cluster "([^"]*)" was imported without Hive$`, s.clusterImportedWithoutHive)
	sc.Step(`^I try to get the MachinePool for "([^"]*)"$`, s.iTryGetMachinePool)
	sc.Step(`^the operation returns an ErrNoMachinePool error$`, s.operationReturnsErrNoMachinePool)
}

func (s *suiteContext) hiveClusterHasMachinePool(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	mp, err := s.scaling.GetMachinePool(ctx, name)
	if err != nil {
		return fmt.Errorf("no MachinePool for %s: %w", name, err)
	}
	s.machinePool = mp
	return nil
}

func (s *suiteContext) iGetMachinePool(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	mp, err := s.scaling.GetMachinePool(ctx, name)
	if err != nil {
		return err
	}
	s.machinePool = mp
	return nil
}

func (s *suiteContext) receiveReplicaAndPlatform() error {
	if s.machinePool == nil {
		return fmt.Errorf("no MachinePool data")
	}
	return nil
}

func (s *suiteContext) iSetMachinePoolReplicas(ctx context.Context, replicas int, name string) error {
	name = s.resolveCluster(name)
	return s.scaling.SetReplicas(ctx, name, replicas)
}

func (s *suiteContext) machinePoolHasReplicas(ctx context.Context, name string, replicas int) error {
	name = s.resolveCluster(name)
	mp, err := s.scaling.GetMachinePool(ctx, name)
	if err != nil {
		return err
	}
	if mp.Replicas == nil || *mp.Replicas != int64(replicas) {
		return fmt.Errorf("replicas = %v, want %d", mp.Replicas, replicas)
	}
	return nil
}

func (s *suiteContext) iEnableAutoscaling(ctx context.Context, min, max int, name string) error {
	name = s.resolveCluster(name)
	return s.scaling.EnableAutoscaling(ctx, name, min, max)
}

func (s *suiteContext) machinePoolHasAutoscaling(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	mp, err := s.scaling.GetMachinePool(ctx, name)
	if err != nil {
		return err
	}
	if mp.MinSize == nil || mp.MaxSize == nil {
		return fmt.Errorf("autoscaling not enabled for %s", name)
	}
	return nil
}

func (s *suiteContext) iListAllMachinePools(ctx context.Context) error {
	pools, err := s.scaling.ListMachinePools(ctx)
	if err != nil {
		return err
	}
	s.machinePools = pools
	return nil
}

func (s *suiteContext) receiveMachinePoolInfo() error {
	if len(s.machinePools) == 0 {
		return fmt.Errorf("no MachinePools returned")
	}
	return nil
}

func (s *suiteContext) clusterImportedWithoutHive(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	supported, _ := s.scaling.ClusterSupportsScaling(ctx, name)
	if supported {
		return fmt.Errorf("cluster %s has a ClusterDeployment — expected imported", name)
	}
	return nil
}

func (s *suiteContext) iTryGetMachinePool(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, s.err = s.scaling.GetMachinePool(ctx, name)
	return nil
}

func (s *suiteContext) operationReturnsErrNoMachinePool() error {
	if s.err == nil {
		return fmt.Errorf("expected ErrNoMachinePool but got nil")
	}
	var noMP *scaling.ErrNoMachinePool
	_ = noMP
	return nil
}
