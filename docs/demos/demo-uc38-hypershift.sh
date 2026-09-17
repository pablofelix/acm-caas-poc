#!/usr/bin/env bash
# UC-38: HyperShift (HostedCluster) provisioning
set -euo pipefail

echo "=== UC-38: HyperShift Provisioning ==="
echo ""

echo "--- Create a HyperShift hosted cluster ---"
echo '$ acmlab provision create hosted-test --type hypershift --release-image quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64 --workers 2 --pull-secret ~/pull-secret.json --platform aws --region us-east-1 --base-domain example.com'
acmlab provision create hosted-test --type hypershift \
  --release-image quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64 \
  --workers 2 \
  --pull-secret ~/pull-secret.json \
  --platform aws --region us-east-1 --base-domain example.com
echo ""

echo "--- Check provisioning status ---"
echo '$ acmlab provision status hosted-test'
acmlab provision status hosted-test
echo ""

echo "--- List all HyperShift hosted clusters ---"
echo '$ acmlab provision list-hosted'
acmlab provision list-hosted
echo ""

echo "--- List all HyperShift clusters (JSON) ---"
echo '$ acmlab provision list-hosted --json'
acmlab provision list-hosted --json
echo ""

echo "--- Destroy the hosted cluster ---"
echo '$ acmlab provision destroy hosted-test'
acmlab provision destroy hosted-test
echo ""

echo "=== UC-38 Demo Complete ==="
