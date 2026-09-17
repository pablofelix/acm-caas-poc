Feature: Cloud-native scaling for HyperShift and CAPI clusters (UC-39)

  As a platform operator
  I want to scale worker nodes on HyperShift and CAPI clusters
  So that capacity adjusts regardless of provisioning backend

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Scale NodePool replicas on a HyperShift cluster
    Given a HyperShift cluster "hosted-test" with a NodePool of 2 replicas
    When I run "acmlab scaling set hosted-test --replicas 4"
    Then the NodePool "hosted-test" is patched to 4 replicas

  Scenario: Scale MachineDeployment replicas on a CAPI cluster
    Given a CAPI cluster "capi-test" with a MachineDeployment of 2 replicas
    When I run "acmlab scaling set capi-test --replicas 3"
    Then the MachineDeployment "capi-test-workers" is patched to 3 replicas

  Scenario: Auto-detect scaling resource type
    Given a cluster "spoke1" exists with an unknown provisioning backend
    When I run "acmlab scaling set spoke1 --replicas 4"
    Then the system tries MachinePool, then NodePool, then MachineDeployment
    And the first matching resource is scaled to 4 replicas

  Scenario: Error when no scaling resource exists
    Given an imported cluster "imported-test" with no MachinePool, NodePool, or MachineDeployment
    When I run "acmlab scaling set imported-test --replicas 4"
    Then the command fails with an error indicating no scaling resource was found
