#!/usr/bin/env bash
# UC-38: HyperShift (HostedCluster) provisioning — raw oc/kubectl commands
# Requires: HyperShift operator installed on the hub, pull secret, cloud credentials

set -euo pipefail

CLUSTER_NAME="hosted-test"
NAMESPACE="clusters-${CLUSTER_NAME}"
RELEASE_IMAGE="quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64"

echo "=== UC-38: HyperShift Provisioning (manual) ==="

echo "1. Create namespace for the hosted cluster"
oc create namespace "$NAMESPACE" --dry-run=client -o yaml | oc apply -f -

echo ""
echo "2. Create pull secret in the namespace"
oc create secret generic pull-secret \
  --from-file=.dockerconfigjson="$HOME/pull-secret.json" \
  --type=kubernetes.io/dockerconfigjson \
  -n "$NAMESPACE" --dry-run=client -o yaml | oc apply -f -

echo ""
echo "3. Create HostedCluster CR"
cat <<EOF | oc apply -f -
apiVersion: hypershift.openshift.io/v1beta1
kind: HostedCluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
spec:
  release:
    image: ${RELEASE_IMAGE}
  pullSecret:
    name: pull-secret
  platform:
    type: AWS
    aws:
      region: us-east-1
  networking:
    clusterNetwork:
      - cidr: 10.132.0.0/14
    serviceNetwork:
      - cidr: 172.31.0.0/16
  services:
    - service: APIServer
      servicePublishingStrategy:
        type: LoadBalancer
    - service: OAuthServer
      servicePublishingStrategy:
        type: Route
EOF

echo ""
echo "4. Create NodePool"
cat <<EOF | oc apply -f -
apiVersion: hypershift.openshift.io/v1beta1
kind: NodePool
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${NAMESPACE}
  labels:
    acmlab.redhat.com/managed: "true"
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: 2
  release:
    image: ${RELEASE_IMAGE}
  platform:
    type: AWS
    aws:
      instanceType: m5.xlarge
  management:
    autoRepair: true
    upgradeType: Replace
EOF

echo ""
echo "5. Register as ManagedCluster on the hub"
cat <<EOF | oc apply -f -
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
    cloud: Amazon
    vendor: OpenShift
spec:
  hubAcceptsClient: true
EOF

echo ""
echo "6. Check HostedCluster status"
oc get hostedcluster -n "$NAMESPACE" "$CLUSTER_NAME" -o custom-columns=NAME:.metadata.name,AVAILABLE:.status.conditions[0].status,VERSION:.status.version.history[0].version

echo ""
echo "7. Check NodePool status"
oc get nodepool -n "$NAMESPACE" "$CLUSTER_NAME" -o custom-columns=NAME:.metadata.name,REPLICAS:.spec.replicas,READY:.status.replicas
