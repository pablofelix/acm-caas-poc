#!/bin/bash
# UC-25: ClusterPool and ClusterClaim — Pre-warmed Clusters
#
# Creates a Hive ClusterPool with pre-provisioned hibernated clusters.
# Developers claim clusters instantly via ClusterClaim instead of
# waiting 30+ minutes for provisioning.
#
# Prerequisites:
#   - oc/kubectl logged into the ACM hub
#   - Hive installed on the hub
#   - Cloud credentials configured
#
# Usage: This script is a reference — run commands one section at a time.

set -euo pipefail

POOL_NAME="caas-pool"
POOL_NS="${POOL_NAME}"
POOL_SIZE=2
IMAGE_SET="img4.22.9-multi-appsub"
BASE_DOMAIN="example.com"
REGION="us-south"
PLATFORM="ibmcloud"
PULL_SECRET_FILE="$HOME/pull-secret.json"
API_KEY="<your-cloud-api-key>"


# ═════════════════════════════════════════════════════════════════════
# CREATE POOL
# ═════════════════════════════════════════════════════════════════════

# ─────────────────────────────────────────────────────────────────────
# Step 1: Create namespace
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 1: Create namespace ==="

kubectl create namespace "$POOL_NS" --dry-run=client -o yaml | kubectl apply -f -

# ─────────────────────────────────────────────────────────────────────
# Step 2: Create pull secret and cloud credentials
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 2: Create secrets ==="

kubectl create secret generic "${POOL_NS}-pull-secret" \
  --namespace "$POOL_NS" \
  --from-file=.dockerconfigjson="$PULL_SECRET_FILE" \
  --type=kubernetes.io/dockerconfigjson \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic "${POOL_NS}-${PLATFORM}-creds" \
  --namespace "$POOL_NS" \
  --from-literal=ibmcloud_api_key="$API_KEY" \
  --dry-run=client -o yaml | kubectl apply -f -

# ─────────────────────────────────────────────────────────────────────
# Step 3: Create ClusterPool
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 3: Create ClusterPool ==="

cat <<EOF | kubectl apply -f -
apiVersion: hive.openshift.io/v1
kind: ClusterPool
metadata:
  name: ${POOL_NAME}
  namespace: ${POOL_NS}
spec:
  size: ${POOL_SIZE}
  baseDomain: ${BASE_DOMAIN}
  installAttemptsLimit: 6
  imageSetRef:
    name: ${IMAGE_SET}
  pullSecretRef:
    name: ${POOL_NS}-pull-secret
  platform:
    ${PLATFORM}:
      region: ${REGION}
      credentialsSecretRef:
        name: ${POOL_NS}-${PLATFORM}-creds
  hibernationConfig:
    resumeTimeout: 20m
EOF

# ─────────────────────────────────────────────────────────────────────
# Step 4: Monitor pool readiness
# ─────────────────────────────────────────────────────────────────────
echo "=== Step 4: Pool status ==="

kubectl get clusterpool "$POOL_NAME" -n "$POOL_NS" \
  -o jsonpath='Size: {.spec.size}, Ready: {.status.ready}, Standby: {.status.standby}'
echo ""


# ═════════════════════════════════════════════════════════════════════
# CLAIM A CLUSTER
# ═════════════════════════════════════════════════════════════════════

echo "=== Claim a cluster ==="

CLAIM_NAME="my-test"

cat <<EOF | kubectl apply -f -
apiVersion: hive.openshift.io/v1
kind: ClusterClaim
metadata:
  name: ${CLAIM_NAME}
  namespace: ${POOL_NS}
spec:
  clusterPoolName: ${POOL_NAME}
  lifetime: 48h
EOF

echo "Waiting for claim to bind..."
kubectl get clusterclaim "$CLAIM_NAME" -n "$POOL_NS" \
  -o jsonpath='Cluster: {.spec.namespace}'
echo ""


# ═════════════════════════════════════════════════════════════════════
# RELEASE AND CLEANUP
# ═════════════════════════════════════════════════════════════════════

# kubectl delete clusterclaim ${CLAIM_NAME} -n ${POOL_NS}
# kubectl delete clusterpool ${POOL_NAME} -n ${POOL_NS}
# kubectl delete namespace ${POOL_NS}


# ═════════════════════════════════════════════════════════════════════
# Alternative: Using the acmlab CLI
# ═════════════════════════════════════════════════════════════════════
#
# acmlab pool create caas-pool --size 2 --image-set img4.22.9-multi-appsub --platform ibmcloud --pull-secret ~/pull-secret.json
# acmlab pool list
# acmlab claim create caas-pool --name my-test --ttl 48h
# acmlab claim release my-test
# acmlab pool delete caas-pool
