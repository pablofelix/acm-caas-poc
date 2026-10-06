@slow @pool
Feature: ClusterPool and ClusterClaim for pre-warmed clusters (UC-25)
  As a platform operator
  I want to maintain a pool of pre-provisioned clusters
  So that teams can claim one in seconds instead of waiting 40 minutes

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  @slow @pool
  Scenario: Create a cluster pool with 2 standby clusters
    Given a ClusterImageSet for the target OCP version exists
    When I create a ClusterPool "caas-pool" on platform "aws" with size 2
    Then the ClusterPool "caas-pool" exists with size 2
    And the pool "caas-pool" eventually has 2 ready clusters

  @slow @pool
  Scenario: Claim a cluster from the pool
    Given the pool "caas-pool" has at least 1 ready cluster
    When I claim a cluster from pool "caas-pool" as "test-claim" with TTL "4h"
    Then the ClusterClaim "test-claim" is bound to a cluster
    And the pool "caas-pool" provisions a replacement cluster

  @pool
  Scenario: List pool status and claims
    When I list cluster pools
    Then I see pool "caas-pool" with size, ready, and claimed counts
    When I list claims in pool "caas-pool"
    Then I see claim "test-claim" with its assigned cluster

  @slow @pool
  Scenario: Release claim and destroy pool
    Given I have a claim "test-claim" in pool "caas-pool"
    When I release claim "test-claim" from pool "caas-pool"
    Then the ClusterClaim "test-claim" is removed
    When I delete pool "caas-pool"
    Then the ClusterPool "caas-pool" is removed
    And all pool clusters are destroyed
