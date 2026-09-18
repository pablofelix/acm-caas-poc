#!/usr/bin/env bash
# UC-53: Cluster templating (ClusterDeploymentCustomization)
# Manual step-by-step guide
# Maps to: internal/provisioning/templating.go, templating_builder.go
# CLI:     cmd/acmlab/provision.go (template-create, template-get, template-list, template-remove, template-apply)

set -euo pipefail

echo "--- Step 1: Create a cluster template ---"
echo "Templates use Hive ClusterDeploymentCustomization CRDs to define"
echo "reusable cluster profiles with installConfigPatches."
echo ""
echo 'acmlab provision template-create small-profile --patch '"'"'{"op":"replace","path":"/compute/0/replicas","value":"2"}'"'"''
acmlab provision template-create small-profile --patch '{"op":"replace","path":"/compute/0/replicas","value":"2"}'

echo ""
echo "--- Step 2: List available templates ---"
echo "acmlab provision template-list"
acmlab provision template-list

echo ""
echo "--- Step 3: Get template details ---"
echo "acmlab provision template-get small-profile"
acmlab provision template-get small-profile

echo ""
echo "--- Step 4: Apply template to a cluster deployment ---"
echo "acmlab provision template-apply spoke3 --template small-profile"
acmlab provision template-apply spoke3 --template small-profile

echo ""
echo "--- Step 5: Remove the template ---"
echo "acmlab provision template-remove small-profile"
acmlab provision template-remove small-profile
