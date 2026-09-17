#!/usr/bin/env bash
# UC-19: Multi-cluster GPU workload routing via ACM Placement
# Routes GPU requests to the correct cluster based on type and availability

set -euo pipefail

echo "=== UC-19: GPU Workload Routing ==="

echo "1. Create GPU Placement for H100"
acmlab gpu route --gpu-type H100

echo ""
echo "2. Create GPU Placement for H100 in specific region"
acmlab gpu route --gpu-type H100 --region eu-gb

echo ""
echo "3. Find best available cluster for H100"
acmlab gpu best-cluster --gpu-type H100

echo ""
echo "4. Mark cluster as saturated (removes from routing pool)"
acmlab gpu mark-saturated gpu-h100-eugb

echo ""
echo "5. Clear saturation flag (re-adds to routing pool)"
acmlab gpu mark-saturated gpu-h100-eugb --clear

echo ""
echo "6. Find best cluster again (should include cleared cluster)"
acmlab gpu best-cluster --gpu-type H100

echo ""
echo "=== Done ==="
