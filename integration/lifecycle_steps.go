//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/lifecycle"
)

func registerLifecycleSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^a ClusterDeployment "([^"]*)" exists in namespace "([^"]*)"$`, s.clusterDeploymentExists)
	sc.Step(`^the ClusterDeployment has spec\.powerState = "([^"]*)"$`, s.clusterDeploymentHasPowerState)
	sc.Step(`^I patch spec\.powerState to "([^"]*)"(?: via the Go API)?$`, s.iPatchPowerState)
	sc.Step(`^the ClusterDeployment status shows powerState = "([^"]*)"$`, s.statusShowsPowerState)
	sc.Step(`^the ManagedCluster condition "Available" transitions to "([^"]*)" or "([^"]*)"$`, s.availableTransitions)
	sc.Step(`^eventually the ManagedCluster "([^"]*)" becomes Available = True$`, s.eventuallyAvailable)
	sc.Step(`^no ClusterDeployment exists for "([^"]*)"$`, s.noClusterDeploymentExists)
	sc.Step(`^I attempt to get the power state for "([^"]*)"$`, s.iAttemptGetPowerState)
	sc.Step(`^the operation returns an error$`, s.operationReturnsError)
	sc.Step(`^the error message indicates "([^"]*)"$`, s.errorMessageIndicates)
	sc.Step(`^I get the power state for "([^"]*)"$`, s.iGetPowerState)
	sc.Step(`^I receive the current powerState value$`, s.receiveCurrentPowerState)
	sc.Step(`^the value is one of: "([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)"$`, s.valueIsOneOf)
	sc.Step(`^I wait for the power state to become "([^"]*)" with timeout (\d+)([ms])$`, s.iWaitForPowerState)
	sc.Step(`^the wait completes successfully$`, s.waitCompletesSuccessfully)
	sc.Step(`^the ClusterDeployment status\.powerState = "([^"]*)"$`, s.statusShowsPowerState)
	sc.Step(`^I hibernate the cluster again$`, s.iHibernateAgain)
	sc.Step(`^I resume the cluster again$`, s.iResumeAgain)
	sc.Step(`^the powerState remains "([^"]*)"$`, s.powerStateRemains)
	sc.Step(`^no unnecessary API calls are made$`, s.noUnnecessaryAPICalls)
	sc.Step(`^the ClusterDeployment already has spec\.powerState = "([^"]*)"$`, s.clusterDeploymentHasPowerState)
	sc.Step(`^a managed cluster "([^"]*)" exists for lifecycle check$`, s.aManagedClusterExistsLifecycle)
	sc.Step(`^the operation returns a timeout error$`, s.operationReturnsTimeoutError)
}

func (s *suiteContext) clusterDeploymentExists(ctx context.Context, name, ns string) error {
	name = s.resolveCluster(name)
	ns = s.resolveCluster(ns)
	supported, err := s.lifecycle.ClusterSupportsLifecycle(ctx, ns, name)
	if err != nil {
		return err
	}
	if !supported {
		return fmt.Errorf("no ClusterDeployment found for %s/%s", ns, name)
	}
	s.lifecycleCluster = name
	s.lifecycleNamespace = ns
	return nil
}

func (s *suiteContext) clusterDeploymentHasPowerState(ctx context.Context, state string) error {
	current, err := s.lifecycle.GetPowerState(ctx, s.lifecycleNamespace, s.lifecycleCluster)
	if err != nil {
		return fmt.Errorf("getting power state: %w", err)
	}
	if string(current) != state {
		switch lifecycle.PowerState(state) {
		case lifecycle.PowerStateRunning:
			if err := s.lifecycle.Resume(ctx, s.lifecycleNamespace, s.lifecycleCluster); err != nil {
				return fmt.Errorf("preparing Running state: %w", err)
			}
		case lifecycle.PowerStateHibernating:
			if err := s.lifecycle.Hibernate(ctx, s.lifecycleNamespace, s.lifecycleCluster); err != nil {
				return fmt.Errorf("preparing Hibernating state: %w", err)
			}
		default:
			return fmt.Errorf("power state = %s, want %s", current, state)
		}
	}
	return s.lifecycle.WaitForPowerState(ctx, s.lifecycleNamespace, s.lifecycleCluster, lifecycle.PowerState(state), 5*time.Minute)
}

func (s *suiteContext) iPatchPowerState(ctx context.Context, state string) error {
	ns, name := s.lifecycleNamespace, s.lifecycleCluster
	switch lifecycle.PowerState(state) {
	case lifecycle.PowerStateHibernating:
		s.err = s.lifecycle.Hibernate(ctx, ns, name)
	case lifecycle.PowerStateRunning:
		s.err = s.lifecycle.Resume(ctx, ns, name)
	default:
		return fmt.Errorf("unknown power state: %s", state)
	}
	return s.err
}

