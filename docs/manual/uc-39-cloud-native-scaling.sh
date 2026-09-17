#!/usr/bin/env bash
# UC-39: Cloud-native scaling — NodePool and MachineDeployment patching
# Shows the fallback chain: MachinePool -> NodePool -> MachineDeployment

set -euo pipefail

CLUSTER_NAME="spoke1"
REPLICAS=4

echo "=== UC-39: Cloud-Native Scaling (manual) ==="

echo "1. Check for Hive MachinePool"
if oc get machinepool -n "$CLUSTER_NAME" -l hive.openshift.io/cluster-deployment-name="$CLUSTER_NAME" --no-headers 2>/dev/null | grep -q .; then
  POOL_NAME=$(oc get machinepool -n "$CLUSTER_NAME" -o jsonpath='{.items[0].metadata.name}')
  echo "   Found MachinePool: $POOL_NAME"
  echo "   Scaling MachinePool to $REPLICAS replicas"
  oc patch machinepool "$POOL_NAME" -n "$CLUSTER_NAME" --type merge \
    -p "{\"spec\":{\"replicas\":$REPLICAS}}"
  exit 0
fi

echo ""
echo "2. Check for HyperShift NodePool"
NAMESPACE="clusters-${CLUSTER_NAME}"
if oc get nodepool -n "$NAMESPACE" --no-headers 2>/dev/null | grep -q .; then
  NP_NAME=$(oc get nodepool -n "$NAMESPACE" -o jsonpath='{.items[0].metadata.name}')
  echo "   Found NodePool: $NP_NAME"
  echo "   Scaling NodePool to $REPLICAS replicas"
  oc patch nodepool "$NP_NAME" -n "$NAMESPACE" --type merge \
    -p "{\"spec\":{\"replicas\":$REPLICAS}}"
  exit 0
fi

echo ""
echo "3. Check for CAPI MachineDeployment"
if oc get machinedeployment.cluster.x-k8s.io -n "$CLUSTER_NAME" --no-headers 2>/dev/null | grep -q .; then
  MD_NAME=$(oc get machinedeployment.cluster.x-k8s.io -n "$CLUSTER_NAME" -o jsonpath='{.items[0].metadata.name}')
  echo "   Found MachineDeployment: $MD_NAME"
  echo "   Scaling MachineDeployment to $REPLICAS replicas"
  oc patch machinedeployment.cluster.x-k8s.io "$MD_NAME" -n "$CLUSTER_NAME" --type merge \
    -p "{\"spec\":{\"replicas\":$REPLICAS}}"
  exit 0
fi

echo "ERROR: No MachinePool, NodePool, or MachineDeployment found for $CLUSTER_NAME"
exit 1
