//go:build integration

package integration

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/tenant"
)

func registerTenantSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a managed cluster "([^"]*)" exists$`, s.aManagedClusterExistsTenant)
	sc.Step(`^I deploy tenant "([^"]*)" to cluster "([^"]*)" with cpu "([^"]*)" and memory "([^"]*)"$`, s.iDeployTenant)
	sc.Step(`^a ManifestWork "([^"]*)" exists in namespace "([^"]*)"$`, s.manifestWorkExists)
	sc.Step(`^the ManifestWork contains a Namespace, RoleBinding, NetworkPolicy, and ResourceQuota$`, s.manifestWorkContainsResources)
	sc.Step(`^tenant "([^"]*)" is deployed to cluster "([^"]*)"$`, s.tenantIsDeployed)
	sc.Step(`^I list tenants on cluster "([^"]*)"$`, s.iListTenants)
	sc.Step(`^the list includes tenant "([^"]*)"$`, s.listIncludesTenant)
	sc.Step(`^I get the status of tenant "([^"]*)" on cluster "([^"]*)"$`, s.iGetTenantStatus)
	sc.Step(`^I receive the ManifestWork sync status$`, s.receiveManifestWorkStatus)
	sc.Step(`^I remove tenant "([^"]*)" from cluster "([^"]*)"$`, s.iRemoveTenant)
	sc.Step(`^the ManifestWork "([^"]*)" no longer exists in namespace "([^"]*)"$`, s.manifestWorkNoLongerExists)
}

func (s *suiteContext) aManagedClusterExistsTenant(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, err := s.fleet.GetCluster(ctx, name)
	return err
}

func (s *suiteContext) iDeployTenant(ctx context.Context, tenantName, cluster, cpu, memory string) error {
	cluster = s.resolveCluster(cluster)
	return s.tenant.Deploy(ctx, tenant.TenantOpts{
		Name:    tenantName,
		Cluster: cluster,
		Team:    tenantName,
		CPULimit:    cpu,
		MemoryLimit: memory,
	})
}

func (s *suiteContext) manifestWorkExists(ctx context.Context, name, namespace string) error {
	_, err := s.client.Get(ctx, client.GVRManifestWork, namespace, name)
	if err != nil {
		return fmt.Errorf("ManifestWork %s/%s not found: %w", namespace, name, err)
	}
	s.lastManifestWorkName = name
	s.lastManifestWorkNS = namespace
	return nil
}

func (s *suiteContext) manifestWorkContainsResources(ctx context.Context) error {
	name := s.lastManifestWorkName
	ns := s.lastManifestWorkNS
	if name == "" {
		return fmt.Errorf("no ManifestWork name in context — call manifestWorkExists first")
	}
	obj, err := s.client.Get(ctx, client.GVRManifestWork, ns, name)
	if err != nil {
		return fmt.Errorf("ManifestWork %s/%s not found: %w", ns, name, err)
	}
	spec, _ := obj.Object["spec"].(map[string]interface{})
	workload, _ := spec["workload"].(map[string]interface{})
	manifests, _ := workload["manifests"].([]interface{})

	required := map[string]bool{"Namespace": false, "RoleBinding": false, "NetworkPolicy": false, "ResourceQuota": false}
	for _, m := range manifests {
		entry, _ := m.(map[string]interface{})
		kind, _ := entry["kind"].(string)
		if _, ok := required[kind]; ok {
			required[kind] = true
		}
	}
	for kind, found := range required {
		if !found {
			return fmt.Errorf("ManifestWork missing %s", kind)
		}
	}
	return nil
}

func (s *suiteContext) tenantIsDeployed(ctx context.Context, tenantName, cluster string) error {
	cluster = s.resolveCluster(cluster)
	_, err := s.tenant.Status(ctx, tenantName, cluster)
	if err != nil {
		return s.iDeployTenant(ctx, tenantName, cluster, "4", "8Gi")
	}
	return nil
}

func (s *suiteContext) iListTenants(ctx context.Context, cluster string) error {
	cluster = s.resolveCluster(cluster)
	tenants, err := s.tenant.List(ctx, cluster)
	if err != nil {
		return err
	}
	s.tenants = tenants
	return nil
}

func (s *suiteContext) listIncludesTenant(name string) error {
	for _, t := range s.tenants {
		if t.Name == name {
			return nil
		}
	}
	return fmt.Errorf("tenant %s not found in list", name)
}

func (s *suiteContext) iGetTenantStatus(ctx context.Context, tenantName, cluster string) error {
	cluster = s.resolveCluster(cluster)
	status, err := s.tenant.Status(ctx, tenantName, cluster)
	if err != nil {
		return err
	}
	s.manifestStatus = status
	return nil
}

func (s *suiteContext) receiveManifestWorkStatus() error {
	if s.manifestStatus == nil {
		return fmt.Errorf("no ManifestWork status available")
	}
	return nil
}

func (s *suiteContext) iRemoveTenant(ctx context.Context, tenantName, cluster string) error {
	cluster = s.resolveCluster(cluster)
	_, err := s.tenant.Remove(ctx, tenantName, cluster)
	return err
}

func (s *suiteContext) manifestWorkNoLongerExists(ctx context.Context, name, namespace string) error {
	_, err := s.client.Get(ctx, client.GVRManifestWork, namespace, name)
	if err == nil {
		return fmt.Errorf("ManifestWork %s/%s still exists", namespace, name)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("unexpected error checking ManifestWork %s/%s: %w", namespace, name, err)
	}
	return nil
}
