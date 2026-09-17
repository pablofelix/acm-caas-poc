#!/usr/bin/env bash
# UC-14: Cluster reclamation — TTL-based lifecycle management

set -euo pipefail

echo "=== UC-14: Cluster Reclamation ==="

echo "1. Set TTL on a cluster (72 hours)"
acmlab reclamation set-ttl spoke-dev --hours 72 --owner team-alpha

echo ""
echo "2. List all clusters with TTL"
acmlab reclamation list-ttl

echo ""
echo "3. Check for expired clusters"
acmlab reclamation check-expired

echo ""
echo "4. Extend TTL by 48 hours"
acmlab reclamation extend spoke-dev --hours 48 --justification "sprint extension"

echo ""
echo "5. Verify updated TTL"
acmlab reclamation list-ttl

echo ""
echo "6. Reclaim an expired cluster (hibernates Hive, detaches imported)"
acmlab reclamation reclaim spoke-old

echo ""
echo "=== Done ==="
