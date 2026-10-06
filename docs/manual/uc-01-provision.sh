#!/bin/bash
# UC-01: Cluster Provisioning via Hive ClusterDeployment
#
# Provisions an OpenShift cluster using Hive ClusterDeployment.
# Hive creates the cloud infrastructure, installs OpenShift, and registers
# the cluster with ACM as a ManagedCluster.
#
# Prerequisites:
#   - oc/kubectl logged into the ACM hub
#   - Pull secret file (from console.example.com)
#   - Cloud credentials (IBM Cloud API key, AWS keys, etc.)
#   - SSH key pair for node access
#
# Usage: This script is a reference — run commands one section at a time.

set -euo pipefail

CLUSTER_NAME="my-cluster"
REGION="us-south"
PLATFORM="ibmcloud"
BASE_DOMAIN="example.com"
IMAGE_SET="img4.22.9-multi-appsub"
WORKER_TYPE="bx2-4x16"
MASTER_TYPE="bx2-8x32"
PULL_SECRET_FILE="$HOME/pull-secret.json"
SSH_PUB_KEY_FILE="$HOME/.ssh/id_rsa.pub"
API_KEY="<your-cloud-api-key>"


# ═════════════════════════════════════════════════════════════════════
# PROVISION
# ═════════════════════════════════════════════════════════════════════

# ─────────────────────────────────────────────────────────────────────
# Step 0: Preflight checks
# ─────────────────────────────────────────────────────────────────────
# Validates credentials, cloud quota, image sets, and name conflicts
# before creating any resources. Catches problems in seconds instead
# of failing after 40 minutes of provisioning.

echo "=== Step 0: Preflight checks ==="

acmlab provision preflight "$CLUSTER_NAME" \
  --platform "$PLATFORM" \
  --pull-secret "$PULL_SECRET_FILE"

# ─────────────────────────────────────────────────────────────────────
# Step 1: Create namespace
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 1: Create namespace ==="

kubectl create namespace "$CLUSTER_NAME" --dry-run=client -o yaml | kubectl apply -f -

# ─────────────────────────────────────────────────────────────────────
# Step 2: Create pull secret
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 2: Create pull secret ==="

kubectl create secret generic "${CLUSTER_NAME}-pull-secret" \
  --namespace "$CLUSTER_NAME" \
  --from-file=.dockerconfigjson="$PULL_SECRET_FILE" \
  --type=kubernetes.io/dockerconfigjson \
  --dry-run=client -o yaml | kubectl apply -f -

# ─────────────────────────────────────────────────────────────────────
# Step 3: Create cloud credentials secret
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 3: Create cloud credentials ==="

kubectl create secret generic "${CLUSTER_NAME}-${PLATFORM}-creds" \
  --namespace "$CLUSTER_NAME" \
  --from-literal=ibmcloud_api_key="$API_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -

# ─────────────────────────────────────────────────────────────────────
# Step 4: Create install-config secret
# ─────────────────────────────────────────────────────────────────────
# The install-config defines cluster topology: platform, region,
# instance types, networking, and SSH key.

echo "=== Step 4: Create install-config ==="

SSH_KEY=$(cat "$SSH_PUB_KEY_FILE")

cat <<EOF > /tmp/install-config.yaml
apiVersion: v1
metadata:
  name: ${CLUSTER_NAME}
baseDomain: ${BASE_DOMAIN}
platform:
  ${PLATFORM}:
    region: ${REGION}
controlPlane:
  name: master
  replicas: 3
  platform:
    ${PLATFORM}:
      type: ${MASTER_TYPE}
compute:
- name: worker
  replicas: 2
  platform:
    ${PLATFORM}:
      type: ${WORKER_TYPE}
networking:
  networkType: OVNKubernetes
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
credentialsMode: Manual
pullSecret: ""
sshKey: ${SSH_KEY}
EOF

