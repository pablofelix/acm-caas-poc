Feature: SCAP scanning via Compliance Operator (UC-31)

  As a platform operator
  I want to deploy and run SCAP compliance scans on spoke clusters
  So that clusters are validated against security benchmarks

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Deploy Compliance Operator to a cluster
    Given a cluster "spoke1" is registered in ACM
    When I run "acmlab security deploy-compliance spoke1 --cluster-set default"
    Then an OperatorPolicy deploys the Compliance Operator to "spoke1"
    And a ConfigurationPolicy monitors operator health

  Scenario: Create a compliance scan
    Given the Compliance Operator is deployed on "spoke1"
    When I run "acmlab security scan spoke1 --profile cis-node"
    Then a ConfigurationPolicy triggers a ComplianceScan on "spoke1"
    And the scan targets the "cis-node" profile

  Scenario: Check scan status
    Given a compliance scan is running on "spoke1"
    When I run "acmlab security scan-status spoke1"
    Then the output shows scan phase, compliant count, and non-compliant count

  Scenario: Get compliance report
    Given a compliance scan has completed on "spoke1"
    When I run "acmlab security compliance-report spoke1"
    Then the output shows each rule with status and severity

  Scenario: Remove compliance scan resources
    Given compliance resources exist for "spoke1"
    When I run "acmlab security remove-compliance spoke1"
    Then all compliance policies and placements for "spoke1" are removed
