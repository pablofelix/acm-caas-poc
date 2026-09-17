#!/usr/bin/env bash
# UC-21: Elastic GPU capacity (manual)
# Shows raw ACM resources: ManagedCluster labels for elastic state, ManifestWork marker

set -euo pipefail

CLUSTER="gpu-ondemand-001"
GPU_TYPE="L4"

echo "=== UC-21: Elastic GPU Capacity (manual) ==="

echo "1. Check current GPU utilization label"
oc get managedcluster "${CLUSTER}" -o jsonpath='{.metadata.labels.gpu-utilization}' 2>/dev/null || echo "(no utilization label)"
echo ""

echo ""
echo "2. Label cluster as on-demand elastic"
oc label managedcluster "${CLUSTER}" \
  gpu-type="${GPU_TYPE}" \
  gpu-available=true \
  gpu-elastic=true \
  gpu-cost-tier=on-demand \
  --overwrite

echo ""
echo "3. Create ManifestWork for elastic stack marker"
cat <<EOF | oc apply -f -
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: ${CLUSTER}-elastic-gpu
  namespace: ${CLUSTER}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/elastic: "true"
    acmlab.redhat.com/gpu-type: "${GPU_TYPE}"
spec:
  workload:
    manifests:
      - apiVersion: v1
        kind: ConfigMap
        metadata:
          name: elastic-gpu-config
          namespace: default
        data:
          gpu-type: "${GPU_TYPE}"
          cost-tier: "on-demand"
          managed-by: "acmlab"
EOF

echo ""
echo "4. List all elastic GPU clusters"
oc get managedcluster -l gpu-elastic=true -o custom-columns=\
NAME:.metadata.name,\
GPU-TYPE:.metadata.labels.gpu-type,\
COST-TIER:.metadata.labels.gpu-cost-tier,\
AVAILABLE:.metadata.labels.gpu-available,\
STATE:.metadata.labels.gpu-elastic-state

echo ""
echo "5. Hibernate idle cluster (set labels)"
oc label managedcluster "${CLUSTER}" \
  gpu-available=false \
  gpu-elastic-state=hibernated \
  --overwrite

echo ""
echo "6. Verify hibernated state"
oc get managedcluster "${CLUSTER}" -o custom-columns=\
NAME:.metadata.name,\
AVAILABLE:.metadata.labels.gpu-available,\
STATE:.metadata.labels.gpu-elastic-state

echo ""
echo "=== Done ==="
