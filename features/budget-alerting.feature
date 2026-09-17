Feature: Cost center attribution and budget alerting (UC-17)
  As a platform operator
  I want to attribute costs to teams and alert when budgets are exceeded
  So that cloud spend is controlled and transparent

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Stamp cost center on a cluster
    Given a ManagedCluster "spoke-test" exists
    When I stamp cost center "engineering" on "spoke-test"
    Then the ManagedCluster "spoke-test" has label "caas/cost-center" = "engineering"

  Scenario: Aggregate costs by cost center
    Given clusters "spoke-a" and "spoke-b" are stamped with cost center "platform"
    And cluster "spoke-c" is stamped with cost center "data-science"
    When I aggregate costs by cost center
    Then the "platform" center shows the combined cost of "spoke-a" and "spoke-b"
    And the "data-science" center shows the cost of "spoke-c"

  Scenario: Create budget policy for a cost center
    Given cost center "platform" exists across the fleet
    When I create a budget policy for "platform" with limit 5000
    Then an ACM Policy is created to monitor "platform" spend
    And a Placement targets clusters with label "caas/cost-center" = "platform"
    And a PlacementBinding links the policy to the placement

  Scenario: Check budgets and detect overage
    Given a budget policy exists for "platform" with limit 5000
    And the estimated cost for "platform" clusters exceeds 5000
    When I check budgets
    Then "platform" is reported as over budget
