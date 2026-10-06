#!/usr/bin/env bash
# UC-51: VM lifecycle management (OpenShift Virtualization)
# Demonstrates deploy, start, stop, migrate, status, list, and remove VMs
# Prerequisite: target cluster must have OpenShift Virtualization (KubeVirt) installed
# Use 'acmlab vm ensure-cnv <cluster>' to install via ACM governance policy

set -euo pipefail

CLUSTER="${1:-spoke1}"

echo "=== UC-51: VM Lifecycle Management ==="

echo "0. Ensure OpenShift Virtualization is installed"
acmlab vm ensure-cnv ${CLUSTER}
acmlab vm cnv-status ${CLUSTER}

echo ""
echo "1. Deploy a VM to a managed cluster"
acmlab vm deploy --name web-vm --cluster ${CLUSTER} --cpu 4 --memory 8Gi

echo ""
echo "2. Check VM status"
acmlab vm status web-vm --cluster ${CLUSTER}

echo ""
echo "3. Deploy a second VM with defaults (2 CPU, 4Gi, RHEL 9)"
acmlab vm deploy --name test-vm --cluster ${CLUSTER}

echo ""
echo "4. List all VMs across clusters"
acmlab vm list

echo ""
echo "5. Stop the VM"
acmlab vm stop web-vm --cluster ${CLUSTER}

echo ""
echo "6. Verify stopped state"
acmlab vm status web-vm --cluster ${CLUSTER}

echo ""
echo "7. Start the VM"
acmlab vm start web-vm --cluster ${CLUSTER}

echo ""
echo "8. Trigger live migration"
acmlab vm migrate web-vm --cluster ${CLUSTER}

echo ""
echo "9. Remove both VMs"
acmlab vm remove web-vm --cluster ${CLUSTER}
acmlab vm remove test-vm --cluster ${CLUSTER}

echo ""
echo "10. Verify removal"
acmlab vm list

echo ""
echo "=== Done ==="
