//go:build integration

package integration

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/virtualization"
)

func registerVMSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a cluster "([^"]*)" is registered in ACM$`, s.vmClusterRegistered)
	sc.Step(`^I run "acmlab vm deploy --name ([^ ]+) --cluster ([^ ]+) --cpu (\d+) --memory ([^"]+)"$`, s.iRunVMDeploy)
	sc.Step(`^a ManifestWork "([^"]*)" is created in namespace "([^"]*)"$`, s.vmManifestWorkExists)
	sc.Step(`^the ManifestWork wraps a kubevirt\.io/v1 VirtualMachine$`, s.vmManifestWorkWrapsVM)
	sc.Step(`^the VM is configured with (\d+) CPU cores and ([^ ]+) memory$`, s.vmConfiguredWithCPUAndMemory)
	sc.Step(`^I run "acmlab vm deploy --name ([^ ]+) --cluster ([^ "]+)"$`, s.iRunVMDeployDefaults)
	sc.Step(`^the VM defaults to (\d+) CPU cores, ([^ ]+) memory, and (.+) image$`, s.vmDefaultsApplied)
	sc.Step(`^a VM "([^"]*)" is deployed to "([^"]*)" in stopped state$`, s.vmDeployedStopped)
	sc.Step(`^I run "acmlab vm start ([^ ]+) --cluster ([^"]+)"$`, s.iRunVMStart)
	sc.Step(`^the ManifestWork is updated with spec\.running=true$`, s.vmManifestWorkRunningIs(true))
	sc.Step(`^a VM "([^"]*)" is running on "([^"]*)"$`, s.vmDeployedRunning)
	sc.Step(`^I run "acmlab vm stop ([^ ]+) --cluster ([^"]+)"$`, s.iRunVMStop)
	sc.Step(`^the ManifestWork is updated with spec\.running=false$`, s.vmManifestWorkRunningIs(false))
	sc.Step(`^I run "acmlab vm migrate ([^ ]+) --cluster ([^"]+)"$`, s.iRunVMMigrate)
	sc.Step(`^a VirtualMachineInstanceMigration is added to the ManifestWork$`, s.vmMigrationAdded)
	sc.Step(`^a VM "([^"]*)" is deployed to "([^"]*)"$`, s.vmDeployed)
	sc.Step(`^I run "acmlab vm status ([^ ]+) --cluster ([^"]+)"$`, s.iRunVMStatus)
	sc.Step(`^the output shows name, cluster, status, CPU, memory, and image$`, s.vmStatusOutputComplete)
	sc.Step(`^VMs are deployed to "([^"]*)" and "([^"]*)"$`, s.vmsDeployedToMultipleClusters)
	sc.Step(`^I run "acmlab vm list"$`, s.iRunVMList)
	sc.Step(`^the output shows name, cluster, status, and IP for all VMs$`, s.vmListOutputComplete)
	sc.Step(`^I run "acmlab vm remove ([^ ]+) --cluster ([^"]+)"$`, s.iRunVMRemove)
	sc.Step(`^the ManifestWork is deleted from namespace "([^"]*)"$`, s.vmManifestWorkDeleted)
}

func (s *suiteContext) vmClusterRegistered(ctx context.Context, name string) error {
	if s.client == nil {
		if err := s.theACMHubIsReachable(ctx); err != nil {
			return err
		}
	}
	resolved := s.resolveCluster(name)
	_, err := s.client.Get(ctx, client.GVRManagedCluster, "", resolved)
	if err != nil {
		return fmt.Errorf("cluster %s not registered in ACM: %w", resolved, err)
	}
	return nil
}

func (s *suiteContext) ensureVMClient(ctx context.Context) error {
	if s.client == nil {
		return s.theACMHubIsReachable(ctx)
	}
	return nil
}

