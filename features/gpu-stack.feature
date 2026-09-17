Feature: GPU sharing stack deployment via ManifestWork (UC-18)
  As a platform operator
  I want the GPU sharing stack to be automatically deployed and maintained on all GPU clusters
  So that teams can share GPU resources without manual operator intervention

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Deploy Kueue and Kyverno to a GPU cluster
    Given a ManagedCluster "gpu-h100-eugb" exists with label "gpu-sharing=enabled"
    When I deploy the GPU sharing stack to "gpu-h100-eugb"
    Then a ManifestWork "gpu-h100-eugb-kueue-stack" exists in namespace "gpu-h100-eugb"
    And a ManifestWork "gpu-h100-eugb-kyverno-gpu" exists in namespace "gpu-h100-eugb"
    And a ConfigurationPolicy monitors Kueue and Kyverno health

  Scenario: Detect GPU stack drift
    Given a GPU cluster "gpu-h100-eugb" has the sharing stack deployed
    When the Kueue operator is degraded on the cluster
    Then drift detection reports the cluster as non-compliant
    And the degraded components are listed in the drift status

  Scenario: Create ClusterQueues per GPU type
    Given a GPU cluster "gpu-h100-eugb" has H100 and L4 nodes
    When I create ClusterQueues for GPU types "H100,L4"
    Then a ManifestWork with ResourceFlavors and ClusterQueues per type exists

  @slow
  Scenario: Remove GPU sharing stack
    Given a GPU cluster "gpu-h100-eugb" has the sharing stack deployed
    When I remove the GPU sharing stack from "gpu-h100-eugb"
    Then the ManifestWorks and ConfigurationPolicy are deleted
