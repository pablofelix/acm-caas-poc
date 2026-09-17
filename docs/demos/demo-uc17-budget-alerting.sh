#!/usr/bin/env bash
# UC-17: Budget alerting — cost center attribution and budget policies

set -euo pipefail

echo "=== UC-17: Budget Alerting ==="

echo "1. Stamp cost centers on clusters"
acmlab cost stamp spoke1 platform
acmlab cost stamp spoke2 platform
acmlab cost stamp spoke3 data-science

echo ""
echo "2. View costs by cost center"
acmlab cost by-center

echo ""
echo "3. Create budget policy (platform: 5000 USD/month)"
acmlab cost create-budget platform 5000

echo ""
echo "4. Check budgets for overages"
acmlab cost check-budgets

echo ""
echo "5. Remove budget policy"
acmlab cost remove-budget platform

echo ""
echo "=== Done ==="
