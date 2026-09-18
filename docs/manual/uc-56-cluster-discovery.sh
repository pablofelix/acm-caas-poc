#!/usr/bin/env bash
# UC-56: Cluster discovery via OpenShift Cluster Manager
# Manual step-by-step guide
# Maps to: internal/discovery/discovery.go, builder.go
# CLI:     cmd/acmlab/discovery.go (enable, disable, list, import, status)

set -euo pipefail
NS="${1:-open-cluster-management}"

echo "--- Step 1: Obtain OCM token ---"
echo "Run: ocm login --use-device-code"
echo "Then: export OCM_TOKEN=\$(ocm token)"
echo ""

if [ -z "${OCM_TOKEN:-}" ]; then
  echo "OCM_TOKEN not set. Set it before running this script."
  echo "export OCM_TOKEN=\$(ocm token)"
  exit 1
fi

echo "--- Step 2: Enable discovery ---"
echo "Creates a DiscoveryConfig (discovery.open-cluster-management.io/v1) and"
echo "a Secret containing the OCM API token in the target namespace."
echo ""
echo "acmlab discovery enable --namespace $NS --token \$OCM_TOKEN"
acmlab discovery enable --namespace "$NS" --token "$OCM_TOKEN"

echo ""
echo "--- Step 3: Check discovery status ---"
echo "acmlab discovery status --namespace $NS"
acmlab discovery status --namespace "$NS"

echo ""
echo "--- Step 4: List discovered clusters ---"
echo "Discovered clusters appear as DiscoveredCluster CRDs."
echo "These are OpenShift clusters registered with Red Hat (UPI, IPI, ROSA, ARO)."
echo ""
echo "acmlab discovery list --namespace $NS"
acmlab discovery list --namespace "$NS"

echo ""
echo "--- Step 5: Import a discovered cluster ---"
echo "acmlab discovery import <cluster-name> --namespace $NS"
echo "(skipped in this manual run)"

echo ""
echo "--- Step 6: Disable discovery ---"
echo "acmlab discovery disable --namespace $NS"
acmlab discovery disable --namespace "$NS"
