#!/usr/bin/env bash
# UC-11: Cost tracking — estimate cluster costs from node metadata
# Uses ManagedClusterInfo instance types + pricing table (ADR-009)

set -euo pipefail

echo "=== UC-11: Cost Tracking ==="

echo "1. Get cost estimate for a single cluster"
acmlab cost cluster spoke1

echo ""
echo "2. Get cost estimate as JSON"
acmlab cost cluster spoke1 --json

echo ""
echo "3. Stamp cost center on clusters"
acmlab cost stamp spoke1 engineering
acmlab cost stamp spoke2 data-science
acmlab cost stamp spoke3 engineering

echo ""
echo "4. Generate fleet-wide cost report"
acmlab cost report

echo ""
echo "5. Generate cost report as CSV"
acmlab cost report --format csv

echo ""
echo "6. Aggregate costs by cost center"
acmlab cost by-center

echo ""
echo "7. Aggregate costs as JSON"
acmlab cost by-center --json

echo ""
echo "=== Done ==="
