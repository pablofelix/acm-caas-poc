#!/usr/bin/env bash
# UC-53: Cluster templating (ClusterDeploymentCustomization)
# Demonstrates creating and applying cluster templates

set -euo pipefail

echo "=== UC-53: Cluster Templating ==="

echo "1. Create a small cluster template"
acmlab provision template-create small-profile --patch '{"op":"replace","path":"/compute/0/replicas","value":"2"}' --patch '{"op":"replace","path":"/networking/machineCIDR","value":"10.0.0.0/16"}'

echo ""
echo "2. Create a large cluster template"
acmlab provision template-create large-profile --patch '{"op":"replace","path":"/compute/0/replicas","value":"6"}' --patch '{"op":"replace","path":"/compute/0/platform/aws/type","value":"m5.4xlarge"}'

echo ""
echo "3. List all templates"
acmlab provision template-list

echo ""
echo "4. Get template details"
acmlab provision template-get small-profile

echo ""
echo "5. Apply template to a cluster deployment"
acmlab provision template-apply spoke3 --template small-profile

echo ""
echo "6. Remove a template"
acmlab provision template-remove large-profile

echo ""
echo "=== Done ==="
