#!/usr/bin/env bash
# UC-19: Multi-cluster GPU workload routing (manual)
# Shows raw ACM resources: Placement with GPU predicates, PlacementDecision

set -euo pipefail

NAMESPACE="open-cluster-management-policies"
PLACEMENT_NAME="gpu-route-h100"
GPU_TYPE="H100"

echo "=== UC-19: GPU Workload Routing (manual) ==="

echo "1. Label ManagedClusters with GPU metadata"
oc label managedcluster gpu-h100-eugb gpu-type=H100 gpu-available=true region=eu-gb --overwrite
oc label managedcluster gpu-l4-ussouth gpu-type=L4 gpu-available=true region=us-south --overwrite

echo ""
echo "2. Create Placement with GPU type and availability predicates"
cat <<EOF | oc apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: ${PLACEMENT_NAME}
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/gpu-placement: "true"
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            gpu-type: "${GPU_TYPE}"
            gpu-available: "true"
EOF

echo ""
echo "3. Wait for PlacementDecision"
sleep 3
oc get placementdecision -n "${NAMESPACE}" -l "cluster.open-cluster-management.io/placement=${PLACEMENT_NAME}"

echo ""
echo "4. Read selected cluster from PlacementDecision"
oc get placementdecision -n "${NAMESPACE}" \
  -l "cluster.open-cluster-management.io/placement=${PLACEMENT_NAME}" \
  -o jsonpath='{.items[0].status.decisions[0].clusterName}'
echo ""

echo ""
echo "5. Mark a cluster as saturated"
oc label managedcluster gpu-h100-eugb gpu-available=false --overwrite

echo ""
echo "6. Verify routing excludes saturated cluster"
sleep 3
oc get placementdecision -n "${NAMESPACE}" \
  -l "cluster.open-cluster-management.io/placement=${PLACEMENT_NAME}" \
  -o jsonpath='{.items[0].status.decisions[*].clusterName}'
echo ""

echo ""
echo "7. Clear saturation"
oc label managedcluster gpu-h100-eugb gpu-available=true --overwrite

echo ""
echo "8. Clean up"
oc delete placement "${PLACEMENT_NAME}" -n "${NAMESPACE}"

echo ""
echo "=== Done ==="
