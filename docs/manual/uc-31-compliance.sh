#!/usr/bin/env bash
# UC-31: SCAP scanning via Compliance Operator — raw oc/kubectl commands
# Requires: Compliance Operator available in OperatorHub

set -euo pipefail

CLUSTER_NAME="spoke1"
CLUSTER_SET="default"
NAMESPACE="open-cluster-management-policies"

echo "=== UC-31: Compliance Operator (manual) ==="

echo "1. Create OperatorPolicy to deploy Compliance Operator"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1beta1
kind: OperatorPolicy
metadata:
  name: compliance-operator-${CLUSTER_NAME}
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/compliance: "true"
spec:
  remediationAction: enforce
  severity: high
  complianceType: musthave
  subscription:
    channel: stable
    name: compliance-operator
    namespace: openshift-compliance
    source: redhat-operators
    sourceNamespace: openshift-marketplace
EOF

echo ""
echo "2. Create health monitoring ConfigurationPolicy"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: compliance-health-${CLUSTER_NAME}
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
spec:
  remediationAction: inform
  disabled: false
  policy-templates:
    - objectDefinition:
        apiVersion: policy.open-cluster-management.io/v1
        kind: ConfigurationPolicy
        metadata:
          name: compliance-operator-health
        spec:
          remediationAction: inform
          severity: high
          object-templates:
            - complianceType: musthave
              objectDefinition:
                apiVersion: apps/v1
                kind: Deployment
                metadata:
                  name: compliance-operator
                  namespace: openshift-compliance
                status:
                  conditions:
                    - type: Available
                      status: "True"
EOF

echo ""
echo "3. Create Placement targeting the cluster"
cat <<EOF | oc apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: compliance-${CLUSTER_NAME}-placement
  namespace: ${NAMESPACE}
spec:
  clusterSets:
    - ${CLUSTER_SET}
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            name: ${CLUSTER_NAME}
EOF

echo ""
echo "4. Create PlacementBinding"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1
kind: PlacementBinding
metadata:
  name: compliance-${CLUSTER_NAME}-binding
  namespace: ${NAMESPACE}
spec:
  placementRef:
    apiGroup: cluster.open-cluster-management.io
    kind: Placement
    name: compliance-${CLUSTER_NAME}-placement
  subjects:
    - apiGroup: policy.open-cluster-management.io
      kind: Policy
      name: compliance-health-${CLUSTER_NAME}
EOF

echo ""
echo "5. Check compliance operator deployment on spoke"
oc get policy -n "$NAMESPACE" -l acmlab.redhat.com/compliance=true \
  -o custom-columns=NAME:.metadata.name,COMPLIANT:.status.compliant

echo ""
echo "6. Check ComplianceScan results on spoke (requires spoke kubeconfig)"
echo "   oc get compliancescan -n openshift-compliance"
echo "   oc get compliancecheckresult -n openshift-compliance"
