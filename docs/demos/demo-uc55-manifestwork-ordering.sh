#!/usr/bin/env bash
# UC-55: ManifestWork ordering (dependency-aware sequencing)
# Demonstrates ordinal-based manifest deployment

set -euo pipefail

echo "=== UC-55: ManifestWork Ordering ==="

echo "1. Create an ordered ManifestWork (namespace first, then configmap, then deployment)"
acmlab workorder create-ordered app-stack --cluster spoke1 \
  --manifest '{"ordinal":0,"object":{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"my-app"}}}' \
  --manifest '{"ordinal":1,"object":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app-config","namespace":"my-app"},"data":{"key":"value"}}}' \
  --manifest '{"ordinal":2,"object":{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"app","namespace":"my-app"}}}'

echo ""
echo "2. Get ordered work status"
acmlab workorder get-ordered app-stack --cluster spoke1

echo ""
echo "3. Get status as JSON"
acmlab workorder get-ordered app-stack --cluster spoke1 --json

echo ""
echo "4. List ordered works on a cluster"
acmlab workorder list-ordered --cluster spoke1

echo ""
echo "5. Remove ordered work"
acmlab workorder remove-ordered app-stack --cluster spoke1

echo ""
echo "=== Done ==="
