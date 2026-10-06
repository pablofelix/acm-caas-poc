//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/pablofelix/acm-caas-poc/internal/registry"
)

func registerRegistrySteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a managed cluster "([^"]*)" exists with ManifestWorks$`, s.clusterExistsWithManifestWorks)
	sc.Step(`^I list required images for "([^"]*)"$`, s.iListRequiredImages)
	sc.Step(`^I receive a list of container images extracted from ManifestWorks$`, s.receiveImageList)
	sc.Step(`^I have a list of required images for "([^"]*)"$`, s.iHaveRequiredImages)
	sc.Step(`^I generate a mirror script targeting "([^"]*)"$`, s.iGenerateMirrorScript)
	sc.Step(`^the script contains skopeo copy commands for each image$`, s.scriptContainsSkopeoCopy)
	sc.Step(`^images have been mirrored to "([^"]*)"$`, s.imagesHaveBeenMirrored)
	sc.Step(`^I configure a registry mirror for "([^"]*)" with target "([^"]*)"$`, s.iConfigureRegistryMirror)
	sc.Step(`^a ManagedClusterImageRegistry exists for "([^"]*)"$`, s.imageRegistryExists)
	sc.Step(`^a Placement with tolerations exists for "([^"]*)"$`, s.placementExists)
	sc.Step(`^a registry mirror is configured for "([^"]*)"$`, s.registryMirrorConfigured)
	sc.Step(`^I get the mirror status for "([^"]*)"$`, s.iGetMirrorStatus)
	sc.Step(`^I receive the mirror configuration and registry mappings$`, s.receiveMirrorConfig)
	sc.Step(`^I remove the registry mirror for "([^"]*)"$`, s.iRemoveRegistryMirror)
	sc.Step(`^the ManagedClusterImageRegistry for "([^"]*)" is removed$`, s.imageRegistryRemoved)
}

func (s *suiteContext) clusterExistsWithManifestWorks(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, err := s.fleet.GetCluster(ctx, name)
	return err
}

func (s *suiteContext) iListRequiredImages(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	images, err := s.registry.ListRequiredImages(ctx, name)
	if err != nil {
		return err
	}
	s.images = images
	return nil
}

func (s *suiteContext) receiveImageList() error {
	if len(s.images) == 0 {
		return fmt.Errorf("no images returned")
	}
	return nil
}

func (s *suiteContext) iHaveRequiredImages(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	if len(s.images) == 0 {
		return s.iListRequiredImages(ctx, name)
	}
	return nil
}

func (s *suiteContext) iGenerateMirrorScript(target string) error {
	s.mirrorScript = registry.GenerateMirrorScript(s.images, target)
	return nil
}

func (s *suiteContext) scriptContainsSkopeoCopy() error {
	if !strings.Contains(s.mirrorScript, "skopeo copy") {
		return fmt.Errorf("mirror script does not contain skopeo copy commands")
	}
	return nil
}

func (s *suiteContext) imagesHaveBeenMirrored(_ string) error {
	return nil
}

func (s *suiteContext) iConfigureRegistryMirror(ctx context.Context, cluster, target string) error {
	cluster = s.resolveCluster(cluster)
	return s.registry.ConfigureMirror(ctx, registry.MirrorConfig{
		ClusterName:    cluster,
		MirrorRegistry: target,
		Registries:     registry.DefaultRegistryMappings(target),
	})
}

func (s *suiteContext) imageRegistryExists(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	status, err := s.registry.GetMirrorStatus(ctx, name)
	if err != nil {
		return err
	}
	if !status.Configured {
		return fmt.Errorf("ManagedClusterImageRegistry not configured for %s", name)
	}
	return nil
}

func (s *suiteContext) placementExists(_ string) error {
	return nil
}

func (s *suiteContext) registryMirrorConfigured(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	status, err := s.registry.GetMirrorStatus(ctx, name)
	if err != nil {
		return fmt.Errorf("mirror not configured for %s: %w", name, err)
	}
	if !status.Configured {
		return fmt.Errorf("mirror not configured for %s", name)
	}
	return nil
}

func (s *suiteContext) iGetMirrorStatus(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	status, err := s.registry.GetMirrorStatus(ctx, name)
	if err != nil {
		return err
	}
	s.mirrorStatus = status
	return nil
}

func (s *suiteContext) receiveMirrorConfig() error {
	if s.mirrorStatus == nil {
		return fmt.Errorf("no mirror status available")
	}
	return nil
}

func (s *suiteContext) iRemoveRegistryMirror(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	return s.registry.RemoveMirror(ctx, name)
}

func (s *suiteContext) imageRegistryRemoved(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	status, err := s.registry.GetMirrorStatus(ctx, name)
	if err != nil {
		return nil
	}
	if status.Configured {
		return fmt.Errorf("ManagedClusterImageRegistry still exists for %s", name)
	}
	return nil
}
