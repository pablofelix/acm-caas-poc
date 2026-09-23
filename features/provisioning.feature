@slow
Feature: Cluster provisioning via Hive ClusterDeployment
  As a platform operator
  I want to provision spoke clusters programmatically via the ACM API
  So that the ComputeRequest controller can automate this

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  @slow
  Scenario: Create a ClusterDeployment and wait for provisioning
    Given cloud credentials exist as a Secret in namespace "spoke-test"
    And a ClusterImageSet for the target OCP version exists
    When I provision cluster "spoke-test" with default settings
    Then the ClusterDeployment "spoke-test" is accepted by Hive
    And the cluster "spoke-test" eventually reaches Provisioned = True

  @slow @aws
  Scenario: Create an AWS ClusterDeployment and wait for provisioning
    Given cloud credentials exist as a Secret in namespace "spoke1"
    And a ClusterImageSet for the target OCP version exists
    When I provision cluster "spoke1" with default settings
    Then the ClusterDeployment "spoke1" is accepted by Hive
    And the cluster "spoke1" eventually reaches Provisioned = True

  @core
  Scenario: Register IBM Cloud credentials via ACM Credentials API
    Given the ACM hub is reachable
    When I register IBM Cloud credentials as an ACM central credential "ibm-caas-creds"
    Then the ACM credential "ibm-caas-creds" exists in open-cluster-management namespace
    And the credential has provider type "ibm"

  Scenario: List provisioned clusters
    When I list all provisioned clusters
    Then I receive a list of ClusterDeployments with status

  @slow
  Scenario: Delete a ClusterDeployment and verify cleanup
    Given a ClusterDeployment "spoke-test" exists with status Provisioned = True
    When I destroy cluster "spoke-test"
    Then the ClusterDeployment "spoke-test" is removed
    And the ManagedCluster "spoke-test" is removed from the hub
