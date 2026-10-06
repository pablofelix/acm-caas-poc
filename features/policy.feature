@core
Feature: Governance policy management
  As a platform operator
  I want to restrict container images to approved registries
  So that the ComputeRequest controller can enforce supply-chain security

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Create an image registry restriction policy
    When I apply policy "test-registry-policy" with registries "registry.redhat.io,quay.io"
    Then the policy "test-registry-policy" exists on the hub
    And the policy has a Placement and PlacementBinding

  Scenario: List governance policies
    Given policy "test-registry-policy" exists
    When I list all policies
    Then the policy list includes "test-registry-policy"

  Scenario: Check policy compliance status
    Given policy "test-registry-policy" exists
    When I get the status of policy "test-registry-policy"
    Then I receive compliance information per cluster

  Scenario: Change policy remediation mode
    Given policy "test-registry-policy" exists
    When I set remediation of "test-registry-policy" to "enforce"
    Then the policy remediation is "enforce"

  @slow
  Scenario: Enforce reconciliation after manual drift
    Given policy "test-registry-policy" exists
    And I set remediation of "test-registry-policy" to "enforce"
    And the cluster "infraops1" eventually becomes Compliant for "test-registry-policy"
    When I manually remove the allowedRegistries config from "infraops1"
    Then ACM re-enforces and the allowedRegistries config is restored on "infraops1"

  Scenario: Remove a policy and its bindings
    Given policy "test-registry-policy" exists
    When I remove policy "test-registry-policy"
    Then the policy "test-registry-policy" no longer exists
    And the Placement and PlacementBinding are removed
