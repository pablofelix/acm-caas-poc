Feature: Elastic GPU capacity — automatic on-demand provisioning (UC-21)
  As a platform operator
  I want the GPU pool to scale automatically
  So that teams are not rejected when shared GPU clusters are full

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Detect GPU saturation above threshold
    Given a GPU cluster "gpu-h100-eugb" has label "gpu-utilization=92"
    When I check saturation with threshold 85
    Then the cluster is reported as saturated
    And the utilization and threshold are included in the status

  Scenario: Detect GPU utilization below threshold
    Given a GPU cluster "gpu-h100-eugb" has label "gpu-utilization=60"
    When I check saturation with threshold 85
    Then the cluster is reported as not saturated

  Scenario: Provision on-demand GPU cluster
    Given a ManagedCluster "gpu-ondemand-001" exists
    When I provision it as on-demand with GPU type "L4"
    Then the cluster is labelled "gpu-cost-tier=on-demand" and "gpu-elastic=true"
    And a ManifestWork deploys the elastic stack marker

  Scenario: Hibernate idle on-demand cluster
    Given a GPU cluster "gpu-ondemand-001" has label "gpu-elastic=true"
    When I hibernate the idle cluster
    Then the cluster label "gpu-available" is set to "false"
    And the cluster label "gpu-elastic-state" is set to "hibernated"

  Scenario: List all elastic GPU clusters
    When I list elastic clusters
    Then all ManagedClusters with "gpu-elastic=true" label are returned
    And each entry includes the GPU type, cost tier, and availability
