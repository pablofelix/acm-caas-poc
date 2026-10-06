//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/policy"
)

func registerPolicySteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^I apply policy "([^"]*)" with registries "([^"]*)"$`, s.iApplyPolicy)
	sc.Step(`^the policy "([^"]*)" exists on the hub$`, s.policyExistsOnHub)
	sc.Step(`^the policy has a Placement and PlacementBinding$`, s.policyHasBindings)
	sc.Step(`^policy "([^"]*)" exists$`, s.policyExists)
	sc.Step(`^I list all policies$`, s.iListAllPolicies)
	sc.Step(`^the policy list includes "([^"]*)"$`, s.listIncludesPolicy)
	sc.Step(`^I get the status of policy "([^"]*)"$`, s.iGetPolicyStatus)
	sc.Step(`^I receive compliance information per cluster$`, s.receiveComplianceInfo)
	sc.Step(`^I set remediation of "([^"]*)" to "([^"]*)"$`, s.iSetRemediation)
	sc.Step(`^the policy remediation is "([^"]*)"$`, s.policyRemediationIs)
	sc.Step(`^I remove policy "([^"]*)"$`, s.iRemovePolicy)
	sc.Step(`^the policy "([^"]*)" no longer exists$`, s.policyNoLongerExists)
	sc.Step(`^the Placement and PlacementBinding are removed$`, s.bindingsRemoved)
	sc.Step(`^the cluster "([^"]*)" eventually becomes Compliant for "([^"]*)"$`, s.clusterBecomesCompliant)
	sc.Step(`^I manually remove the allowedRegistries config from "([^"]*)"$`, s.removeAllowedRegistries)
	sc.Step(`^the cluster "([^"]*)" becomes NonCompliant for "([^"]*)"$`, s.clusterBecomesNonCompliant)
	sc.Step(`^ACM re-enforces and "([^"]*)" becomes Compliant again for "([^"]*)"$`, s.clusterBecomesCompliantAgain)
	sc.Step(`^ACM re-enforces and the allowedRegistries config is restored on "([^"]*)"$`, s.allowedRegistriesRestored)
}

func (s *suiteContext) iApplyPolicy(ctx context.Context, name, registries string) error {
	ns := "default"
	binding := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1beta2",
			"kind":       "ManagedClusterSetBinding",
			"metadata":   map[string]interface{}{"name": "global", "namespace": ns},
			"spec":       map[string]interface{}{"clusterSet": "global"},
		},
	}
	_ = s.client.CreateIfNotExists(ctx, client.GVRManagedClusterSetBinding, ns, binding)

	regs := strings.Split(registries, ",")
	for i := range regs {
		regs[i] = strings.TrimSpace(regs[i])
	}
	return s.policy.Apply(ctx, policy.PolicyOpts{
		Name:              name,
		Namespace:         ns,
		AllowedRegistries: regs,
		RemediationAction: "inform",
	})
}

func (s *suiteContext) policyExistsOnHub(ctx context.Context, name string) error {
	info, err := s.policy.Get(ctx, name, "default")
	if err != nil {
		return fmt.Errorf("policy %s not found: %w", name, err)
	}
	s.policyInfo = info
	fmt.Printf("\n  ┌─ Policy %q exists on hub\n", name)
	fmt.Printf("  │  Namespace: default, Remediation: %s, Compliance: %s\n", info.RemediationAction, info.Compliant)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) policyHasBindings(ctx context.Context) error {
	if s.policyInfo == nil {
		return fmt.Errorf("no policy info available")
	}
	name := s.policyInfo.Name
	ns := "default"
	placementName := name + "-placement"
	bindingName := name + "-placement-binding"
	if _, err := s.client.Get(ctx, client.GVRPlacement, ns, placementName); err != nil {
		return fmt.Errorf("Placement %s/%s not found: %w", ns, placementName, err)
	}
	if _, err := s.client.Get(ctx, client.GVRPlacementBinding, ns, bindingName); err != nil {
		return fmt.Errorf("PlacementBinding %s/%s not found: %w", ns, bindingName, err)
	}
	fmt.Printf("\n  ┌─ Policy bindings verified\n")
	fmt.Printf("  │  Placement:        %s/%s\n", ns, placementName)
	fmt.Printf("  │  PlacementBinding: %s/%s\n", ns, bindingName)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) policyExists(ctx context.Context, name string) error {
	info, err := s.policy.Get(ctx, name, "default")
	if err != nil {
		if applyErr := s.iApplyPolicy(ctx, name, "registry.redhat.io"); applyErr != nil {
			return applyErr
		}
		info, err = s.policy.Get(ctx, name, "default")
		if err != nil {
			return err
		}
	}
	s.policyInfo = info
	return nil
}