kubectl create secret generic "${CLUSTER_NAME}-install-config" \
  --namespace "$CLUSTER_NAME" \
  --from-file=install-config.yaml=/tmp/install-config.yaml \
  --dry-run=client -o yaml | kubectl apply -f -

# ─────────────────────────────────────────────────────────────────────
# Step 5: Create ClusterDeployment
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 5: Create ClusterDeployment ==="

cat <<EOF | kubectl apply -f -
apiVersion: hive.openshift.io/v1
kind: ClusterDeployment
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
  labels:
    acmlab.redhat.com/managed: "true"
spec:
  clusterName: ${CLUSTER_NAME}
  baseDomain: ${BASE_DOMAIN}
  platform:
    ${PLATFORM}:
      region: ${REGION}
      credentialsSecretRef:
        name: ${CLUSTER_NAME}-${PLATFORM}-creds
  provisioning:
    imageSetRef:
      name: ${IMAGE_SET}
    installConfigSecretRef:
      name: ${CLUSTER_NAME}-install-config
  pullSecretRef:
    name: ${CLUSTER_NAME}-pull-secret
EOF

# ─────────────────────────────────────────────────────────────────────
# Step 6: Create ManagedCluster (so ACM adopts the cluster)
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 6: Create ManagedCluster ==="

cat <<EOF | kubectl apply -f -
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: ${CLUSTER_NAME}
  labels:
    cloud: IBM
    vendor: OpenShift
    cluster.open-cluster-management.io/clusterset: default
    acmlab.redhat.com/managed: "true"
spec:
  hubAcceptsClient: true
EOF

# ─────────────────────────────────────────────────────────────────────
# Step 7: Create KlusterletAddonConfig
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 7: Create KlusterletAddonConfig ==="

cat <<EOF | kubectl apply -f -
apiVersion: agent.open-cluster-management.io/v1
kind: KlusterletAddonConfig
metadata:
  name: ${CLUSTER_NAME}
  namespace: ${CLUSTER_NAME}
spec:
  applicationManager:
    enabled: true
  certPolicyController:
    enabled: true
  policyController:
    enabled: true
  searchCollector:
    enabled: true
EOF


# ═════════════════════════════════════════════════════════════════════
# MONITOR PROVISIONING
# ═════════════════════════════════════════════════════════════════════
# Provisioning takes ~30-40 minutes.

# ─────────────────────────────────────────────────────────────────────
# Check provisioning status
# ─────────────────────────────────────────────────────────────────────

echo "=== Monitor provisioning ==="

echo "Installed timestamp (empty = still provisioning):"
kubectl get clusterdeployment -n "$CLUSTER_NAME" "$CLUSTER_NAME" \
  -o jsonpath='{.status.installedTimestamp}'
echo ""

echo "Conditions:"
kubectl get clusterdeployment -n "$CLUSTER_NAME" "$CLUSTER_NAME" \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} — {.message}{"\n"}{end}'

echo ""
echo "ManagedCluster status:"
kubectl get managedcluster "$CLUSTER_NAME" \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status}{"\n"}{end}'


# ═════════════════════════════════════════════════════════════════════
# DESTROY
# ═════════════════════════════════════════════════════════════════════
# Deleting the ClusterDeployment triggers Hive to deprovision
# all cloud infrastructure. The ManagedCluster is cleaned up
# by ACM's cleanup controllers.

# kubectl delete clusterdeployment ${CLUSTER_NAME} -n ${CLUSTER_NAME}
# kubectl delete managedcluster ${CLUSTER_NAME}
# kubectl delete namespace ${CLUSTER_NAME}


# ═════════════════════════════════════════════════════════════════════
# Alternative: Using the acmlab CLI
# ═════════════════════════════════════════════════════════════════════
#
# acmlab provision create my-cluster --pull-secret ~/pull-secret.json --region us-south
# acmlab provision status my-cluster
# acmlab provision list
# acmlab provision destroy my-cluster
