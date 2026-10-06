@slow @pool @manual-pool
Feature: Manual ClusterPool for IBM Cloud (UC-25)
  As a platform operator
  I want to maintain a pool of pre-provisioned clusters on IBM Cloud
  So that teams can claim one quickly instead of waiting 40 minutes
  Note: Hive ClusterPool does not support IBM Cloud (ADR-017),
  so we use manual pool orchestration via provisioning + lifecycle packages

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  @slow @pool @manual-pool
  Scenario: Create a manual pool with 2 clusters on IBM Cloud
    Given a ClusterImageSet for the target OCP version exists
    When I create a manual pool "caas-pool" on platform "ibmcloud" in region "us-east" with size 2
    Then the manual pool "caas-pool" has 2 clusters provisioning

  @slow @pool @manual-pool
  Scenario: Wait for manual pool clusters to be ready and hibernated
    Given the manual pool "caas-pool" exists
    When I wait for manual pool "caas-pool" to be ready
    Then the manual pool "caas-pool" has 2 standby clusters

  @slow @pool @manual-pool
  Scenario: Claim a cluster from the manual pool
    Given the manual pool "caas-pool" has at least 1 standby cluster
    When I claim a cluster from manual pool "caas-pool" as "test-claim"
    Then the claimed cluster is running
    And the manual pool "caas-pool" has 1 claimed and 1 standby

  @pool @manual-pool
  Scenario: List manual pool status
    When I list manual pool "caas-pool"
    Then I see pool "caas-pool" with size 2, showing claimed and standby counts

  @slow @pool @manual-pool
  Scenario: Release claim and keep clusters
    Given I have a claimed cluster "test-claim" in pool "caas-pool"
    When I release manual claim "test-claim"
    Then the cluster is hibernated
    And the manual pool "caas-pool" has 2 standby clusters
