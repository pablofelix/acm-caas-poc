Feature: AI platform operator version fleet segregation (UC-20)
  As a platform operator
  I want each GPU cluster to run a specific operator version
  So that teams testing different versions get dedicated capacity without conflicts

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Route workload to cluster with specific operator version
    Given GPU clusters are labelled "ai-platform-version=2.17" and "ai-platform-version=2.18"
    When I route a request for version "2.18"
    Then only clusters with label "ai-platform-version=2.18" and "gpu-available=true" are considered
    And the first matching cluster name is returned

  Scenario: Enforce single operator version per cluster
    Given a GPU cluster "gpu-h100-eugb" has "ai-platform-version=2.17"
    When I enforce version policy for "gpu-h100-eugb" at version "2.17"
    Then a ConfigurationPolicy checks for exactly one operator version
    And a Policy, Placement, and PlacementBinding are created

  Scenario: Provision cluster for new operator version
    Given a ManagedCluster "gpu-new-cluster" exists
    When I provision it for operator version "2.19"
    Then the cluster is labelled "ai-platform-version=2.19" and "gpu-available=true"
    And a ManifestWork deploys the operator subscription at version "2.19"

  Scenario: List all version-segregated clusters
    When I list version clusters
    Then all ManagedClusters with "ai-platform-version" label are returned
    And each entry includes the version, channel, and availability status
