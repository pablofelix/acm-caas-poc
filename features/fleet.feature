Feature: Fleet-wide observability from the hub
  As a platform operator
  I want to query cluster health programmatically
  So that the ComputeRequest controller can reflect spoke state in its status

  Scenario: List managed clusters and read their conditions
    Given the ACM hub is healthy
    When I list ManagedCluster resources
    Then each cluster has a name and condition list
    And available clusters report Available = True

  Scenario: Get a specific managed cluster by name
    Given the ACM hub is healthy
    And a ManagedCluster "local-cluster" exists
    When I get ManagedCluster "local-cluster"
    Then the cluster info includes name, labels, and conditions
