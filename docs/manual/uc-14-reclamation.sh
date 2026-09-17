#!/usr/bin/env bash
# UC-14: Cluster reclamation — manual oc/kubectl reference
# TTL labels: caas/ttl-hours, caas/expiry-date, caas/ttl-owner

set -euo pipefail

CLUSTER="spoke-test"
TTL_HOURS="72"
OWNER="team-alpha"

echo "=== UC-14: Cluster Reclamation (manual) ==="

echo "1. Set TTL labels on ManagedCluster"
EXPIRY_DATE=$(date -u -d "+${TTL_HOURS} hours" +%Y-%m-%dT%H:%M:%SZ)
oc label managedcluster "${CLUSTER}" \
  "caas/ttl-hours=${TTL_HOURS}" \
  "caas/expiry-date=${EXPIRY_DATE}" \
  "caas/ttl-owner=${OWNER}" \
  --overwrite

echo ""
echo "2. Verify TTL labels"
oc get managedcluster "${CLUSTER}" \
  -o jsonpath='{.metadata.labels.caas/ttl-hours}{"\t"}{.metadata.labels.caas/expiry-date}{"\t"}{.metadata.labels.caas/ttl-owner}{"\n"}'

echo ""
echo "3. List all clusters with TTL labels"
oc get managedcluster \
  -l 'caas/ttl-hours' \
  -o custom-columns='NAME:.metadata.name,TTL:.metadata.labels.caas/ttl-hours,EXPIRY:.metadata.labels.caas/expiry-date,OWNER:.metadata.labels.caas/ttl-owner'

echo ""
echo "4. Find expired clusters (compare expiry-date with now)"
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
echo "Current time: ${NOW}"
echo "Clusters with expiry before now are candidates for reclamation."
oc get managedcluster \
  -l 'caas/expiry-date' \
  -o custom-columns='NAME:.metadata.name,EXPIRY:.metadata.labels.caas/expiry-date'

echo ""
echo "5. Extend TTL (add 48 hours to expiry)"
NEW_EXPIRY=$(date -u -d "+48 hours" +%Y-%m-%dT%H:%M:%SZ)
oc label managedcluster "${CLUSTER}" \
  "caas/expiry-date=${NEW_EXPIRY}" \
  "caas/ttl-extend-justification=sprint-extension" \
  --overwrite

echo ""
echo "6. Reclaim a Hive-provisioned cluster (hibernate)"
oc patch clusterdeployment "${CLUSTER}" \
  -n "${CLUSTER}" \
  --type merge \
  -p '{"spec":{"powerState":"Hibernating"}}'

echo ""
echo "7. Reclaim an imported cluster (detach)"
oc delete managedcluster "${CLUSTER}" --wait=false

echo ""
echo "=== Done ==="