func (s *suiteContext) iRunVMDeploy(ctx context.Context, name, cluster string, cpu int, memory string) error {
	if err := s.ensureVMClient(ctx); err != nil {
		return err
	}
	resolved := s.resolveCluster(cluster)
	opts := virtualization.VMOpts{
		Name:    name,
		Cluster: resolved,
		CPU:     strconv.Itoa(cpu),
		Memory:  memory,
	}
	s.lastManifestWorkName = fmt.Sprintf("vm-%s-%s", name, resolved)
	s.lastManifestWorkNS = resolved
	return s.vm.Deploy(ctx, opts)
}

func (s *suiteContext) iRunVMDeployDefaults(ctx context.Context, name, cluster string) error {
	if err := s.ensureVMClient(ctx); err != nil {
		return err
	}
	resolved := s.resolveCluster(cluster)
	opts := virtualization.VMOpts{
		Name:    name,
		Cluster: resolved,
	}
	s.lastManifestWorkName = fmt.Sprintf("vm-%s-%s", name, resolved)
	s.lastManifestWorkNS = resolved
	return s.vm.Deploy(ctx, opts)
}

func (s *suiteContext) vmManifestWorkExists(ctx context.Context, name, namespace string) error {
	resolved := s.resolveCluster(namespace)
	mwName := s.lastManifestWorkName
	if mwName == "" {
		mwName = name
	}
	_, err := s.client.Get(ctx, client.GVRManifestWork, resolved, mwName)
	if err != nil {
		return fmt.Errorf("ManifestWork %s/%s not found: %w", resolved, mwName, err)
	}
	return nil
}

func (s *suiteContext) vmManifestWorkWrapsVM(ctx context.Context) error {
	obj, err := s.client.Get(ctx, client.GVRManifestWork, s.lastManifestWorkNS, s.lastManifestWorkName)
	if err != nil {
		return fmt.Errorf("ManifestWork not found: %w", err)
	}
	manifests, _, _ := unstructured.NestedSlice(obj.Object, "spec", "workload", "manifests")
	if len(manifests) == 0 {
		return fmt.Errorf("ManifestWork has no manifests")
	}
	vm, ok := manifests[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("first manifest is not a map")
	}
	apiVersion, _ := vm["apiVersion"].(string)
	kind, _ := vm["kind"].(string)
	if apiVersion != "kubevirt.io/v1" || kind != "VirtualMachine" {
		return fmt.Errorf("expected kubevirt.io/v1 VirtualMachine, got %s %s", apiVersion, kind)
	}
	return nil
}

func (s *suiteContext) vmConfiguredWithCPUAndMemory(ctx context.Context, cpu int, memory string) error {
	obj, err := s.client.Get(ctx, client.GVRManifestWork, s.lastManifestWorkNS, s.lastManifestWorkName)
	if err != nil {
		return err
	}
	manifests, _, _ := unstructured.NestedSlice(obj.Object, "spec", "workload", "manifests")
	if len(manifests) == 0 {
		return fmt.Errorf("no manifests")
	}
	vm, _ := manifests[0].(map[string]interface{})
	spec, _ := vm["spec"].(map[string]interface{})
	tmpl, _ := spec["template"].(map[string]interface{})
	domain, _ := tmpl["spec"].(map[string]interface{})["domain"].(map[string]interface{})

	cpuMap, _ := domain["cpu"].(map[string]interface{})
	cores, _ := cpuMap["cores"].(int64)
	if cores != int64(cpu) {
		return fmt.Errorf("CPU cores = %d, want %d", cores, cpu)
	}

	memMap, _ := domain["memory"].(map[string]interface{})
	guest, _ := memMap["guest"].(string)
	if guest != memory {
		return fmt.Errorf("memory = %s, want %s", guest, memory)
	}
	return nil
}

