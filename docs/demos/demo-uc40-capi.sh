#!/usr/bin/env bash
# UC-40: CAPI provisioning for vanilla Kubernetes clusters on AWS
set -euo pipefail

echo "=== UC-40: CAPI Provisioning ==="
echo ""

echo "--- Step 1: Set up CAPI controllers on the hub ---"
echo '$ acmlab provision setup-capi --infra-providers aws --ssh-key-name acmlab-capi --region us-east-1'
acmlab provision setup-capi \
  --infra-providers aws \
  --ssh-key-name acmlab-capi \
  --region us-east-1
echo ""

echo "--- Step 2: Verify CAPI controllers are running ---"
echo '$ acmlab provision capi-status'
acmlab provision capi-status
echo ""

echo "--- Step 3: Create a CAPI cluster on AWS ---"
echo '$ acmlab provision create capi-test --type capi --infra-provider aws --region us-east-1 --kubernetes-version v1.34.8 --workers 2 --ssh-key-name acmlab-capi'
acmlab provision create capi-test --type capi \
  --infra-provider aws \
  --region us-east-1 \
  --kubernetes-version v1.34.8 \
  --workers 2 \
  --ssh-key-name acmlab-capi
echo ""

echo "--- Step 4: Check provisioning status ---"
echo '$ acmlab provision list-capi'
acmlab provision list-capi
echo ""

echo "--- Step 5: Monitor the cluster until Ready ---"
echo '$ kubectl get cluster,awscluster,kubeadmcontrolplane,machines,awsmachine -n capi-test'
kubectl get cluster,awscluster,kubeadmcontrolplane,machines,awsmachine -n capi-test
echo ""

echo "--- Step 6: Destroy the CAPI cluster ---"
echo '$ acmlab provision destroy capi-test'
acmlab provision destroy capi-test
echo ""

echo "=== UC-40 Demo Complete ==="
