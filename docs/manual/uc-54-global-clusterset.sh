#!/usr/bin/env bash
# UC-54: Global ManagedClusterSet
# Manual step-by-step guide
# Maps to: internal/clusterset/global.go
# CLI:     cmd/acmlab/clusterset.go (global-enable, global-bind, global-unbind, global-status)

set -euo pipefail

echo "--- Step 1: Enable the global ClusterSet ---"
echo "Creates a ManagedClusterSet with selectorType=LabelSelector and empty"
echo "matchLabels, which matches ALL clusters. Labels them acmlab.redhat.com/global=true."
echo ""
echo "acmlab clusterset global-enable"
acmlab clusterset global-enable

echo ""
echo "--- Step 2: Bind the global set to a namespace ---"
echo "Binds allow the namespace to use Placements scoped to this set."
echo ""
echo "acmlab clusterset global-bind team-alpha"
acmlab clusterset global-bind team-alpha

echo ""
echo "--- Step 3: Check global set status ---"
echo "acmlab clusterset global-status"
acmlab clusterset global-status

echo ""
echo "--- Step 4: Unbind from a namespace ---"
echo "acmlab clusterset global-unbind team-alpha"
acmlab clusterset global-unbind team-alpha

echo ""
echo "--- Step 5: Verify ---"
echo "acmlab clusterset global-status"
acmlab clusterset global-status
