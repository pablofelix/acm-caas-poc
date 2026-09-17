#!/usr/bin/env bash
# UC-20: AI platform operator version fleet segregation (manual)
# Shows raw ACM resources: ManagedCluster labels, ConfigurationPolicy, ManifestWork

set -euo pipefail

CLUSTER="gpu-h100-eugb"
VERSION="2.18"
POLICY_NS="open-cluster-management-policies"

echo "=== UC-20: Version Segregation (manual) ==="

echo "1. Label ManagedCluster with operator version"
oc label managedcluster "${CLUSTER}" \
  ai-platform-version="${VERSION}" \
  ai-platform-channel=stable \
  gpu-available=true \
  --overwrite

echo ""
echo "2. Create ConfigurationPolicy to enforce single version"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1
kind: ConfigurationPolicy
metadata:
  name: ${CLUSTER}-version-enforcement
  namespace: ${POLICY_NS}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/version-policy: "true"
spec:
  remediationAction: inform
  severity: high
  object-templates:
    - complianceType: musthave
      objectDefinition:
        apiVersion: operators.coreos.com/v1alpha1
        kind: Subscription
        metadata:
          namespace: openshift-operators
        spec:
          channel: "stable-${VERSION}"
EOF

echo ""
echo "3. Create ManifestWork to deploy operator at version"
cat <<EOF | oc apply -f -
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: ${CLUSTER}-ai-platform-operator
  namespace: ${CLUSTER}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/version: "${VERSION}"
spec:
  workload:
    manifests:
      - apiVersion: operators.coreos.com/v1
        kind: OperatorGroup
        metadata:
          name: ai-platform-og
          namespace: openshift-operators
        spec:
          targetNamespaces:
            - openshift-operators
      - apiVersion: operators.coreos.com/v1alpha1
        kind: Subscription
        metadata:
          name: ai-platform-operator
          namespace: openshift-operators
        spec:
          channel: "stable-${VERSION}"
          name: ai-platform-operator
          source: certified-operators
          sourceNamespace: openshift-marketplace
EOF

echo ""
echo "4. Create Placement to route by version"
cat <<EOF | oc apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: version-${VERSION}-placement
  namespace: ${POLICY_NS}
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            ai-platform-version: "${VERSION}"
            gpu-available: "true"
EOF

echo ""
echo "5. List all version-labelled clusters"
oc get managedcluster -l ai-platform-version -o custom-columns=\
NAME:.metadata.name,\
VERSION:.metadata.labels.ai-platform-version,\
CHANNEL:.metadata.labels.ai-platform-channel,\
AVAILABLE:.metadata.labels.gpu-available

echo ""
echo "=== Done ==="
