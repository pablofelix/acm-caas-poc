@slow
Feature: CAPI provisioning for vanilla Kubernetes clusters (UC-40)

  As a platform operator
  I want to provision vanilla Kubernetes clusters via Cluster API
  So that teams can run workloads on non-OpenShift Kubernetes

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  @slow
  Scenario: Create a CAPI Cluster with MachineDeployment
    Given CAPI infrastructure provider "docker" is available
    When I run "acmlab provision create capi-test --type capi --kubernetes-version v1.30.0 --workers 2"
    Then a CAPI Cluster "capi-test" is created in namespace "capi-test"
    And a MachineDeployment "capi-test-workers" is created with 2 replicas
    And a ManagedCluster "capi-test" is registered on the hub

  Scenario: List CAPI clusters
    Given a CAPI Cluster "capi-test" exists
    When I run "acmlab provision list-capi"
    Then the output shows "capi-test" with namespace, phase, readiness, and Kubernetes version

  @slow
  Scenario: Destroy a CAPI cluster
    Given a CAPI Cluster "capi-test" exists
    When I run "acmlab provision destroy capi-test"
    Then the CAPI Cluster "capi-test" is removed
    And the MachineDeployment "capi-test-workers" is removed
    And the ManagedCluster "capi-test" is detached from the hub
