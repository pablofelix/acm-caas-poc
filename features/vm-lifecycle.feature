@vm
Feature: VM lifecycle management via OpenShift Virtualization (UC-51)

  As a platform operator
  I want to manage virtual machines across managed clusters from the hub
  So that I can deploy and operate VMs alongside containers in a unified fleet

  Scenario: Deploy a virtual machine to a managed cluster
    Given a cluster "spoke1" is registered in ACM
    When I run "acmlab vm deploy --name web-vm --cluster spoke1 --cpu 4 --memory 8Gi"
    Then a ManifestWork "vm-web-vm-spoke1" is created in namespace "spoke1"
    And the ManifestWork wraps a kubevirt.io/v1 VirtualMachine
    And the VM is configured with 4 CPU cores and 8Gi memory

  Scenario: Deploy a VM with default values
    When I run "acmlab vm deploy --name test-vm --cluster spoke1"
    Then the VM defaults to 2 CPU cores, 4Gi memory, and RHEL 9 image

  Scenario: Start a stopped virtual machine
    Given a VM "web-vm" is deployed to "spoke1" in stopped state
    When I run "acmlab vm start web-vm --cluster spoke1"
    Then the ManifestWork is updated with spec.running=true

  Scenario: Stop a running virtual machine
    Given a VM "web-vm" is running on "spoke1"
    When I run "acmlab vm stop web-vm --cluster spoke1"
    Then the ManifestWork is updated with spec.running=false

  Scenario: Live-migrate a virtual machine
    Given a VM "web-vm" is running on "spoke1"
    When I run "acmlab vm migrate web-vm --cluster spoke1"
    Then a VirtualMachineInstanceMigration is added to the ManifestWork

  Scenario: Show VM status
    Given a VM "web-vm" is deployed to "spoke1"
    When I run "acmlab vm status web-vm --cluster spoke1"
    Then the output shows name, cluster, status, CPU, memory, and image

  Scenario: List all VMs across the fleet
    Given VMs are deployed to "spoke1" and "spoke2"
    When I run "acmlab vm list"
    Then the output shows name, cluster, status, and IP for all VMs

  Scenario: Remove a virtual machine
    Given a VM "web-vm" is deployed to "spoke1"
    When I run "acmlab vm remove web-vm --cluster spoke1"
    Then the ManifestWork is deleted from namespace "spoke1"
