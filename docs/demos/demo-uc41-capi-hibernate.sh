#!/usr/bin/env bash
# UC-41: CAPI cluster hibernate via scale-to-zero
set -euo pipefail

echo "=== UC-41: CAPI Hibernate ==="
echo ""

echo "--- Check cluster lifecycle status ---"
echo '$ acmlab lifecycle status capi-test'
acmlab lifecycle status capi-test
echo ""

echo "--- Hibernate a CAPI cluster (scales MachineDeployment to 0) ---"
echo '$ acmlab lifecycle hibernate capi-test'
acmlab lifecycle hibernate capi-test
echo ""

echo "--- Verify hibernated state ---"
echo '$ acmlab lifecycle status capi-test'
acmlab lifecycle status capi-test
echo ""

echo "--- Resume the CAPI cluster (restores original replicas) ---"
echo '$ acmlab lifecycle resume capi-test'
acmlab lifecycle resume capi-test
echo ""

echo "--- Verify resumed state ---"
echo '$ acmlab lifecycle status capi-test'
acmlab lifecycle status capi-test
echo ""

echo "=== UC-41 Demo Complete ==="
