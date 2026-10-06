@observability
Feature: Observability stack customisation (UC-52)

  As a platform operator
  I want to customise the ACM observability stack
  So that I can configure production storage, custom metrics, rules, dashboards, and alerting

  Background:
    Given the ACM hub is reachable
    And I have a dynamic client for the hub

  # --- Setup and Storage (Reqs 1, 2, 3) ---

  Scenario: Setup observability with MinIO backend
    When I set up the observability stack with backend "minio"
    Then the namespace "open-cluster-management-observability" exists
    And the pull secret "multiclusterhub-operator-pull-secret" exists in the observability namespace
    And a MinIO deployment exists in the observability namespace
    And a MultiClusterObservability "observability" is created

  Scenario: Setup observability with OBC backend
    When I set up the observability stack with backend "obc" and storage class "openshift-storage.noobaa.io"
    Then the namespace "open-cluster-management-observability" exists
    And the pull secret "multiclusterhub-operator-pull-secret" exists in the observability namespace
    And an ObjectBucketClaim "observability-obc" is created in the observability namespace
    And a Secret "thanos-object-storage" exists with the OBC-derived credentials
    And a MultiClusterObservability "observability" is created
    And no MinIO deployment exists in the observability namespace

  Scenario: Separate storage classes for OBC and volumes
    When I set up with OBC storage class "noobaa.io" and volume storage class "gp3-csi"
    Then the OBC references storage class "noobaa.io"
    And the MCO spec.storageConfig.storageClass is "gp3-csi"

  Scenario: Backend switch is rejected
    Given a MinIO-based observability stack is running
    When I attempt setup with backend "obc"
    Then the operation fails with a backend mismatch error

  # --- Pull Secret (Req 1) ---

  Scenario: Pull secret copy fails when source is missing
    When I attempt to copy the pull secret without a source
    Then the operation fails with an error about the missing pull secret

  # --- Status and Verification (Reqs 4, 14) ---

  Scenario: Verify observability deployment
    Given the observability stack is set up
    When I verify the observability deployment
    Then the result includes MCO status
    And the result includes workload readiness
    And the result includes PVC status
    And the result includes addon health per managed cluster
    And the result indicates whether metrics reception was verified

  Scenario: Status distinguishes pending, progressing, and ready
    Given a MultiClusterObservability exists without status
    When I check observability status
    Then the status is "Pending"

  # --- Grafana URL Discovery (Req 5) ---

  Scenario: Discover Grafana URL
    Given a Route "grafana" exists in the observability namespace with host "grafana.apps.example.com"
    When I request the Grafana URL
    Then the URL is "https://grafana.apps.example.com"

  Scenario: Grafana URL not found
    When I request the Grafana URL without a Grafana route
    Then the operation fails with guidance on how to check Grafana availability

  # --- Enable/Disable per Cluster (Req 6) ---

  Scenario: Disable observability for a managed cluster
    Given a ManagedCluster "spoke1" with labels "vendor=OpenShift"
    When I disable observability for cluster "spoke1"
    Then the ManagedCluster "spoke1" has label "observability=disabled"
    And the label "vendor=OpenShift" is preserved

  Scenario: Enable observability for a managed cluster
    Given a ManagedCluster "spoke1" with label "observability=disabled"
    When I enable observability for cluster "spoke1"
    Then the ManagedCluster "spoke1" does not have label "observability"
    And other labels are preserved

  # --- Custom Rules (Req 9) ---

  Scenario: Deploy custom recording and alerting rules
    Given a valid rules YAML with groups, recording rules, and alert rules
    When I deploy custom rules
    Then a ConfigMap "thanos-ruler-custom-rules" is created in the observability namespace
    And the rules are stored under the "custom_rules.yaml" key

  Scenario: Update existing custom rules
    Given custom rules are deployed
    When I deploy updated rules
    Then the ConfigMap "thanos-ruler-custom-rules" is updated with the new content

  Scenario: Invalid rules YAML is rejected
    Given a rules YAML missing the "groups" field
    When I attempt to deploy custom rules
    Then the operation fails with a validation error about the YAML structure
    And the error indicates that PromQL semantics were not validated

  Scenario: Remove custom rules
    Given custom rules are deployed
    When I remove custom rules
    Then the ConfigMap "thanos-ruler-custom-rules" is deleted

  # --- Metrics Configuration (Req 10) ---

  Scenario: Configure global platform metrics
    When I configure metrics with scope "global" and metrics "node_cpu_seconds_total,container_memory_rss"
    Then the ConfigMap "observability-metrics-custom-allowlist" is created
    And the "metrics_list.yaml" key contains the specified metrics

  @pending
  Scenario: Configure user workload metrics
    When I configure metrics with scope "workload" and metrics "my_app_requests_total"
    Then the operation indicates that user workload monitoring must be configured on each managed cluster

  @pending
  Scenario: Configure per-cluster metrics
    When I configure metrics with scope "cluster" for cluster "spoke1"
    Then the operation indicates that direct access to the managed cluster is required

  # --- Advanced MCO Configuration (Req 11) ---

  Scenario: Configure advanced MCO settings
    Given the observability stack is set up
    When I configure advanced settings with receive replicas 6 and collection interval 30
    Then the MCO spec.advanced.receive.replicas is 6
    And the MCO spec.observabilityAddonSpec.interval is 30

  Scenario: Partial MCO update preserves existing fields
    Given the MCO has retention "24h" and block duration "2h"
    When I update only the delete delay to "48h"
    Then the retention remains "24h"
    And the block duration remains "2h"
    And the delete delay is "48h"

  # --- Dashboards (Req 12) ---

  Scenario: Deploy a custom Grafana dashboard
    Given a valid JSON dashboard definition
    When I deploy dashboard "gpu-overview"
    Then a ConfigMap "gpu-overview" is created with label "grafana-custom-dashboard=true"

  Scenario: Update an existing dashboard
    Given a dashboard "gpu-overview" is deployed
    When I deploy dashboard "gpu-overview" with updated JSON
    Then the ConfigMap "gpu-overview" is updated with the new content

  Scenario: Invalid dashboard JSON is rejected
    When I attempt to deploy a dashboard with invalid JSON
    Then the operation fails with a JSON validation error

  # --- Alertmanager (Req 13) ---

  Scenario: Configure Alertmanager
    Given a valid Alertmanager configuration YAML
    When I configure Alertmanager
    Then a Secret "alertmanager-config" is created in the observability namespace
    And the configuration is stored under the "alertmanager.yaml" key

  Scenario: Invalid Alertmanager config is rejected
    Given an Alertmanager configuration missing "receivers"
    When I attempt to configure Alertmanager
    Then the operation fails with a validation error

  Scenario: Disable alert forwarding
    Given the observability stack is set up
    When I disable alert forwarding
    Then the MCO has annotation "mco-disable-alerting=true"

  Scenario: Enable alert forwarding
    Given alert forwarding is disabled
    When I enable alert forwarding
    Then the MCO does not have annotation "mco-disable-alerting"
