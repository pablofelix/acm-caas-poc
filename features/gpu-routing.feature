Feature: Multi-cluster GPU workload routing via ACM Placement (UC-19)
  As a platform engineer
  I want users to request GPUs by type without knowing which cluster has them
  So that the service routes automatically to available capacity

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Route H100 request to correct cluster
    Given GPU clusters exist with labels "gpu-type=H100" and "gpu-available=true"
    When I create a GPU Placement for type "H100"
    Then a Placement with gpu-type and gpu-available predicates is created
    And the PlacementDecision selects the matching cluster

  Scenario: Mark cluster as saturated
    Given a GPU cluster "gpu-h100-eugb" has label "gpu-available=true"
    When I mark "gpu-h100-eugb" as saturated
    Then the ManagedCluster label "gpu-available" is set to "false"
    And new GPU requests are not routed to this cluster

  Scenario: Clear saturation flag
    Given a GPU cluster "gpu-h100-eugb" has label "gpu-available=false"
    When I clear the saturation flag on "gpu-h100-eugb"
    Then the ManagedCluster label "gpu-available" is set to "true"

  Scenario: Select best cluster for GPU type
    Given multiple GPU clusters with type "H100" and "gpu-available=true"
    When I request the best cluster for type "H100"
    Then a temporary Placement is created and evaluated
    And the first cluster from the PlacementDecision is returned
    And the temporary Placement is cleaned up
