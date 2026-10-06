//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"

	"os"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/importing"
)

func registerImportingSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^I have a kubeconfig for external cluster "([^"]*)"$`, s.iHaveKubeconfigForCluster)
	sc.Step(`^I import cluster "([^"]*)" with labels "([^"]*)"$`, s.iImportClusterWithLabels)
	sc.Step(`^a ManagedCluster "([^"]*)" exists on the hub$`, s.managedClusterExistsOnHub)
	sc.Step(`^a KlusterletAddonConfig exists in namespace "([^"]*)"$`, s.klusterletAddonConfigExists)
	sc.Step(`^an auto-import secret exists in namespace "([^"]*)"$`, s.autoImportSecretExists)
	sc.Step(`^cluster "([^"]*)" has been imported$`, s.clusterHasBeenImported)
	sc.Step(`^I get the import status of "([^"]*)"$`, s.iGetImportStatus)
	sc.Step(`^I receive availability and join state$`, s.receiveAvailabilityAndJoinState)
	sc.Step(`^I list all imported clusters$`, s.iListAllImportedClusters)
	sc.Step(`^the imported list includes "([^"]*)"$`, s.importListIncludes)
	sc.Step(`^I detach cluster "([^"]*)"$`, s.iDetachCluster)
	sc.Step(`^the ManagedCluster "([^"]*)" is removed from the hub$`, s.managedClusterRemovedFromHub)
}

func (s *suiteContext) iHaveKubeconfigForCluster(name string) error {
	envKey := fmt.Sprintf("IMPORT_KUBECONFIG_%s", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	path := os.Getenv(envKey)
	if path == "" {
		path = os.Getenv("IMPORT_KUBECONFIG")
	}
	if path == "" {
		return fmt.Errorf("set %s or IMPORT_KUBECONFIG to the external cluster kubeconfig path", envKey)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading kubeconfig from %s: %w", path, err)
	}
	s.importKubeconfig = data
	return nil
}

func (s *suiteContext) iImportClusterWithLabels(ctx context.Context, name, labelStr string) error {
	name = s.resolveCluster(name)
	labels := map[string]string{}
	for _, pair := range strings.Split(labelStr, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}
	_, err := s.importing.Import(ctx, importing.ImportOptions{
		Name:       name,
		Labels:     labels,
		Kubeconfig: s.importKubeconfig,
	})
	return err
}

func (s *suiteContext) managedClusterExistsOnHub(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, err := s.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err != nil {
		return fmt.Errorf("ManagedCluster %s not found: %w", name, err)
	}
	return nil
}

func (s *suiteContext) klusterletAddonConfigExists(ctx context.Context, ns string) error {
	_, err := s.client.Get(ctx, client.GVRKlusterletAddonConfig, ns, ns)
	if err != nil {
		return fmt.Errorf("KlusterletAddonConfig not found in %s: %w", ns, err)
	}
	return nil
}

func (s *suiteContext) autoImportSecretExists(ctx context.Context, ns string) error {
	_, err := s.client.Get(ctx, client.GVRSecret, ns, "auto-import-secret")
	if err != nil {
		return fmt.Errorf("auto-import-secret not found in %s: %w", ns, err)
	}
	return nil
}

func (s *suiteContext) clusterHasBeenImported(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	imported, err := s.importing.IsImported(ctx, name)
	if err != nil {
		return err
	}
	if !imported {
		_, err = s.importing.Import(ctx, importing.ImportOptions{Name: name})
		return err
	}
	return nil
}

func (s *suiteContext) iGetImportStatus(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	status, err := s.importing.GetImportStatus(ctx, name)
	if err != nil {
		return err
	}
	s.importStatus = status
	return nil
}

func (s *suiteContext) receiveAvailabilityAndJoinState() error {
	if s.importStatus == nil {
		return fmt.Errorf("no import status available")
	}
	return nil
}

func (s *suiteContext) iListAllImportedClusters(ctx context.Context) error {
	list, err := s.importing.ListImported(ctx)
	if err != nil {
		return err
	}
	s.importList = list
	return nil
}

func (s *suiteContext) importListIncludes(name string) error {
	for _, i := range s.importList {
		if i.Name == name {
			return nil
		}
	}
	return fmt.Errorf("cluster %s not found in imported list", name)
}

func (s *suiteContext) iDetachCluster(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	return s.importing.Detach(ctx, name)
}

func (s *suiteContext) managedClusterRemovedFromHub(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, err := s.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err == nil {
		return fmt.Errorf("ManagedCluster %s still exists", name)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("unexpected error checking ManagedCluster %s: %w", name, err)
	}
	return nil
}
