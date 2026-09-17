#!/usr/bin/env bash
# UC-18: GPU sharing stack deployment (Kueue + Kyverno via ManifestWork)
# Deploys and monitors the GPU sharing infrastructure on GPU clusters

set -euo pipefail

echo "=== UC-18: GPU Sharing Stack ==="

echo "1. Deploy Kueue + Kyverno stack to GPU cluster"
acmlab gpu deploy-stack gpu-h100-eugb --cluster-set gpu-clusters

echo ""
echo "2. Check stack health (drift detection)"
acmlab gpu drift-status gpu-h100-eugb

echo ""
echo "3. Create ClusterQueues per GPU type"
acmlab gpu create-queues gpu-h100-eugb --gpu-types H100,L4

echo ""
echo "4. Verify deployment"
acmlab gpu drift-status gpu-h100-eugb

echo ""
echo "5. Remove stack from cluster"
acmlab gpu remove-stack gpu-h100-eugb

echo ""
echo "=== Done ==="