func (s *suiteContext) iListAllPolicies(ctx context.Context) error {
	policies, err := s.policy.List(ctx, "default")
	if err != nil {
		return err
	}
	s.policies = policies
	return nil
}

func (s *suiteContext) listIncludesPolicy(name string) error {
	fmt.Printf("\n  ┌─ Policies: %d\n", len(s.policies))
	found := false
	for _, p := range s.policies {
		marker := " "
		if p.Name == name {
			marker = "→"
			found = true
		}
		fmt.Printf("  │ %s %-30s remediation=%-8s compliance=%s\n", marker, p.Name, p.RemediationAction, p.Compliant)
	}
	fmt.Printf("  └─\n")
	if !found {
		return fmt.Errorf("policy %s not found in list", name)
	}
	return nil
}

func (s *suiteContext) iGetPolicyStatus(ctx context.Context, name string) error {
	info, err := s.policy.Get(ctx, name, "default")
	if err != nil {
		return err
	}
	s.policyInfo = info
	return nil
}

func (s *suiteContext) receiveComplianceInfo() error {
	if s.policyInfo == nil {
		return fmt.Errorf("no policy info available")
	}
	fmt.Printf("\n  ┌─ Compliance for %q: %s\n", s.policyInfo.Name, s.policyInfo.Compliant)
	if len(s.policyInfo.ClusterCompliance) > 0 {
		for _, cs := range s.policyInfo.ClusterCompliance {
			fmt.Printf("  │  %-20s %s\n", cs.ClusterName, cs.ComplianceState)
		}
	} else {
		fmt.Printf("  │  (no per-cluster status yet — policy may still be propagating)\n")
	}
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) iSetRemediation(ctx context.Context, name, action string) error {
	return s.policy.SetRemediation(ctx, name, "default", action)
}

func (s *suiteContext) policyRemediationIs(ctx context.Context, action string) error {
	if s.policyInfo == nil {
		return fmt.Errorf("no policy info")
	}
	info, err := s.policy.Get(ctx, s.policyInfo.Name, "default")
	if err != nil {
		return err
	}
	if info.RemediationAction != action {
		return fmt.Errorf("remediation = %s, want %s", info.RemediationAction, action)
	}
	fmt.Printf("\n  ┌─ Remediation updated\n")
	fmt.Printf("  │  Policy: %s, Remediation: %s\n", info.Name, info.RemediationAction)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) iRemovePolicy(ctx context.Context, name string) error {
	_, err := s.policy.Remove(ctx, name, "default")
	return err
}

func (s *suiteContext) policyNoLongerExists(ctx context.Context, name string) error {
	_, err := s.policy.Get(ctx, name, "default")
	if err == nil {
		return fmt.Errorf("policy %s still exists", name)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("unexpected error checking policy %s: %w", name, err)
	}
	fmt.Printf("\n  ┌─ Policy %q confirmed removed\n", name)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) bindingsRemoved(ctx context.Context) error {
	if s.policyInfo == nil {
		return fmt.Errorf("no policy info available")
	}
	name := s.policyInfo.Name
	ns := "default"
	placementName := name + "-placement"
	bindingName := name + "-placement-binding"
	if _, err := s.client.Get(ctx, client.GVRPlacement, ns, placementName); err == nil {
		return fmt.Errorf("Placement %s/%s still exists", ns, placementName)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("unexpected error checking Placement %s: %w", placementName, err)
	}
	if _, err := s.client.Get(ctx, client.GVRPlacementBinding, ns, bindingName); err == nil {
		return fmt.Errorf("PlacementBinding %s/%s still exists", ns, bindingName)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("unexpected error checking PlacementBinding %s: %w", bindingName, err)
	}
	fmt.Printf("\n  ┌─ Bindings confirmed removed\n")
	fmt.Printf("  │  Placement:        %s/%s (gone)\n", ns, placementName)
	fmt.Printf("  │  PlacementBinding: %s/%s (gone)\n", ns, bindingName)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) clusterComplianceForPolicy(ctx context.Context, cluster, policyName string) (string, error) {
	info, err := s.policy.Get(ctx, policyName, "default")
	if err != nil {
		return "", err
	}
	for _, cc := range info.ClusterCompliance {
		if cc.ClusterName == cluster {
			return cc.ComplianceState, nil
		}
	}
	return "", nil
}