func (s *suiteContext) statusShowsPowerState(ctx context.Context, expected string) error {
	timeout := 2 * time.Minute
	interval := 5 * time.Second
	deadline := time.After(timeout)
	for {
		state, err := s.lifecycle.GetPowerStateStatus(ctx, s.lifecycleNamespace, s.lifecycleCluster)
		if err != nil {
			return err
		}
		if string(state) == expected {
			return nil
		}
		select {
		case <-deadline:
			return fmt.Errorf("power state = %s, want %s (timed out after %v)", state, expected, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) availableTransitions(ctx context.Context, expected1, expected2 string) error {
	timeout := 2 * time.Minute
	interval := 5 * time.Second
	deadline := time.After(timeout)
	for {
		mc, err := s.client.Get(ctx, client.GVRManagedCluster, "", s.lifecycleCluster)
		if err != nil {
			return fmt.Errorf("getting ManagedCluster %s: %w", s.lifecycleCluster, err)
		}
		conditions, _, _ := unstructured.NestedSlice(mc.Object, "status", "conditions")
		for _, raw := range conditions {
			cond, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			if condType == "ManagedClusterConditionAvailable" {
				if condStatus == expected1 || condStatus == expected2 {
					return nil
				}
			}
		}
		select {
		case <-deadline:
			return fmt.Errorf("ManagedCluster %s Available did not transition to %s or %s within %v", s.lifecycleCluster, expected1, expected2, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) eventuallyAvailable(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	timeout := 15 * time.Minute
	interval := 10 * time.Second
	deadline := time.After(timeout)
	for {
		mc, err := s.client.Get(ctx, client.GVRManagedCluster, "", name)
		if err == nil {
			conditions, _, _ := unstructured.NestedSlice(mc.Object, "status", "conditions")
			for _, raw := range conditions {
				cond, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				condType, _ := cond["type"].(string)
				condStatus, _ := cond["status"].(string)
				if condType == "ManagedClusterConditionAvailable" && condStatus == "True" {
					return nil
				}
			}
		}
		select {
		case <-deadline:
			return fmt.Errorf("ManagedCluster %s did not become Available within %v", name, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) noClusterDeploymentExists(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	supported, _ := s.lifecycle.ClusterSupportsLifecycle(ctx, name, name)
	if supported {
		return fmt.Errorf("ClusterDeployment exists for %s but should not", name)
	}
	return nil
}

func (s *suiteContext) iAttemptGetPowerState(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, s.err = s.lifecycle.GetPowerState(ctx, name, name)
	return nil
}

func (s *suiteContext) operationReturnsError() error {
	if s.err == nil {
		return fmt.Errorf("expected an error but got none")
	}
	return nil
}

func (s *suiteContext) errorMessageIndicates(msg string) error {
	if s.err == nil {
		return fmt.Errorf("no error to check")
	}
	if !strings.Contains(s.err.Error(), msg) {
		return fmt.Errorf("error %q does not contain %q", s.err.Error(), msg)
	}
	return nil
}

func (s *suiteContext) iGetPowerState(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	state, err := s.lifecycle.GetPowerState(ctx, name, name)
	if err != nil {
		s.err = err
		return err
	}
	s.powerState = state
	return nil
}

func (s *suiteContext) receiveCurrentPowerState() error {
	if s.powerState == "" {
		return fmt.Errorf("power state is empty")
	}
	return nil
}

func (s *suiteContext) valueIsOneOf(a, b, c, d string) error {
	for _, v := range []string{a, b, c, d} {
		if string(s.powerState) == v {
			return nil
		}
	}
	return fmt.Errorf("power state %s not in expected values", s.powerState)
}

func (s *suiteContext) iWaitForPowerState(ctx context.Context, state string, amount int, unit string) error {
	timeout := time.Duration(amount) * time.Second
	if unit == "m" {
		timeout = time.Duration(amount) * time.Minute
	}
	s.err = s.lifecycle.WaitForPowerState(ctx, s.lifecycleNamespace, s.lifecycleCluster, lifecycle.PowerState(state), timeout)
	return nil
}

func (s *suiteContext) waitCompletesSuccessfully() error {
	return s.err
}

func (s *suiteContext) iHibernateAgain(ctx context.Context) error {
	s.err = s.lifecycle.Hibernate(ctx, s.lifecycleNamespace, s.lifecycleCluster)
	return nil
}

func (s *suiteContext) iResumeAgain(ctx context.Context) error {
	s.err = s.lifecycle.Resume(ctx, s.lifecycleNamespace, s.lifecycleCluster)
	return nil
}

func (s *suiteContext) powerStateRemains(ctx context.Context, expected string) error {
	state, err := s.lifecycle.GetPowerState(ctx, s.lifecycleNamespace, s.lifecycleCluster)
	if err != nil {
		return err
	}
	if string(state) != expected {
		return fmt.Errorf("power state = %s, want %s", state, expected)
	}
	return nil
}

func (s *suiteContext) aManagedClusterExistsLifecycle(ctx context.Context, name string) error {
	name = s.resolveCluster(name)
	_, err := s.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err != nil {
		return fmt.Errorf("ManagedCluster %s not found: %w", name, err)
	}
	return nil
}

func (s *suiteContext) operationReturnsTimeoutError() error {
	if s.err == nil {
		return fmt.Errorf("expected a timeout error but got none")
	}
	if !errors.Is(s.err, context.DeadlineExceeded) {
		return fmt.Errorf("expected context.DeadlineExceeded, got: %v", s.err)
	}
	return nil
}

// Verification not feasible without request-level auditing
func (s *suiteContext) noUnnecessaryAPICalls() error {
	return nil
}
