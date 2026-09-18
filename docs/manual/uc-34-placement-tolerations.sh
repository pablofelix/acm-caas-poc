#!/usr/bin/env bash
# UC-34: Placement tolerations and taints
# Manual step-by-step guide
# Maps to: internal/fleet/taints.go, taints_builder.go
# CLI:     cmd/acmlab/fleet.go (add-taint, remove-taint, list-taints, create-tolerant-placement)

set -euo pipefail
CLUSTER="${1:-gpu-spoke1}"

echo "--- Step 1: Add a taint to a managed cluster ---"
echo "Taints prevent workloads from scheduling on a cluster unless"
echo "the Placement explicitly tolerates the taint."
echo ""
echo "acmlab fleet add-taint $CLUSTER --key gpu-workloads --value reserved --effect NoSchedule"
acmlab fleet add-taint "$CLUSTER" --key gpu-workloads --value reserved --effect NoSchedule

echo ""
echo "--- Step 2: Verify the taint ---"
echo "acmlab fleet list-taints $CLUSTER"
acmlab fleet list-taints "$CLUSTER"

echo ""
echo "--- Step 3: Create a tolerant Placement ---"
echo "This Placement will only schedule to clusters whose taints it tolerates."
echo "Without the toleration, the Placement skips tainted clusters."
echo ""
echo "acmlab fleet create-tolerant-placement ml-pipeline --tolerate gpu-workloads=reserved --namespace default"
acmlab fleet create-tolerant-placement ml-pipeline --tolerate gpu-workloads=reserved --namespace default

echo ""
echo "--- Step 4: Remove the taint ---"
echo "acmlab fleet remove-taint $CLUSTER --key gpu-workloads"
acmlab fleet remove-taint "$CLUSTER" --key gpu-workloads

echo ""
echo "--- Step 5: Verify taint removed ---"
echo "acmlab fleet list-taints $CLUSTER"
acmlab fleet list-taints "$CLUSTER"
