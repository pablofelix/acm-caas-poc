#!/usr/bin/env bash
# UC-54: Global ManagedClusterSet
# Demonstrates enabling a global ClusterSet for cross-team visibility

set -euo pipefail

echo "=== UC-54: Global ManagedClusterSet ==="

echo "1. Enable the global ClusterSet"
acmlab clusterset global-enable

echo ""
echo "2. Bind global set to team namespaces"
acmlab clusterset global-bind team-alpha
acmlab clusterset global-bind team-beta

echo ""
echo "3. Check global set status"
acmlab clusterset global-status

echo ""
echo "4. Check status as JSON"
acmlab clusterset global-status --json

echo ""
echo "5. Unbind from a namespace"
acmlab clusterset global-unbind team-beta

echo ""
echo "6. Verify binding removed"
acmlab clusterset global-status

echo ""
echo "=== Done ==="
