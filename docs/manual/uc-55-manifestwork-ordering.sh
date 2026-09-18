#!/usr/bin/env bash
# UC-55: ManifestWork ordering (ordinal-based sequencing)
# Manual step-by-step guide
# Maps to: internal/rollout/ordering.go, ordering_builder.go
# CLI:     cmd/acmlab/workorder.go (create-ordered, get-ordered, list-ordered, remove-ordered)

set -euo pipefail
CLUSTER="${1:-spoke1}"

echo "--- Step 1: Prepare ordered manifests file ---"
echo "The ManifestWork uses ordinal fields to sequence resource application."
echo "Lower ordinal = applied first. Useful for namespace-before-deployment ordering."
echo ""

cat > /tmp/ordered-manifests.json <<'EOF'
[
  {
    "ordinal": 0,
    "object": {
      "apiVersion": "v1",
      "kind": "Namespace",
      "metadata": { "name": "my-app" }
    }
  },
  {
    "ordinal": 1,
    "object": {
      "apiVersion": "v1",
      "kind": "ConfigMap",
      "metadata": { "name": "app-config", "namespace": "my-app" },
      "data": { "key": "value" }
    }
  }
]
EOF
echo "Manifests file written to /tmp/ordered-manifests.json"

echo ""
echo "--- Step 2: Create ordered ManifestWork ---"
echo "acmlab workorder create-ordered app-stack --cluster $CLUSTER --manifests /tmp/ordered-manifests.json"
acmlab workorder create-ordered app-stack --cluster "$CLUSTER" --manifests /tmp/ordered-manifests.json

echo ""
echo "--- Step 3: Get status ---"
echo "acmlab workorder get-ordered app-stack --cluster $CLUSTER"
acmlab workorder get-ordered app-stack --cluster "$CLUSTER"

echo ""
echo "--- Step 4: List ordered works ---"
echo "acmlab workorder list-ordered --cluster $CLUSTER"
acmlab workorder list-ordered --cluster "$CLUSTER"

echo ""
echo "--- Step 5: Remove ordered work ---"
echo "acmlab workorder remove-ordered app-stack --cluster $CLUSTER"
acmlab workorder remove-ordered app-stack --cluster "$CLUSTER"

rm -f /tmp/ordered-manifests.json
