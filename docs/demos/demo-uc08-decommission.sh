#!/usr/bin/env bash
# UC-08: Legacy cluster decommissioning lifecycle
# Demo: full decommission workflow — audit, start, advance, cancel
set -euo pipefail

CLUSTER="${1:-spoke2}"
OWNER="${2:-admin@example.com}"

echo "=== UC-08: Cluster Decommissioning Lifecycle ==="

echo "--- Step 1: Standalone audit (read-only) ---"
acmlab decommission audit "$CLUSTER"

echo "--- Step 2: Check no active workflows ---"
acmlab decommission list

echo "--- Step 3: Start decommission workflow ---"
acmlab decommission start "$CLUSTER" --owner "$OWNER"

echo "--- Step 4: Check status (should be audited) ---"
acmlab decommission status "$CLUSTER"

echo "--- Step 5: List active workflows ---"
acmlab decommission list

echo "--- Step 6: Idempotency — second start returns existing state ---"
acmlab decommission start "$CLUSTER" --owner "someone-else@example.com"

echo "--- Step 7: Advance to notified ---"
acmlab decommission advance "$CLUSTER"

echo "--- Step 8: Check status after advance ---"
acmlab decommission status "$CLUSTER"

echo "--- Step 9: Cancel (keeps cluster intact) ---"
acmlab decommission cancel "$CLUSTER"

echo "--- Step 10: Verify cleanup ---"
acmlab decommission list

echo ""
echo "=== Demo complete ==="
echo "Cluster $CLUSTER was NOT decommissioned — workflow was cancelled at 'notified' phase."
echo "To run a full lifecycle (WARNING: destructive), advance through all 7 phases without cancelling."
