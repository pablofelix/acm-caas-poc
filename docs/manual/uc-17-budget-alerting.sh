#!/usr/bin/env bash
# UC-17: Budget alerting — manual oc/kubectl reference
# Cost center labels + ACM Policy for budget enforcement

set -euo pipefail

NAMESPACE="open-cluster-management-policies"
COST_CENTER="platform"
BUDGET_LIMIT="5000"

echo "=== UC-17: Budget Alerting (manual) ==="

echo "1. Stamp cost center label on clusters"
oc label managedcluster spoke1 "caas/cost-center=${COST_CENTER}" --overwrite
oc label managedcluster spoke2 "caas/cost-center=${COST_CENTER}" --overwrite

echo ""
echo "2. List clusters by cost center"
oc get managedcluster \
  -l "caas/cost-center=${COST_CENTER}" \
  -o custom-columns='NAME:.metadata.name,COST-CENTER:.metadata.labels.caas/cost-center'

echo ""
echo "3. Create budget alerting Policy (inform-only)"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: budget-${COST_CENTER}
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/budget: "true"
  annotations:
    acmlab.redhat.com/cost-center: "${COST_CENTER}"
    acmlab.redhat.com/budget-limit: "${BUDGET_LIMIT}"
spec:
  remediationAction: inform
  disabled: false
  policy-templates:
    - objectDefinition:
        apiVersion: policy.open-cluster-management.io/v1
        kind: ConfigurationPolicy
        metadata:
          name: budget-${COST_CENTER}-check
        spec:
          remediationAction: inform
          severity: high
          object-templates:
            - complianceType: musthave
              objectDefinition:
                apiVersion: v1
                kind: ConfigMap
                metadata:
                  name: budget-status
                  namespace: default
                data:
                  costCenter: "${COST_CENTER}"
                  budgetLimit: "${BUDGET_LIMIT}"
EOF

echo ""
echo "4. Create Placement targeting cost center clusters"
cat <<EOF | oc apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: budget-${COST_CENTER}-placement
  namespace: ${NAMESPACE}
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            caas/cost-center: "${COST_CENTER}"
EOF

echo ""
echo "5. Create PlacementBinding"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  name: budget-${COST_CENTER}-binding
  namespace: ${NAMESPACE}
placementRef:
  name: budget-${COST_CENTER}-placement
  apiGroup: cluster.open-cluster-management.io
  kind: Placement
subjects:
  - name: budget-${COST_CENTER}
    apiGroup: policy.open-cluster-management.io
    kind: Policy
EOF

echo ""
echo "6. Check policy compliance status"
oc get policy "budget-${COST_CENTER}" \
  -n "${NAMESPACE}" \
  -o jsonpath='{.status.compliant}'

echo ""
echo "7. Clean up budget policy"
oc delete placementbinding "budget-${COST_CENTER}-binding" -n "${NAMESPACE}" --ignore-not-found
oc delete placement "budget-${COST_CENTER}-placement" -n "${NAMESPACE}" --ignore-not-found
oc delete policy "budget-${COST_CENTER}" -n "${NAMESPACE}" --ignore-not-found

echo ""
echo "=== Done ==="
