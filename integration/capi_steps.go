//go:build integration

package integration

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

func registerCAPISteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^CAPI infrastructure provider "([^"]*)" is available$`, s.capiInfraProviderAvailable)
	sc.Step(`^I run "acmlab provision create ([^ ]+) --type capi --kubernetes-version ([^ ]+) --workers (\d+)"$`, s.iRunCAPIProvisionCreate)
	sc.Step(`^a CAPI Cluster "([^"]*)" is created in namespace "([^"]*)"$`, s.capiClusterExistsInNamespace)
	sc.Step(`^a MachineDeployment "([^"]*)" is created with (\d+) replicas$`, s.capiMachineDeploymentExistsWithReplicas)
	sc.Step(`^a CAPI Cluster "([^"]*)" exists$`, s.capiClusterExists)
	sc.Step(`^I run "acmlab provision list-capi"$`, s.iRunCAPIProvisionList)
	sc.Step(`^the output shows "([^"]*)" with namespace, phase, readiness, and Kubernetes version$`, s.outputShowsCAPICluster)
	sc.Step(`^I run "acmlab provision destroy ([^"]*)"$`, s.iRunCAPIProvisionDestroy)
	sc.Step(`^the CAPI Cluster "([^"]*)" is removed$`, s.capiClusterRemoved)
	sc.Step(`^the MachineDeployment "([^"]*)" is removed$`, s.capiMachineDeploymentRemoved)
}

func (s *suiteContext) capiInfraProviderAvailable(ctx context.Context, provider string) error {
	_, err := s.client.List(ctx, client.GVRCAPICluster, "", "")
	if err != nil {
		return fmt.Errorf("CAPI Cluster CRD not available (provider %s may not be installed): %w", provider, err)
	}
	return nil
}

func (s *suiteContext) iRunCAPIProvisionCreate(ctx context.Context, name, k8sVersion string, workers int) error {
	opts := provisioning.CAPIClusterOpts{
		Name:              name,
		KubernetesVersion: k8sVersion,
		WorkerReplicas:    int64(workers),
	}
	return s.provisioner.CreateCAPI(ctx, opts)
}

func (s *suiteContext) capiClusterExistsInNamespace(ctx context.Context, name, namespace string) error {
	_, err := s.client.Get(ctx, client.GVRCAPICluster, namespace, name)
	if err != nil {
		return fmt.Errorf("CAPI Cluster %s/%s not found: %w", namespace, name, err)
	}
	return nil
}

func (s *suiteContext) capiMachineDeploymentExistsWithReplicas(ctx context.Context, name string, replicas int) error {
	ns := name[:len(name)-len("-workers")]
	obj, err := s.client.Get(ctx, client.GVRCAPIMachineDeployment, ns, name)
	if err != nil {
		list, listErr := s.client.List(ctx, client.GVRCAPIMachineDeployment, "", "")
		if listErr == nil {
			for _, item := range list.Items {
				if item.GetName() == name {
					obj = &item
					break
				}
			}
		}
		if obj == nil {
			return fmt.Errorf("MachineDeployment %s not found: %w", name, err)
		}
	}
	actual, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if actual != int64(replicas) {
		return fmt.Errorf("MachineDeployment %s has %d replicas, want %d", name, actual, replicas)
	}
	return nil
}

func (s *suiteContext) capiClusterExists(ctx context.Context, name string) error {
	return s.capiClusterExistsInNamespace(ctx, name, name)
}

func (s *suiteContext) iRunCAPIProvisionList(ctx context.Context) error {
	clusters, err := s.provisioner.ListCAPI(ctx)
	if err != nil {
		return err
	}
	s.capiList = clusters
	return nil
}

func (s *suiteContext) outputShowsCAPICluster(ctx context.Context, name string) error {
	for _, c := range s.capiList {
		if c.Name == name {
			if c.Namespace == "" {
				return fmt.Errorf("CAPI Cluster %s has empty namespace", name)
			}
			fmt.Printf("\n  ┌─ CAPI Cluster %s\n", name)
			fmt.Printf("  │  namespace=%s phase=%s ready=%s k8s=%s\n",
				c.Namespace, c.Phase, strconv.FormatBool(c.Ready), c.KubernetesVersion)
			fmt.Printf("  └─\n")
			return nil
		}
	}
	return fmt.Errorf("CAPI Cluster %s not found in list", name)
}

func (s *suiteContext) iRunCAPIProvisionDestroy(ctx context.Context, name string) error {
	return s.provisioner.DestroyCAPI(ctx, name)
}

func (s *suiteContext) capiClusterRemoved(ctx context.Context, name string) error {
	return s.waitForRemoval(ctx, client.GVRCAPICluster, name, name)
}

func (s *suiteContext) capiMachineDeploymentRemoved(ctx context.Context, name string) error {
	ns := name[:len(name)-len("-workers")]
	return s.waitForRemoval(ctx, client.GVRCAPIMachineDeployment, ns, name)
}
