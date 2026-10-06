#!/usr/bin/env bash
# UC-52: Observability stack customisation (manual)
# Shows raw ACM resources: pull secret, OBC, custom rules, dashboards, metrics allowlist

set -euo pipefail

NS="open-cluster-management-observability"

echo "=== UC-52: Observability Stack Customisation (manual) ==="

echo "1. Copy pull secret from openshift-config to observability namespace"
oc get secret multiclusterhub-operator-pull-secret -n openshift-config -o yaml \
  | sed "s/namespace: openshift-config/namespace: ${NS}/" \
  | oc apply -f -

echo ""
echo "2. Create ObjectBucketClaim for Thanos storage"
cat <<EOF | oc apply -f -
apiVersion: objectbucket.io/v1alpha1
kind: ObjectBucketClaim
metadata:
  name: observability-obc
  namespace: ${NS}
spec:
  generateBucketName: thanos
  storageClassName: gp3-csi
EOF

echo ""
echo "3. Deploy custom Prometheus recording/alerting rules"
cat <<EOF | oc apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: thanos-ruler-custom-rules
  namespace: ${NS}
data:
  custom_rules.yaml: |
    groups:
      - name: acmlab-custom
        rules:
          - alert: HighCPUUsage
            expr: cluster:cpu_usage_cores:sum > 80
            for: 10m
            labels:
              severity: warning
            annotations:
              summary: "High CPU usage on {{ \$labels.cluster }}"
EOF

echo ""
echo "4. Deploy custom Grafana dashboard"
cat <<EOF | oc apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: gpu-overview
  namespace: ${NS}
  labels:
    grafana-custom-dashboard: "true"
data:
  gpu-overview.json: |
    {
      "title": "GPU Fleet Overview",
      "panels": [],
      "schemaVersion": 30
    }
EOF

echo ""
echo "5. Configure custom metrics allowlist"
cat <<EOF | oc apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: observability-metrics-custom-allowlist
  namespace: ${NS}
data:
  metrics_list.yaml: |
    names:
      - node_cpu_seconds_total
      - container_memory_rss
      - kube_pod_container_resource_requests
EOF

echo ""
echo "6. Configure Alertmanager"
cat <<EOF | oc apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-config
  namespace: ${NS}
type: Opaque
stringData:
  alertmanager.yaml: |
    route:
      receiver: default
      group_by: ['alertname', 'cluster']
    receivers:
      - name: default
EOF

echo ""
echo "7. Disable alert forwarding"
oc annotate multiclusterobservabilities.observability.open-cluster-management.io observability \
  mco-disable-alerting="true" --overwrite

echo ""
echo "8. Re-enable alert forwarding"
oc annotate multiclusterobservabilities.observability.open-cluster-management.io observability \
  mco-disable-alerting- --overwrite

echo ""
echo "9. Disable observability for a cluster"
oc label managedcluster spoke1 observability=disabled --overwrite

echo ""
echo "10. Re-enable observability for a cluster"
oc label managedcluster spoke1 observability- --overwrite

echo ""
echo "11. Discover Grafana URL"
oc get route -n "${NS}" grafana -o jsonpath='https://{.spec.host}'
echo ""

echo ""
echo "12. Verify resources"
oc get secret -n "${NS}" multiclusterhub-operator-pull-secret
oc get objectbucketclaim -n "${NS}" observability-obc
oc get configmap -n "${NS}" thanos-ruler-custom-rules
oc get configmap -n "${NS}" gpu-overview
oc get configmap -n "${NS}" observability-metrics-custom-allowlist
oc get secret -n "${NS}" alertmanager-config

echo ""
echo "13. Diagnose observability health"
acmlab observability diagnose

echo ""
echo "14. Diagnose and auto-repair"
acmlab observability diagnose --repair

echo ""
echo "15. Clean up"
oc delete secret -n "${NS}" alertmanager-config
oc delete configmap -n "${NS}" observability-metrics-custom-allowlist
oc delete configmap -n "${NS}" gpu-overview
oc delete configmap -n "${NS}" thanos-ruler-custom-rules
oc delete objectbucketclaim -n "${NS}" observability-obc

echo ""
echo "=== Done ==="
