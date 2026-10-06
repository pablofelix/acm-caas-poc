#!/usr/bin/env bash
# UC-01: Cluster provisioning via Hive ClusterDeployment (multi-platform)
# Demo: provision a cluster, monitor, verify, destroy
#
# This demo has two phases because provisioning takes ~30-40 minutes.
# Run Phase 1, wait for provisioning to complete, then run Phase 2.
#
# Usage:
#   ./demo-uc01-provision.sh launch [cluster-name]    # Phase 1: create
#   ./demo-uc01-provision.sh verify [cluster-name]    # Phase 2: verify
#   ./demo-uc01-provision.sh destroy [cluster-name]   # Cleanup
set -euo pipefail

PHASE="${1:-launch}"
CLUSTER_NAME="${2:-demo-cluster}"
PULL_SECRET="${PULL_SECRET_PATH:-~/pull-secret.json}"
REGION="${ACM_REGION:-us-south}"

echo "=== UC-01: Cluster Provisioning ==="

case "$PHASE" in
  launch)
    echo "--- Phase 1: Launch ---"

    echo "Step 1: Preflight checks"
    acmlab provision preflight "$CLUSTER_NAME" \
      --pull-secret "$PULL_SECRET"

    echo "Step 2: List available ClusterImageSets"
    acmlab provision image-sets

    echo "Step 3: Provision the cluster"
    acmlab provision create "$CLUSTER_NAME" \
      --pull-secret "$PULL_SECRET" \
      --region "$REGION" \
      --workers 2

    echo "Step 4: Initial status"
    acmlab provision status "$CLUSTER_NAME"

    echo ""
    echo "Provisioning takes ~30-40 minutes."
    echo "Monitor progress with:  acmlab provision status $CLUSTER_NAME"
    echo "When installed, run:    $0 verify $CLUSTER_NAME"
    ;;

  verify)
    echo "--- Phase 2: Verify ---"

    echo "Step 1: Provisioning status"
    acmlab provision status "$CLUSTER_NAME"

    echo "Step 2: List all provisioned clusters"
    acmlab provision list

    echo "Step 3: Verify in fleet"
    acmlab fleet status "$CLUSTER_NAME"

    echo ""
    echo "Cluster is ready. To destroy: $0 destroy $CLUSTER_NAME"
    ;;

  destroy)
    echo "--- Cleanup: Destroy ---"
    acmlab provision destroy "$CLUSTER_NAME"
    echo "Cluster $CLUSTER_NAME destruction initiated."
    echo "Monitor with: acmlab provision status $CLUSTER_NAME"
    ;;

  *)
    echo "Usage: $0 {launch|verify|destroy} [cluster-name]"
    exit 1
    ;;
esac
