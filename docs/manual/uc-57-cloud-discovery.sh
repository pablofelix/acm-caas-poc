#!/usr/bin/env bash
# UC-57: Cloud-native cluster discovery (AWS, IBM Cloud, kubeconfig)
# Manual step-by-step guide
# Maps to: internal/discovery/cloud_discovery.go
# CLI:     cmd/acmlab/discovery.go (scan, auto-import, scan-kubeconfigs, auto-import-kubeconfig)

set -euo pipefail

echo "--- Step 1: Scan AWS for EKS and ROSA clusters ---"
echo "Requires aws and/or rosa CLI configured with valid credentials."
echo "Cross-references with ACM ManagedClusters to flag already-managed clusters."
echo ""
echo "acmlab discovery scan --provider aws --region us-east-1"
acmlab discovery scan --provider aws --region us-east-1

echo ""
echo "--- Step 2: Scan IBM Cloud for IKS and ROKS clusters ---"
echo "Requires ibmcloud CLI with ks (Kubernetes Service) plugin."
echo ""
echo "acmlab discovery scan --provider ibmcloud"
acmlab discovery scan --provider ibmcloud

echo ""
echo "--- Step 3: Scan all providers ---"
echo "acmlab discovery scan"
acmlab discovery scan

echo ""
echo "--- Step 4: Scan kubeconfig directory ---"
echo "Scans all kubeconfig files in a directory. Works for any cluster type."
echo "Matches by name and server URL against ACM ManagedClusters."
echo ""
echo "acmlab discovery scan-kubeconfigs --dir ~/.kube/"
acmlab discovery scan-kubeconfigs --dir ~/.kube/

echo ""
echo "--- Step 5: Auto-import a cloud-discovered cluster ---"
echo "acmlab discovery auto-import my-eks-cluster --provider aws"
echo "(skipped in this manual run)"

echo ""
echo "--- Step 6: Auto-import from kubeconfig ---"
echo "acmlab discovery auto-import-kubeconfig --name my-cluster --kubeconfig ~/.kube/my-cluster.kubeconfig"
echo "(skipped in this manual run)"
