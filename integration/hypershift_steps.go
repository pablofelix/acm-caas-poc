//go:build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

func registerHyperShiftSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a pull secret exists for the target platform$`, s.pullSecretExistsForPlatform)
	sc.Step(`^I create a HyperShift cluster "([^"]*)" with (\d+) workers$`, s.iCreateHyperShiftCluster)
	sc.Step(`^a HostedCluster "([^"]*)" is created in namespace "([^"]*)"$`, s.hostedClusterExistsInNamespace)
	sc.Step(`^a HostedCluster "([^"]*)" exists$`, s.hostedClusterExists)
	sc.Step(`^a NodePool "([^"]*)" is created with (\d+) replicas$`, s.nodePoolExistsWithReplicas)
	sc.Step(`^a ManagedCluster "([^"]*)" is registered on the hub$`, s.managedClusterRegistered)
	sc.Step(`^I list hosted clusters$`, s.iListHostedClusters)
	sc.Step(`^the output shows "([^"]*)" with namespace, availability, and version$`, s.outputShowsHostedCluster)
	sc.Step(`^I destroy hosted cluster "([^"]*)"$`, s.iDestroyHostedCluster)
	sc.Step(`^the HostedCluster "([^"]*)" is removed$`, s.hostedClusterRemoved)
	sc.Step(`^the NodePool "([^"]*)" is removed$`, s.nodePoolRemoved)
	sc.Step(`^the ManagedCluster "([^"]*)" is detached from the hub$`, s.managedClusterDetached)
}

func (s *suiteContext) pullSecretExistsForPlatform(ctx context.Context) error {
	list, err := s.client.List(ctx, client.GVRSecret, "openshift-config", "")
	if err != nil {
		list, err = s.client.List(ctx, client.GVRSecret, "open-cluster-management", "")
		if err != nil {
			return fmt.Errorf("no pull secret namespace accessible: %w", err)
		}
	}
	for _, secret := range list.Items {
		data, _, _ := unstructured.NestedMap(secret.Object, "data")
		if _, ok := data[".dockerconfigjson"]; ok {
			return nil
		}
	}
	return fmt.Errorf("no pull secret with .dockerconfigjson found")
}

func (s *suiteContext) iCreateHyperShiftCluster(ctx context.Context, name string, workers int) error {
	pullSecret, err := s.fetchPullSecret(ctx)
	if err != nil {
		return err
	}

	opts := provisioning.HyperShiftOpts{
		Name:             name,
		Platform:         s.cfg.Platform,
		Region:           s.cfg.IBMCloudRegion,
		BaseDomain:       s.cfg.BaseDomain,
		NodePoolReplicas: int64(workers),
		ReleaseImage:     s.cfg.ClusterImageSet,
		PullSecret:       pullSecret,
	}

	return s.provisioner.CreateHyperShift(ctx, opts)
}

func (s *suiteContext) hostedClusterExistsInNamespace(ctx context.Context, name, namespace string) error {
	_, err := s.client.Get(ctx, client.GVRHostedCluster, namespace, name)
	if err != nil {
		return fmt.Errorf("HostedCluster %s/%s not found: %w", namespace, name, err)
	}
	return nil
}

func (s *suiteContext) hostedClusterExists(ctx context.Context, name string) error {
	return s.hostedClusterExistsInNamespace(ctx, name, provisioning.DefaultHyperShiftNamespace)
}

func (s *suiteContext) nodePoolExistsWithReplicas(ctx context.Context, name string, replicas int) error {
	obj, err := s.client.Get(ctx, client.GVRNodePool, provisioning.DefaultHyperShiftNamespace, name)
	if err != nil {
		return fmt.Errorf("NodePool %s not found: %w", name, err)
	}
	actual, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if actual != int64(replicas) {
		return fmt.Errorf("NodePool %s has %d replicas, want %d", name, actual, replicas)
	}
	return nil
}

func (s *suiteContext) managedClusterRegistered(ctx context.Context, name string) error {
	_, err := s.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err != nil {
		return fmt.Errorf("ManagedCluster %s not found: %w", name, err)
	}
	return nil
}

func (s *suiteContext) iListHostedClusters(ctx context.Context) error {
	clusters, err := s.provisioner.ListHyperShift(ctx)
	if err != nil {
		return err
	}
	s.hypershiftList = clusters
	return nil
}

func (s *suiteContext) outputShowsHostedCluster(ctx context.Context, name string) error {
	for _, c := range s.hypershiftList {
		if c.Name == name {
			if c.Namespace == "" {
				return fmt.Errorf("HostedCluster %s has empty namespace", name)
			}
			return nil
		}
	}
	return fmt.Errorf("HostedCluster %s not found in list", name)
}

func (s *suiteContext) iDestroyHostedCluster(ctx context.Context, name string) error {
	return s.provisioner.DestroyHyperShift(ctx, provisioning.DefaultHyperShiftNamespace, name)
}

func (s *suiteContext) hostedClusterRemoved(ctx context.Context, name string) error {
	return s.waitForRemoval(ctx, client.GVRHostedCluster, provisioning.DefaultHyperShiftNamespace, name)
}

func (s *suiteContext) nodePoolRemoved(ctx context.Context, name string) error {
	return s.waitForRemoval(ctx, client.GVRNodePool, provisioning.DefaultHyperShiftNamespace, name)
}

func (s *suiteContext) managedClusterDetached(ctx context.Context, name string) error {
	return s.waitForRemoval(ctx, client.GVRManagedCluster, "", name)
}

func (s *suiteContext) waitForRemoval(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	timeout := 5 * time.Minute
	interval := 5 * time.Second
	deadline := time.After(timeout)

	for {
		_, err := s.client.Get(ctx, gvr, namespace, name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("checking removal of %s: %w", name, err)
		}
		select {
		case <-deadline:
			return fmt.Errorf("%s not removed within %v", name, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
