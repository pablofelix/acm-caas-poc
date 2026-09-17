#!/usr/bin/env bash
# UC-41: CAPI cluster hibernate via scale-to-zero — raw oc/kubectl commands
# Stores pre-hibernate replicas in annotation for resume

set -euo pipefail

CLUSTER_NAME="capi-test"
NAMESPACE="$CLUSTER_NAME"
ANNOTATION_KEY="acmlab.redhat.com/pre-hibernate-replicas"

echo "=== UC-41: CAPI Hibernate (manual) ==="

echo "1. Get current MachineDeployment replicas"
MD_NAME=$(oc get machinedeployment.cluster.x-k8s.io -n "$NAMESPACE" \
  -l cluster.x-k8s.io/cluster-name="$CLUSTER_NAME" \
  -o jsonpath='{.items[0].metadata.name}')
CURRENT_REPLICAS=$(oc get machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$NAMESPACE" \
  -o jsonpath='{.spec.replicas}')
echo "   MachineDeployment: $MD_NAME, Replicas: $CURRENT_REPLICAS"

echo ""
echo "2. Store current replicas in annotation before hibernating"
oc annotate machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$NAMESPACE" \
  "${ANNOTATION_KEY}=${CURRENT_REPLICAS}" --overwrite

echo ""
echo "3. Scale to zero (hibernate)"
oc patch machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$NAMESPACE" \
  --type merge -p '{"spec":{"replicas":0}}'
echo "   Cluster hibernated — all worker nodes will be drained and removed"

echo ""
echo "--- To resume ---"
echo ""
echo "4. Read pre-hibernate replicas from annotation"
STORED_REPLICAS=$(oc get machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$NAMESPACE" \
  -o jsonpath="{.metadata.annotations.acmlab\.redhat\.com/pre-hibernate-replicas}")
RESTORE_REPLICAS="${STORED_REPLICAS:-2}"
echo "   Restoring to $RESTORE_REPLICAS replicas"

echo ""
echo "5. Scale back up (resume)"
oc patch machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$NAMESPACE" \
  --type merge -p "{\"spec\":{\"replicas\":$RESTORE_REPLICAS}}"
echo "   Cluster resumed — new worker nodes will be provisioned"

echo ""
echo "6. Verify MachineDeployment status after resume"
oc get machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$NAMESPACE" \
  -o custom-columns=NAME:.metadata.name,REPLICAS:.spec.replicas,READY:.status.readyReplicas,ANNOTATION:.metadata.annotations
