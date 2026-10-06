#!/usr/bin/env bash
# UC-52: Observability stack customisation
# Demonstrates setup, verify, custom rules, dashboards, metrics, alertmanager,
# alert forwarding, cluster enable/disable, Grafana URL, advanced config, retention

set -euo pipefail

echo "=== UC-52: Observability Stack Customisation ==="

echo "1. Setup observability stack"
acmlab observability setup

echo ""
echo "2. Verify deployment"
acmlab observability verify

echo ""
echo "3. Check status"
acmlab observability status

echo ""
echo "4. Configure pull secret"
acmlab observability configure-pull-secret

echo ""
echo "5. Configure OBC storage"
acmlab observability configure-storage --storage-class gp3-csi --bucket thanos

echo ""
echo "6. Deploy custom Prometheus rules"
acmlab observability deploy-rules --rules-file custom-rules.yaml

echo ""
echo "7. Deploy a custom Grafana dashboard"
acmlab observability deploy-dashboard --name gpu-overview --dashboard-file gpu-dashboard.json

echo ""
echo "8. Configure custom metrics allowlist"
acmlab observability configure-metrics --metric node_cpu_seconds_total --metric container_memory_rss

echo ""
echo "9. Configure Alertmanager"
acmlab observability configure-alertmanager --config-file alertmanager.yaml

echo ""
echo "10. Disable alert forwarding"
acmlab observability disable-alerting

echo ""
echo "11. Re-enable alert forwarding"
acmlab observability enable-alerting

echo ""
echo "12. Disable observability for a cluster"
acmlab observability disable-cluster spoke1

echo ""
echo "13. Re-enable observability for a cluster"
acmlab observability enable-cluster spoke1

echo ""
echo "14. Discover Grafana URL"
acmlab observability grafana-url

echo ""
echo "15. Configure advanced MCO settings"
acmlab observability configure-advanced --receive-replicas 6 --collection-interval 30

echo ""
echo "16. Check add-on health"
acmlab observability addon-health

echo ""
echo "17. Configure retention settings"
acmlab observability configure-retention --retention 24h --block-duration 2h --delete-delay 48h

echo ""
echo "18. Diagnose observability health"
acmlab observability diagnose

echo ""
echo "19. Remove custom rules"
acmlab observability remove-rules

echo ""
echo "20. Remove custom dashboard"
acmlab observability remove-dashboard gpu-overview

echo ""
echo "21. Teardown observability stack"
acmlab observability teardown

echo ""
echo "=== Done ==="
