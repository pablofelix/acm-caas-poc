@slow
Feature: HyperShift hosted cluster provisioning via ACM (UC-38)

  As a platform operator
  I want to provision hosted control plane clusters via the ACM API
  So that teams get lightweight clusters with shared control planes

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  @slow
  Scenario: Create a HostedCluster with NodePool
    Given a pull secret exists for the target platform
    When I run "acmlab provision create hosted-test --type hypershift --release-image quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64 --workers 2 --pull-secret pull-secret.json"
    Then a HostedCluster "hosted-test" is created in namespace "clusters-hosted-test"
    And a NodePool "hosted-test" is created with 2 replicas
    And a ManagedCluster "hosted-test" is registered on the hub

  Scenario: List hosted clusters
    Given a HostedCluster "hosted-test" exists
    When I run "acmlab provision list-hosted"
    Then the output shows "hosted-test" with namespace, availability, and version

  @slow
  Scenario: Destroy a HostedCluster
    Given a HostedCluster "hosted-test" exists
    When I run "acmlab provision destroy hosted-test"
    Then the HostedCluster "hosted-test" is removed
    And the NodePool "hosted-test" is removed
    And the ManagedCluster "hosted-test" is detached from the hub
