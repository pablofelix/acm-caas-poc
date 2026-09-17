#!/usr/bin/env bash
# UC-39: Cloud-provider native scaling (NodePool / MachineDeployment auto-detect)
set -euo pipefail

echo "=== UC-39: Cloud-Native Scaling ==="
echo ""

echo "--- Scale a cluster (auto-detects MachinePool / NodePool / MachineDeployment) ---"
echo '$ acmlab scaling set spoke1 --replicas 4'
acmlab scaling set spoke1 --replicas 4
echo ""

echo "--- Scale a HyperShift cluster (auto-detects NodePool) ---"
echo '$ acmlab scaling set hosted-test --replicas 3'
acmlab scaling set hosted-test --replicas 3
echo ""

echo "--- Scale a CAPI cluster (auto-detects MachineDeployment) ---"
echo '$ acmlab scaling set capi-test --replicas 2'
acmlab scaling set capi-test --replicas 2
echo ""

echo "--- Check current scaling state ---"
echo '$ acmlab scaling get spoke1'
acmlab scaling get spoke1
echo ""

echo "--- List all scaling resources ---"
echo '$ acmlab scaling list'
acmlab scaling list
echo ""

echo "=== UC-39 Demo Complete ==="
