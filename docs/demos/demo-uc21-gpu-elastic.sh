#!/usr/bin/env bash
# UC-21: Elastic GPU capacity — automatic on-demand provisioning
# Detects saturation, provisions on-demand clusters, hibernates idle ones

set -euo pipefail

echo "=== UC-21: Elastic GPU Capacity ==="

echo "1. Detect GPU saturation on a cluster"
acmlab gpu detect-saturation gpu-h100-eugb --threshold 85

echo ""
echo "2. Provision an on-demand GPU cluster"
acmlab gpu provision-ondemand gpu-ondemand-001 --gpu-type L4

echo ""
echo "3. List all elastic (on-demand) clusters"
acmlab gpu list-elastic

echo ""
echo "4. Hibernate an idle on-demand cluster"
acmlab gpu hibernate-idle gpu-ondemand-001

echo ""
echo "5. Verify elastic cluster state"
acmlab gpu list-elastic

echo ""
echo "=== Done ==="
