#!/usr/bin/env bash
# UC-31: SCAP scanning via Compliance Operator
set -euo pipefail

echo "=== UC-31: Compliance Operator ==="
echo ""

echo "--- Deploy Compliance Operator to a cluster ---"
echo '$ acmlab security deploy-compliance spoke1 --cluster-set default'
acmlab security deploy-compliance spoke1 --cluster-set default
echo ""

echo "--- Create a compliance scan (CIS node profile) ---"
echo '$ acmlab security scan spoke1 --profile cis-node'
acmlab security scan spoke1 --profile cis-node
echo ""

echo "--- Check scan status ---"
echo '$ acmlab security scan-status spoke1'
acmlab security scan-status spoke1
echo ""

echo "--- Get compliance report ---"
echo '$ acmlab security compliance-report spoke1'
acmlab security compliance-report spoke1
echo ""

echo "--- Get compliance report (JSON) ---"
echo '$ acmlab security compliance-report spoke1 --json'
acmlab security compliance-report spoke1 --json
echo ""

echo "--- Remove compliance resources ---"
echo '$ acmlab security remove-compliance spoke1'
acmlab security remove-compliance spoke1
echo ""

echo "=== UC-31 Demo Complete ==="
