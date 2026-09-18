#!/usr/bin/env bash
# UC-56: Cluster discovery via OpenShift Cluster Manager
# Demonstrates OCM-based discovery of unmanaged OpenShift clusters

set -euo pipefail

echo "=== UC-56: Cluster Discovery (OCM) ==="

echo "1. Enable discovery with OCM token"
acmlab discovery enable --namespace open-cluster-management --token "<ocm-api-token>"

echo ""
echo "2. Check discovery status"
acmlab discovery status --namespace open-cluster-management

echo ""
echo "3. List discovered clusters"
acmlab discovery list --namespace open-cluster-management

echo ""
echo "4. List as JSON"
acmlab discovery list --namespace open-cluster-management --json

echo ""
echo "5. Import a discovered cluster"
acmlab discovery import my-rosa-cluster --namespace open-cluster-management

echo ""
echo "6. Disable discovery"
acmlab discovery disable --namespace open-cluster-management

echo ""
echo "=== Done ==="
