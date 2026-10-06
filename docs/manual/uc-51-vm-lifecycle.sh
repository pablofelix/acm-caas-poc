#!/usr/bin/env bash
# UC-51: VM lifecycle management — OpenShift Virtualization (manual)
# Shows raw ACM resources: ManifestWork wrapping KubeVirt VirtualMachine CRD
# Prerequisite: target cluster must have OpenShift Virtualization (KubeVirt) installed
# Install via governance policy: acmlab vm ensure-cnv <cluster>
# Or manually with: oc apply -f (Policy + ConfigurationPolicy for Namespace, OperatorGroup, Subscription, HyperConverged)

set -euo pipefail

VM_NAME="${1:-web-vm}"
CLUSTER="${2:-spoke1}"

echo "=== UC-51: VM Lifecycle (manual) ==="

echo "1. Create ManifestWork with VirtualMachine"
cat <<EOF | oc apply -f -
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: vm-${VM_NAME}-${CLUSTER}
  namespace: ${CLUSTER}
  labels:
    acmlab.redhat.com/vm: "true"
    acmlab.redhat.com/vm-name: ${VM_NAME}
  annotations:
    acmlab.redhat.com/vm-image: registry.redhat.io/rhel9/rhel-guest-image:latest
    acmlab.redhat.com/vm-disk: 20Gi
spec:
  workload:
    manifests:
      - apiVersion: kubevirt.io/v1
        kind: VirtualMachine
        metadata:
          name: ${VM_NAME}
          namespace: default
        spec:
          running: true
          template:
            metadata:
              labels:
                kubevirt.io/vm: ${VM_NAME}
            spec:
              domain:
                cpu:
                  cores: 4
                memory:
                  guest: 8Gi
                devices:
                  disks:
                    - name: rootdisk
                      disk:
                        bus: virtio
                    - name: cloudinitdisk
                      disk:
                        bus: virtio
                  interfaces:
                    - name: default
                      masquerade: {}
              networks:
                - name: default
                  pod: {}
              volumes:
                - name: rootdisk
                  containerDisk:
                    image: registry.redhat.io/rhel9/rhel-guest-image:latest
                - name: cloudinitdisk
                  cloudInitNoCloud:
                    userData: |
                      #cloud-config
                      password: acmlab
                      chpasswd: { expire: False }
EOF

echo ""
echo "2. Verify ManifestWork created"
oc get manifestwork -n ${CLUSTER} -l "acmlab.redhat.com/vm-name=${VM_NAME}"

echo ""
echo "3. Check ManifestWork status (Applied = spoke has the VM)"
oc get manifestwork vm-${VM_NAME}-${CLUSTER} -n ${CLUSTER} \
  -o jsonpath='{.status.conditions[?(@.type=="Applied")].status}' && echo ""

echo ""
echo "4. Stop the VM (patch spec.running=false)"
oc get manifestwork vm-${VM_NAME}-${CLUSTER} -n ${CLUSTER} -o json \
  | jq '.spec.workload.manifests[0].spec.running = false' \
  | oc apply -f -

echo ""
echo "5. Start the VM (patch spec.running=true)"
oc get manifestwork vm-${VM_NAME}-${CLUSTER} -n ${CLUSTER} -o json \
  | jq '.spec.workload.manifests[0].spec.running = true' \
  | oc apply -f -

echo ""
echo "6. Trigger live migration (add VirtualMachineInstanceMigration)"
oc get manifestwork vm-${VM_NAME}-${CLUSTER} -n ${CLUSTER} -o json \
  | jq --arg vm "${VM_NAME}" '.spec.workload.manifests += [{
      "apiVersion": "kubevirt.io/v1",
      "kind": "VirtualMachineInstanceMigration",
      "metadata": {"name": "migrate-"+$vm, "namespace": "default"},
      "spec": {"vmiName": $vm}
    }]' \
  | oc apply -f -

echo ""
echo "7. List all VM ManifestWorks across the fleet"
oc get manifestwork -A -l acmlab.redhat.com/vm=true \
  -o custom-columns='NAME:.metadata.name,CLUSTER:.metadata.namespace,VM:.metadata.labels.acmlab\.redhat\.com/vm-name,APPLIED:.status.conditions[?(@.type=="Applied")].status'

echo ""
echo "8. Remove the VM"
oc delete manifestwork vm-${VM_NAME}-${CLUSTER} -n ${CLUSTER}

echo ""
echo "=== Done ==="
