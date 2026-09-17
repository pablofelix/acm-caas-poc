Feature: Automatic cluster reclamation via TTL labels (UC-14)
  As a platform operator
  I want to set time-to-live policies on clusters
  So that idle or expired clusters are automatically reclaimed

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Set TTL on a cluster
    Given a ManagedCluster "spoke-test" exists
    When I set a TTL of 72 hours on "spoke-test" with owner "team-alpha"
    Then the ManagedCluster "spoke-test" has label "caas/ttl-hours" = "72"
    And the ManagedCluster "spoke-test" has label "caas/expiry-date" set to 72 hours from now

  Scenario: Check for expired clusters
    Given a ManagedCluster "spoke-old" exists with an expired TTL
    And a ManagedCluster "spoke-new" exists with a valid TTL
    When I check for expired clusters
    Then "spoke-old" appears in the expired list
    And "spoke-new" does not appear in the expired list

  Scenario: Extend TTL on a cluster
    Given a ManagedCluster "spoke-test" has a TTL set
    When I extend the TTL by 48 hours with justification "sprint extension"
    Then the expiry date is pushed forward by 48 hours
    And the justification is recorded in the cluster labels

  @slow
  Scenario: Reclaim an expired Hive-provisioned cluster
    Given a ManagedCluster "spoke-expired" has an expired TTL
    And "spoke-expired" has a ClusterDeployment (Hive-provisioned)
    When I reclaim "spoke-expired"
    Then the cluster is hibernated via ClusterDeployment powerState
