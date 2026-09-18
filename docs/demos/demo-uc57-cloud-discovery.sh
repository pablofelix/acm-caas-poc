#!/usr/bin/env bash
# UC-57: Cloud-native cluster discovery (AWS, IBM Cloud, kubeconfig)
# Demonstrates discovering clusters from cloud providers and kubeconfig files

set -euo pipefail

echo "=== UC-57: Cloud-Native Cluster Discovery ==="

echo "1. Scan all configured cloud providers"
acmlab discovery scan

echo ""
echo "2. Scan AWS only (EKS + ROSA)"
acmlab discovery scan --provider aws

echo ""
echo "3. Scan AWS with region filter"
acmlab discovery scan --provider aws --region us-east-1

echo ""
echo "4. Scan IBM Cloud (IKS + ROKS)"
acmlab discovery scan --provider ibmcloud

echo ""
echo "5. Scan kubeconfig directory"
acmlab discovery scan-kubeconfigs --dir ~/.kube/

echo ""
echo "6. Scan as JSON"
acmlab discovery scan --provider aws --json

echo ""
echo "7. Auto-import a cloud-discovered cluster"
acmlab discovery auto-import my-eks-cluster --provider aws

echo ""
echo "8. Auto-import from kubeconfig"
acmlab discovery auto-import-kubeconfig --name my-cluster --kubeconfig ~/.kube/my-cluster.kubeconfig

echo ""
echo "=== Done ==="