func (s *suiteContext) clusterBecomesCompliant(ctx context.Context, cluster, policyName string) error {
	cluster = s.resolveCluster(cluster)
	timeout := 3 * time.Minute
	interval := 10 * time.Second
	deadline := time.After(timeout)
	for {
		state, err := s.clusterComplianceForPolicy(ctx, cluster, policyName)
		if err != nil {
			return err
		}
		if state == "Compliant" {
			fmt.Printf("\n  ┌─ Cluster %q is Compliant for %q\n", cluster, policyName)
			fmt.Printf("  └─\n")
			return nil
		}
		fmt.Printf("  │  waiting... %s = %q\n", cluster, state)
		select {
		case <-deadline:
			return fmt.Errorf("cluster %s did not become Compliant for %s within %v (last state: %s)", cluster, policyName, timeout, state)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) removeAllowedRegistries(ctx context.Context, cluster string) error {
	cluster = s.resolveCluster(cluster)
	patch := []byte(`[{"op":"remove","path":"/spec/registrySources"}]`)
	_, err := s.client.Patch(ctx, client.GVRImageConfig, "", "cluster", types.JSONPatchType, patch)
	if err != nil {
		return fmt.Errorf("removing allowedRegistries from %s: %w", cluster, err)
	}
	fmt.Printf("\n  ┌─ Manually removed allowedRegistries from %q\n", cluster)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) clusterBecomesNonCompliant(ctx context.Context, cluster, policyName string) error {
	cluster = s.resolveCluster(cluster)
	timeout := 3 * time.Minute
	interval := 10 * time.Second
	deadline := time.After(timeout)
	for {
		state, err := s.clusterComplianceForPolicy(ctx, cluster, policyName)
		if err != nil {
			return err
		}
		if state == "NonCompliant" {
			fmt.Printf("\n  ┌─ Cluster %q detected drift: NonCompliant for %q\n", cluster, policyName)
			fmt.Printf("  └─\n")
			return nil
		}
		fmt.Printf("  │  waiting for drift detection... %s = %q\n", cluster, state)
		select {
		case <-deadline:
			return fmt.Errorf("cluster %s did not become NonCompliant for %s within %v (last state: %s)", cluster, policyName, timeout, state)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) allowedRegistriesRestored(ctx context.Context, cluster string) error {
	cluster = s.resolveCluster(cluster)
	timeout := 3 * time.Minute
	interval := 5 * time.Second
	deadline := time.After(timeout)
	for {
		obj, err := s.client.Get(ctx, client.GVRImageConfig, "", "cluster")
		if err != nil {
			return fmt.Errorf("getting Image config: %w", err)
		}
		registries, found, _ := unstructured.NestedStringSlice(obj.Object, "spec", "registrySources", "allowedRegistries")
		if found && len(registries) > 0 {
			fmt.Printf("\n  ┌─ ACM re-enforced allowedRegistries on %q\n", cluster)
			for _, r := range registries {
				fmt.Printf("  │  %s\n", r)
			}
			fmt.Printf("  └─\n")
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("allowedRegistries not restored on %s within %v", cluster, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) clusterBecomesCompliantAgain(ctx context.Context, cluster, policyName string) error {
	cluster = s.resolveCluster(cluster)
	timeout := 3 * time.Minute
	interval := 10 * time.Second
	deadline := time.After(timeout)
	for {
		state, err := s.clusterComplianceForPolicy(ctx, cluster, policyName)
		if err != nil {
			return err
		}
		if state == "Compliant" {
			fmt.Printf("\n  ┌─ ACM re-enforced: %q is Compliant again for %q\n", cluster, policyName)
			fmt.Printf("  └─\n")
			return nil
		}
		fmt.Printf("  │  waiting for re-enforcement... %s = %q\n", cluster, state)
		select {
		case <-deadline:
			return fmt.Errorf("cluster %s did not become Compliant again for %s within %v (last state: %s)", cluster, policyName, timeout, state)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
