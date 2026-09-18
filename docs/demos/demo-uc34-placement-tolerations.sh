#!/usr/bin/env bash
# UC-34: Placement tolerations and taints
# Demonstrates tainting clusters and creating tolerant placements

set -euo pipefail

echo "=== UC-34: Placement Tolerations and Taints ==="

echo "1. Add a taint to a GPU cluster"
acmlab fleet add-taint gpu-spoke1 --key gpu-workloads --value reserved --effect NoSchedule

echo ""
echo "2. Add a taint to another cluster"
acmlab fleet add-taint gpu-spoke2 --key gpu-workloads --value reserved --effect NoSchedule

echo ""
echo "3. List taints on a cluster"
acmlab fleet list-taints gpu-spoke1

echo ""
echo "4. Create a placement that tolerates the GPU taint"
acmlab fleet create-tolerant-placement ml-workloads --tolerate gpu-workloads=reserved --namespace default --cluster-set gpu-set

echo ""
echo "5. Remove a taint"
acmlab fleet remove-taint gpu-spoke1 --key gpu-workloads

echo ""
echo "6. Verify taint removed"
acmlab fleet list-taints gpu-spoke1

echo ""
echo "=== Done ==="
