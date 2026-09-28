#!/usr/bin/env bash
# UC-40: CAPI provisioning for vanilla Kubernetes on AWS — raw kubectl commands
# Requires: CAPI controllers installed on hub, AWS credentials configured,
#           SSH key pair created in target region

set -euo pipefail

CLUSTER_NAME="capi-test"
K8S_VERSION="v1.34.8"
REGION="us-east-1"
INSTANCE_TYPE="t3.large"
SSH_KEY_NAME="acmlab-capi"

echo "=== UC-40: CAPI Provisioning (manual) ==="

echo "1. Install CAPI controllers via clusterctl"
clusterctl init --bootstrap kubeadm --control-plane kubeadm --infrastructure aws

echo ""
echo "2. Grant privileged SCC to CAPI service accounts (OpenShift only)"
for SA in \
  system:serviceaccount:capi-system:capi-manager \
  system:serviceaccount:capi-kubeadm-bootstrap-system:capi-kubeadm-bootstrap-manager \
  system:serviceaccount:capi-kubeadm-control-plane-system:capi-kubeadm-control-plane-manager \
  system:serviceaccount:capa-system:capa-controller-manager; do
  oc adm policy add-scc-to-user privileged "$SA"
done

echo ""
echo "3. Upgrade CAPI CRDs and webhooks to v1beta2 (if upgrading from v1.10.x)"
kubectl apply --server-side --force-conflicts \
  -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.14.2/cluster-api-components.yaml

echo ""
echo "4. Patch controller deployments to fix env var syntax on OpenShift"
# v1.14.2 uses '${VAR:=default}' syntax that OpenShift doesn't resolve
for DEPLOY in \
  capi-controller-manager:capi-system \
  capi-kubeadm-bootstrap-controller-manager:capi-kubeadm-bootstrap-system \
  capi-kubeadm-control-plane-controller-manager:capi-kubeadm-control-plane-system; do
  NAME="${DEPLOY%%:*}"
  NS="${DEPLOY##*:}"
  kubectl rollout restart "deployment/$NAME" -n "$NS"
done

echo ""
echo "5. Create namespace for the CAPI cluster"
kubectl create namespace "$CLUSTER_NAME" --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "6. Create IAM instance profiles via clusterawsadm (one-time setup)"
clusterawsadm bootstrap iam create-cloudformation-stack --region "$REGION"

echo ""
echo "7. Create AWS SSH key pair (if not exists)"
aws ec2 describe-key-pairs --key-names "$SSH_KEY_NAME" --region "$REGION" 2>/dev/null || \
  aws ec2 create-key-pair --key-name "$SSH_KEY_NAME" --region "$REGION" --query KeyPairId --output text

echo ""
echo "7. Create AWSCluster"
cat <<EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSCluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
spec:
  region: ${REGION}
  sshKeyName: ${SSH_KEY_NAME}
EOF

echo ""
echo "8. Create AWSMachineTemplate for control plane"
cat <<EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-control-plane
  namespace: ${CLUSTER_NAME}
spec:
  template:
    spec:
      iamInstanceProfile: control-plane.cluster-api-provider-aws.sigs.k8s.io
      instanceType: ${INSTANCE_TYPE}
      sshKeyName: ${SSH_KEY_NAME}
      rootVolume:
        size: 80
EOF

echo ""
echo "9. Create KubeadmControlPlane"
cat <<EOF | kubectl apply -f -
apiVersion: controlplane.cluster.x-k8s.io/v1beta2
kind: KubeadmControlPlane
metadata:
  name: ${CLUSTER_NAME}-control-plane
  namespace: ${CLUSTER_NAME}
spec:
  replicas: 1
  version: ${K8S_VERSION}
  machineTemplate:
    spec:
      infrastructureRef:
        apiGroup: infrastructure.cluster.x-k8s.io
        kind: AWSMachineTemplate
        name: ${CLUSTER_NAME}-control-plane
  kubeadmConfigSpec:
    initConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          - name: cloud-provider
            value: external
    joinConfiguration:
      nodeRegistration:
        kubeletExtraArgs:
          - name: cloud-provider
            value: external
    clusterConfiguration:
      apiServer:
        extraArgs:
          - name: cloud-provider
            value: external
      controllerManager:
        extraArgs:
          - name: cloud-provider
            value: external
EOF

echo ""
echo "10. Create CAPI Cluster (references AWSCluster + KubeadmControlPlane)"
cat <<EOF | kubectl apply -f -
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/provisioner: capi
    acmlab.redhat.com/infra-provider: aws
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
        - 192.168.0.0/16
    services:
      cidrBlocks:
        - 10.128.0.0/12
  infrastructureRef:
    apiGroup: infrastructure.cluster.x-k8s.io
    kind: AWSCluster
    name: ${CLUSTER_NAME}
  controlPlaneRef:
    apiGroup: controlplane.cluster.x-k8s.io
    kind: KubeadmControlPlane
    name: ${CLUSTER_NAME}-control-plane
EOF

echo ""
echo "11. Create AWSMachineTemplate for workers"
cat <<EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind: AWSMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-workers
  namespace: ${CLUSTER_NAME}
spec:
  template:
    spec:
      instanceType: ${INSTANCE_TYPE}
      iamInstanceProfile: nodes.cluster-api-provider-aws.sigs.k8s.io
      sshKeyName: ${SSH_KEY_NAME}
      rootVolume:
        size: 80
EOF

echo ""
echo "12. Create KubeadmConfigTemplate for workers"
cat <<EOF | kubectl apply -f -
apiVersion: bootstrap.cluster.x-k8s.io/v1beta2
kind: KubeadmConfigTemplate
metadata:
  name: ${CLUSTER_NAME}-workers
  namespace: ${CLUSTER_NAME}
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs:
            - name: cloud-provider
              value: external
EOF

echo ""
echo "13. Create MachineDeployment for worker nodes"
cat <<EOF | kubectl apply -f -
apiVersion: cluster.x-k8s.io/v1beta2
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
          apiGroup: bootstrap.cluster.x-k8s.io
          kind: KubeadmConfigTemplate
          name: ${CLUSTER_NAME}-workers
      infrastructureRef:
        apiGroup: infrastructure.cluster.x-k8s.io
        kind: AWSMachineTemplate
        name: ${CLUSTER_NAME}-workers
EOF

echo ""
echo "14. Register as ManagedCluster on ACM hub"
cat <<EOF | kubectl apply -f -
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
    acmlab.redhat.com/provisioner: capi
    vendor: Kubernetes
    cluster.open-cluster-management.io/clusterset: default
spec:
  hubAcceptsClient: true
EOF

echo ""
echo "15. Check CAPI Cluster status"
kubectl get cluster.cluster.x-k8s.io,awscluster,kubeadmcontrolplane,machines,awsmachine \
  -n "$CLUSTER_NAME"

echo ""
echo "16. Check MachineDeployment status"
kubectl get machinedeployment.cluster.x-k8s.io -n "$CLUSTER_NAME"
