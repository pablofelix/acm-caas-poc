Feature: Cost tracking and chargeback via node metadata estimation (UC-11)
  As a platform operator
  I want to estimate cluster costs from ManagedClusterInfo node metadata
  So that teams can be charged for their compute consumption

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Get cost estimate for a single cluster
    Given a ManagedCluster "spoke-test" exists with node metadata
    When I request the cost estimate for "spoke-test"
    Then I receive an estimated hourly and monthly cost based on instance types
    And the estimate includes node count, CPU hours, and memory hours

  Scenario: Generate a fleet-wide cost report
    Given multiple ManagedClusters exist with node metadata
    When I generate a cost report for the fleet
    Then I receive a report with per-cluster cost estimates
    And the report can be exported as CSV or JSON

  Scenario: Identify clusters with no cost center
    Given a ManagedCluster "spoke-test" exists without a cost center label
    When I generate a cost report
    Then "spoke-test" appears with cost center "unassigned"