func (s *suiteContext) vmDefaultsApplied(ctx context.Context, cpu int, memory, image string) error {
	obj, err := s.client.Get(ctx, client.GVRManifestWork, s.lastManifestWorkNS, s.lastManifestWorkName)
	if err != nil {
		return err
	}
	manifests, _, _ := unstructured.NestedSlice(obj.Object, "spec", "workload", "manifests")
	if len(manifests) == 0 {
		return fmt.Errorf("no manifests")
	}
	vm, _ := manifests[0].(map[string]interface{})
	spec, _ := vm["spec"].(map[string]interface{})
	tmpl, _ := spec["template"].(map[string]interface{})
	domain, _ := tmpl["spec"].(map[string]interface{})["domain"].(map[string]interface{})

	cpuMap, _ := domain["cpu"].(map[string]interface{})
	cores, _ := cpuMap["cores"].(int64)
	if cores != int64(cpu) {
		return fmt.Errorf("default CPU = %d, want %d", cores, cpu)
	}

	memMap, _ := domain["memory"].(map[string]interface{})
	guest, _ := memMap["guest"].(string)
	if guest != memory {
		return fmt.Errorf("default memory = %s, want %s", guest, memory)
	}

	annotations := obj.GetAnnotations()
	vmImage := annotations["acmlab.redhat.com/vm-image"]
	if strings.HasPrefix(image, "RHEL") {
		if !strings.Contains(strings.ToLower(vmImage), "rhel") {
			return fmt.Errorf("default image = %s, expected RHEL image", vmImage)
		}
	} else if vmImage != image {
		return fmt.Errorf("image = %s, want %s", vmImage, image)
	}
	return nil
}

func (s *suiteContext) vmDeployedStopped(ctx context.Context, name, cluster string) error {
	if err := s.ensureVMClient(ctx); err != nil {
		return err
	}
	resolved := s.resolveCluster(cluster)
	opts := virtualization.VMOpts{
		Name:    name,
		Cluster: resolved,
	}
	s.lastManifestWorkName = fmt.Sprintf("vm-%s-%s", name, resolved)
	s.lastManifestWorkNS = resolved
	if err := s.vm.Deploy(ctx, opts); err != nil {
		return err
	}
	return s.vm.Stop(ctx, name, resolved)
}

func (s *suiteContext) vmDeployedRunning(ctx context.Context, name, cluster string) error {
	if err := s.ensureVMClient(ctx); err != nil {
		return err
	}
	resolved := s.resolveCluster(cluster)
	opts := virtualization.VMOpts{
		Name:    name,
		Cluster: resolved,
	}
	s.lastManifestWorkName = fmt.Sprintf("vm-%s-%s", name, resolved)
	s.lastManifestWorkNS = resolved
	return s.vm.Deploy(ctx, opts)
}

func (s *suiteContext) vmDeployed(ctx context.Context, name, cluster string) error {
	return s.vmDeployedRunning(ctx, name, cluster)
}

func (s *suiteContext) iRunVMStart(ctx context.Context, name, cluster string) error {
	resolved := s.resolveCluster(cluster)
	return s.vm.Start(ctx, name, resolved)
}

func (s *suiteContext) iRunVMStop(ctx context.Context, name, cluster string) error {
	resolved := s.resolveCluster(cluster)
	return s.vm.Stop(ctx, name, resolved)
}

func (s *suiteContext) vmManifestWorkRunningIs(expected bool) func(context.Context) error {
	return func(ctx context.Context) error {
		obj, err := s.client.Get(ctx, client.GVRManifestWork, s.lastManifestWorkNS, s.lastManifestWorkName)
		if err != nil {
			return err
		}
		manifests, _, _ := unstructured.NestedSlice(obj.Object, "spec", "workload", "manifests")
		if len(manifests) == 0 {
			return fmt.Errorf("no manifests")
		}
		vm, _ := manifests[0].(map[string]interface{})
		spec, _ := vm["spec"].(map[string]interface{})
		running, ok := spec["running"].(bool)
		if !ok {
			return fmt.Errorf("spec.running not found")
		}
		if running != expected {
			return fmt.Errorf("spec.running = %v, want %v", running, expected)
		}
		return nil
	}
}

