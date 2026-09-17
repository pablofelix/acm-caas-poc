#!/usr/bin/env bash
# UC-40: CAPI provisioning for vanilla Kubernetes clusters
set -euo pipefail

echo "=== UC-40: CAPI Provisioning ==="
echo ""

echo "--- Create a CAPI cluster (Docker provider) ---"
echo '$ acmlab provision create capi-test --type capi --kubernetes-version v1.30.0 --workers 2 --infra-provider docker'
acmlab provision create capi-test --type capi \
  --kubernetes-version v1.30.0 \
  --workers 2 \
  --infra-provider docker
echo ""

echo "--- Check provisioning status ---"
echo '$ acmlab provision status capi-test'
acmlab provision status capi-test
echo ""

echo "--- List all CAPI clusters ---"
echo '$ acmlab provision list-capi'
acmlab provision list-capi
echo ""

echo "--- List CAPI clusters (JSON) ---"
echo '$ acmlab provision list-capi --json'
acmlab provision list-capi --json
echo ""

echo "--- Destroy the CAPI cluster ---"
echo '$ acmlab provision destroy capi-test'
acmlab provision destroy capi-test
echo ""

echo "=== UC-40 Demo Complete ==="
