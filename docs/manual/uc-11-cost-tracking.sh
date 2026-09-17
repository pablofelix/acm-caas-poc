#!/usr/bin/env bash
# UC-11: Cost tracking — manual oc/kubectl reference
# Reads ManagedClusterInfo node metadata to estimate costs (ADR-009)

set -euo pipefail

CLUSTER="spoke-test"

echo "=== UC-11: Cost Tracking (manual) ==="

echo "1. Read ManagedClusterInfo to get node metadata"
oc get managedclusterinfo "${CLUSTER}" \
  -n "${CLUSTER}" \
  -o jsonpath='{.status.nodeList[*]}' | python3 -m json.tool

echo ""
echo "2. Extract node instance types and counts"
oc get managedclusterinfo "${CLUSTER}" \
  -n "${CLUSTER}" \
  -o jsonpath='{range .status.nodeList[*]}{.name}{"\t"}{.labels.node\.kubernetes\.io/instance-type}{"\t"}{.capacity.cpu}{"\t"}{.capacity.memory}{"\n"}{end}'

echo ""
echo "3. Check ManagedCluster labels for cost center"
oc get managedcluster "${CLUSTER}" \
  -o jsonpath='{.metadata.labels.caas/cost-center}'

echo ""
echo "4. Stamp cost center label on ManagedCluster"
oc label managedcluster "${CLUSTER}" \
  "caas/cost-center=engineering" --overwrite

echo ""
echo "5. List all clusters with cost center labels"
oc get managedcluster \
  -l 'caas/cost-center' \
  -o custom-columns='NAME:.metadata.name,COST-CENTER:.metadata.labels.caas/cost-center'

echo ""
echo "6. Manual cost calculation example"
echo "   Instance type: bx2-4x16 (4 CPU, 16 GiB)"
echo "   Hourly rate:   \$0.192/hr (placeholder)"
echo "   Node count:    3"
echo "   Monthly est:   3 * \$0.192 * 730 = \$420.48"

echo ""
echo "=== Done ==="
