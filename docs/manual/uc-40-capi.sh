#!/usr/bin/env bash
# UC-40: CAPI provisioning for vanilla Kubernetes — raw oc/kubectl commands
# Requires: CAPI controllers installed, infrastructure provider configured

set -euo pipefail

CLUSTER_NAME="capi-test"
K8S_VERSION="v1.30.0"
INFRA_PROVIDER="docker"

echo "=== UC-40: CAPI Provisioning (manual) ==="

echo "1. Create namespace for the CAPI cluster"
oc create namespace "$CLUSTER_NAME" --dry-run=client -o yaml | oc apply -f -

echo ""
echo "2. Create CAPI Cluster CR"
cat <<EOF | oc apply -f -
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/provisioner: capi
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
        - 192.168.0.0/16
    services:
      cidrBlocks:
        - 10.96.0.0/12
  topology:
    class: ${INFRA_PROVIDER}
    version: ${K8S_VERSION}
EOF

echo ""
echo "3. Create MachineDeployment for worker nodes"
cat <<EOF | oc apply -f -
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: ${CLUSTER_NAME}-workers
  namespace: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
    cluster.x-k8s.io/cluster-name: ${CLUSTER_NAME}
spec:
  clusterName: ${CLUSTER_NAME}
  replicas: 2
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: ${CLUSTER_NAME}
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: ${CLUSTER_NAME}
    spec:
      clusterName: ${CLUSTER_NAME}
      version: ${K8S_VERSION}
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-workers
EOF

echo ""
echo "4. Register as ManagedCluster on ACM hub"
cat <<EOF | oc apply -f -
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/provisioner: capi
    vendor: Kubernetes
spec:
  hubAcceptsClient: true
EOF

echo ""
echo "5. Check CAPI Cluster status"
oc get cluster.cluster.x-k8s.io -n "$CLUSTER_NAME" "$CLUSTER_NAME" \
  -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,READY:.status.conditions[0].status

echo ""
echo "6. Check MachineDeployment status"
oc get machinedeployment.cluster.x-k8s.io -n "$CLUSTER_NAME" \
  -o custom-columns=NAME:.metadata.name,REPLICAS:.spec.replicas,READY:.status.readyReplicas,PHASE:.status.phase
