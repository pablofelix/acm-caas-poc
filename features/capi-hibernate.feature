Feature: CAPI cluster hibernate via scale-to-zero (UC-41)

  As a platform operator
  I want to hibernate CAPI clusters by scaling workers to zero
  So that idle vanilla Kubernetes clusters do not incur compute cost

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  Scenario: Hibernate a CAPI cluster scales MachineDeployment to zero
    Given a CAPI cluster "capi-test" with MachineDeployment replicas = 3
    When I run "acmlab lifecycle hibernate capi-test"
    Then the MachineDeployment "capi-test-workers" is patched to 0 replicas
    And annotation "acmlab.redhat.com/pre-hibernate-replicas" is set to "3"

  Scenario: Resume a CAPI cluster restores original replicas
    Given a hibernated CAPI cluster "capi-test" with annotation pre-hibernate-replicas = 3
    When I run "acmlab lifecycle resume capi-test"
    Then the MachineDeployment "capi-test-workers" is patched to 3 replicas

  Scenario: Resume defaults to 2 replicas when annotation is missing
    Given a hibernated CAPI cluster "capi-test" without pre-hibernate annotation
    When I run "acmlab lifecycle resume capi-test"
    Then the MachineDeployment "capi-test-workers" is patched to 2 replicas

  Scenario: Fallback from Hive to CAPI hibernate
    Given a cluster "capi-test" has no ClusterDeployment but has a MachineDeployment
    When I run "acmlab lifecycle hibernate capi-test"
    Then the system falls back to CAPI hibernate
    And the MachineDeployment is scaled to zero