func (s *suiteContext) iRunVMMigrate(ctx context.Context, name, cluster string) error {
	resolved := s.resolveCluster(cluster)
	s.lastManifestWorkName = fmt.Sprintf("vm-%s-%s", name, resolved)
	s.lastManifestWorkNS = resolved
	return s.vm.Migrate(ctx, name, resolved)
}

func (s *suiteContext) vmMigrationAdded(ctx context.Context) error {
	obj, err := s.client.Get(ctx, client.GVRManifestWork, s.lastManifestWorkNS, s.lastManifestWorkName)
	if err != nil {
		return err
	}
	manifests, _, _ := unstructured.NestedSlice(obj.Object, "spec", "workload", "manifests")
	for _, m := range manifests {
		manifest, _ := m.(map[string]interface{})
		kind, _ := manifest["kind"].(string)
		if kind == "VirtualMachineInstanceMigration" {
			return nil
		}
	}
	return fmt.Errorf("VirtualMachineInstanceMigration not found in ManifestWork manifests")
}

func (s *suiteContext) iRunVMStatus(ctx context.Context, name, cluster string) error {
	resolved := s.resolveCluster(cluster)
	detail, err := s.vm.Status(ctx, name, resolved)
	if err != nil {
		return err
	}
	s.vmDetail = &detail
	return nil
}

func (s *suiteContext) vmStatusOutputComplete(ctx context.Context) error {
	if s.vmDetail == nil {
		return fmt.Errorf("no VM detail available")
	}
	if s.vmDetail.Name == "" {
		return fmt.Errorf("VM name is empty")
	}
	if s.vmDetail.Cluster == "" {
		return fmt.Errorf("VM cluster is empty")
	}
	if s.vmDetail.Status == "" {
		return fmt.Errorf("VM status is empty")
	}
	fmt.Printf("\n  ┌─ VM %s\n", s.vmDetail.Name)
	fmt.Printf("  │  cluster=%s status=%s cpu=%s memory=%s image=%s\n",
		s.vmDetail.Cluster, s.vmDetail.Status, s.vmDetail.CPU, s.vmDetail.Memory, s.vmDetail.Image)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) vmsDeployedToMultipleClusters(ctx context.Context, cluster1, cluster2 string) error {
	if err := s.ensureVMClient(ctx); err != nil {
		return err
	}
	r1 := s.resolveCluster(cluster1)
	r2 := s.resolveCluster(cluster2)
	for _, r := range []struct{ name, cluster string }{
		{"fleet-vm-1", r1},
		{"fleet-vm-2", r2},
	} {
		opts := virtualization.VMOpts{Name: r.name, Cluster: r.cluster}
		if err := s.vm.Deploy(ctx, opts); err != nil {
			return fmt.Errorf("deploying %s to %s: %w", r.name, r.cluster, err)
		}
	}
	return nil
}

func (s *suiteContext) iRunVMList(ctx context.Context) error {
	vms, err := s.vm.List(ctx)
	if err != nil {
		return err
	}
	s.vmList = vms
	return nil
}

func (s *suiteContext) vmListOutputComplete(ctx context.Context) error {
	if len(s.vmList) == 0 {
		return fmt.Errorf("VM list is empty")
	}
	for _, vm := range s.vmList {
		if vm.Name == "" || vm.Cluster == "" || vm.Status == "" {
			return fmt.Errorf("VM entry missing required fields: %+v", vm)
		}
	}
	return nil
}

func (s *suiteContext) iRunVMRemove(ctx context.Context, name, cluster string) error {
	resolved := s.resolveCluster(cluster)
	s.lastManifestWorkName = fmt.Sprintf("vm-%s-%s", name, resolved)
	s.lastManifestWorkNS = resolved
	return s.vm.Remove(ctx, name, resolved)
}

func (s *suiteContext) vmManifestWorkDeleted(ctx context.Context, namespace string) error {
	resolved := s.resolveCluster(namespace)
	return s.waitForRemoval(ctx, client.GVRManifestWork, resolved, s.lastManifestWorkName)
}
