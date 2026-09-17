#!/usr/bin/env bash
# UC-18: GPU sharing stack deployment (manual)
# Shows raw ACM resources: ManifestWork for Kueue/Kyverno, ConfigurationPolicy for health

set -euo pipefail

CLUSTER="gpu-h100-eugb"
NAMESPACE="${CLUSTER}"
POLICY_NS="open-cluster-management-policies"

echo "=== UC-18: GPU Sharing Stack (manual) ==="

echo "1. Label the ManagedCluster for GPU sharing"
oc label managedcluster "${CLUSTER}" gpu-sharing=enabled --overwrite

echo ""
echo "2. Create ManifestWork for Kueue stack"
cat <<EOF | oc apply -f -
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: ${CLUSTER}-kueue-stack
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/gpu-stack: "kueue"
spec:
  workload:
    manifests:
      - apiVersion: v1
        kind: Namespace
        metadata:
          name: kueue-system
      - apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: kueue-controller-manager
          namespace: kueue-system
        spec:
          replicas: 1
          selector:
            matchLabels:
              control-plane: controller-manager
          template:
            metadata:
              labels:
                control-plane: controller-manager
            spec:
              containers:
                - name: manager
                  image: registry.k8s.io/kueue/kueue:v0.6.0
EOF

echo ""
echo "3. Create ManifestWork for Kyverno GPU policies"
cat <<EOF | oc apply -f -
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: ${CLUSTER}-kyverno-gpu
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/gpu-stack: "kyverno"
spec:
  workload:
    manifests:
      - apiVersion: v1
        kind: Namespace
        metadata:
          name: kyverno
      - apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: kyverno-admission-controller
          namespace: kyverno
        spec:
          replicas: 1
          selector:
            matchLabels:
              app.kubernetes.io/name: kyverno
          template:
            metadata:
              labels:
                app.kubernetes.io/name: kyverno
            spec:
              containers:
                - name: kyverno
                  image: ghcr.io/kyverno/kyverno:v1.12.0
EOF

echo ""
echo "4. Create ConfigurationPolicy to monitor stack health"
cat <<EOF | oc apply -f -
apiVersion: policy.open-cluster-management.io/v1
kind: ConfigurationPolicy
metadata:
  name: ${CLUSTER}-gpu-stack-health
  namespace: ${POLICY_NS}
spec:
  remediationAction: inform
  severity: high
  object-templates:
    - complianceType: musthave
      objectDefinition:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: kueue-controller-manager
          namespace: kueue-system
        status:
          conditions:
            - type: Available
              status: "True"
    - complianceType: musthave
      objectDefinition:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: kyverno-admission-controller
          namespace: kyverno
        status:
          conditions:
            - type: Available
              status: "True"
EOF

echo ""
echo "5. Verify ManifestWorks"
oc get manifestwork -n "${NAMESPACE}" -l acmlab.redhat.com/managed=true

echo ""
echo "6. Check ConfigurationPolicy compliance"
oc get configurationpolicy -n "${POLICY_NS}" "${CLUSTER}-gpu-stack-health" -o jsonpath='{.status.compliant}'
echo ""

echo ""
echo "=== Done ==="
