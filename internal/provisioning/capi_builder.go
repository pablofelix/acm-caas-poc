package provisioning

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func copyMap(m map[string]interface{}) map[string]interface{} {
	c := make(map[string]interface{}, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func k8sMinorVersion(version string) int {
	v := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0
	}
	minor, _ := strconv.Atoi(parts[1])
	return minor
}

func buildCAPICluster(opts CAPIClusterOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta2",
			"kind":       "Cluster",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed":        "true",
					"acmlab.redhat.com/provisioner":    "capi",
					"acmlab.redhat.com/infra-provider": opts.InfraProvider,
				},
			},
			"spec": map[string]interface{}{
				"clusterNetwork": map[string]interface{}{
					"pods": map[string]interface{}{
						"cidrBlocks": []interface{}{"192.168.0.0/16"},
					},
					"services": map[string]interface{}{
						"cidrBlocks": []interface{}{"10.128.0.0/12"},
					},
				},
				"infrastructureRef": map[string]interface{}{
					"apiGroup": "infrastructure.cluster.x-k8s.io",
					"kind":     infraClusterKind(opts.InfraProvider),
					"name":     opts.Name,
				},
				"controlPlaneRef": map[string]interface{}{
					"apiGroup": "controlplane.cluster.x-k8s.io",
					"kind":     "KubeadmControlPlane",
					"name":     opts.Name + "-control-plane",
				},
			},
		},
	}
}

func infraClusterKind(provider string) string {
	switch provider {
	case "aws":
		return "AWSCluster"
	case "azure":
		return "AzureCluster"
	case "gcp":
		return "GCPCluster"
	case "docker":
		return "DockerCluster"
	default:
		return "InfrastructureCluster"
	}
}

func buildCAPIMachineDeployment(opts CAPIClusterOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta2",
			"kind":       "MachineDeployment",
			"metadata": map[string]interface{}{
				"name":      opts.Name + "-workers",
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
					"cluster.x-k8s.io/cluster-name": opts.Name,
				},
			},
			"spec": map[string]interface{}{
				"clusterName": opts.Name,
				"replicas":    opts.WorkerReplicas,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"cluster.x-k8s.io/cluster-name": opts.Name,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"cluster.x-k8s.io/cluster-name": opts.Name,
						},
					},
					"spec": map[string]interface{}{
						"clusterName": opts.Name,
						"version":     opts.KubernetesVersion,
						"bootstrap": map[string]interface{}{
							"configRef": map[string]interface{}{
								"apiGroup": "bootstrap.cluster.x-k8s.io",
								"kind":     "KubeadmConfigTemplate",
								"name":     opts.Name + "-workers",
							},
						},
						"infrastructureRef": map[string]interface{}{
							"apiGroup": "infrastructure.cluster.x-k8s.io",
							"kind":     infraMachineTemplateKind(opts.InfraProvider),
							"name":     opts.Name + "-workers",
						},
					},
				},
			},
		},
	}
}

func infraMachineTemplateKind(provider string) string {
	switch provider {
	case "aws":
		return "AWSMachineTemplate"
	case "azure":
		return "AzureMachineTemplate"
	case "gcp":
		return "GCPMachineTemplate"
	case "docker":
		return "DockerMachineTemplate"
	default:
		return "InfrastructureMachineTemplate"
	}
}

func buildCAPIClusterForAWS(opts CAPIClusterOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta2",
			"kind":       "Cluster",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed":         "true",
					"acmlab.redhat.com/provisioner":     "capi",
					"acmlab.redhat.com/infra-provider":  "aws",
					"acmlab.redhat.com/kubernetes-version": opts.KubernetesVersion,
				},
			},
			"spec": map[string]interface{}{
				"clusterNetwork": map[string]interface{}{
					"pods": map[string]interface{}{
						"cidrBlocks": []interface{}{"192.168.0.0/16"},
					},
					"services": map[string]interface{}{
						"cidrBlocks": []interface{}{"10.128.0.0/12"},
					},
				},
				"infrastructureRef": map[string]interface{}{
					"apiGroup": "infrastructure.cluster.x-k8s.io",
					"kind":     "AWSCluster",
					"name":     opts.Name,
				},
				"controlPlaneRef": map[string]interface{}{
					"apiGroup": "controlplane.cluster.x-k8s.io",
					"kind":     "KubeadmControlPlane",
					"name":     opts.Name + "-control-plane",
				},
			},
		},
	}
}

func buildAWSCluster(opts CAPIClusterOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
			"kind":       "AWSCluster",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": map[string]interface{}{
				"region":     opts.Region,
				"sshKeyName": opts.SSHKeyName,
			},
		},
	}
}

func buildAWSMachineTemplate(name string, opts CAPIClusterOpts, iamProfile string) *unstructured.Unstructured {
	machineSpec := map[string]interface{}{
		"instanceType": opts.InstanceType,
		"rootVolume": map[string]interface{}{
			"size": opts.RootVolumeSize,
		},
	}
	if iamProfile != "" {
		machineSpec["iamInstanceProfile"] = iamProfile
	}
	if opts.SSHKeyName != "" {
		machineSpec["sshKeyName"] = opts.SSHKeyName
	}
	if opts.AMI != "" {
		machineSpec["ami"] = map[string]interface{}{
			"id": opts.AMI,
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
			"kind":       "AWSMachineTemplate",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": machineSpec,
				},
			},
		},
	}
}

func buildKubeadmControlPlane(opts CAPIClusterOpts) *unstructured.Unstructured {
	nodeReg := map[string]interface{}{
		"name": "{{ ds.meta_data.local_hostname }}",
	}

	minor := k8sMinorVersion(opts.KubernetesVersion)
	if minor > 0 && minor < 31 {
		cloudProviderArg := []interface{}{
			map[string]interface{}{"name": "cloud-provider", "value": "external"},
		}
		nodeReg["kubeletExtraArgs"] = cloudProviderArg
	}

	kubeadmConfigSpec := map[string]interface{}{
		"initConfiguration": map[string]interface{}{
			"nodeRegistration": nodeReg,
		},
		"joinConfiguration": map[string]interface{}{
			"nodeRegistration": copyMap(nodeReg),
		},
	}

	if minor > 0 && minor < 29 {
		cloudProviderArg := []interface{}{
			map[string]interface{}{"name": "cloud-provider", "value": "external"},
		}
		kubeadmConfigSpec["clusterConfiguration"] = map[string]interface{}{
			"apiServer": map[string]interface{}{
				"extraArgs": cloudProviderArg,
			},
			"controllerManager": map[string]interface{}{
				"extraArgs": cloudProviderArg,
			},
		}
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "controlplane.cluster.x-k8s.io/v1beta2",
			"kind":       "KubeadmControlPlane",
			"metadata": map[string]interface{}{
				"name":      opts.Name + "-control-plane",
				"namespace": opts.Namespace,
			},
			"spec": map[string]interface{}{
				"replicas": opts.ControlPlaneReplicas,
				"version":  opts.KubernetesVersion,
				"machineTemplate": map[string]interface{}{
					"spec": map[string]interface{}{
						"infrastructureRef": map[string]interface{}{
							"apiGroup": "infrastructure.cluster.x-k8s.io",
							"kind":     "AWSMachineTemplate",
							"name":     opts.Name + "-control-plane",
						},
					},
				},
				"kubeadmConfigSpec": kubeadmConfigSpec,
			},
		},
	}
}

func buildKubeadmConfigTemplate(opts CAPIClusterOpts) *unstructured.Unstructured {
	nodeRegistration := map[string]interface{}{
		"name": "{{ ds.meta_data.local_hostname }}",
	}
	minor := k8sMinorVersion(opts.KubernetesVersion)
	if minor > 0 && minor < 31 {
		nodeRegistration["kubeletExtraArgs"] = []interface{}{
			map[string]interface{}{"name": "cloud-provider", "value": "external"},
		}
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta2",
			"kind":       "KubeadmConfigTemplate",
			"metadata": map[string]interface{}{
				"name":      opts.Name + "-workers",
				"namespace": opts.Namespace,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"joinConfiguration": map[string]interface{}{
							"nodeRegistration": nodeRegistration,
						},
					},
				},
			},
		},
	}
}

func buildCAPIManagedCluster(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"vendor":                                        "Kubernetes",
					"cluster.open-cluster-management.io/clusterset": "default",
					"acmlab.redhat.com/managed":                     "true",
					"acmlab.redhat.com/provisioner":                  "capi",
				},
			},
			"spec": map[string]interface{}{
				"hubAcceptsClient": true,
			},
		},
	}
}

func parseCAPIClusterInfo(obj map[string]interface{}) *CAPIClusterInfo {
	info := &CAPIClusterInfo{}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		info.Namespace, _ = meta["namespace"].(string)
	}
	if status, ok := obj["status"].(map[string]interface{}); ok {
		info.Phase, _ = status["phase"].(string)
		conditions, _ := status["conditions"].([]interface{})
		for _, raw := range conditions {
			cond, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			info.Conditions = append(info.Conditions, fmt.Sprintf("%s=%s", condType, condStatus))
			if condType == "Ready" && condStatus == "True" {
				info.Ready = true
			}
		}
	}
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if tmpl, ok := spec["topology"].(map[string]interface{}); ok {
			info.KubernetesVersion, _ = tmpl["version"].(string)
		}
	}
	if labels, ok := getLabels(obj); ok {
		if info.KubernetesVersion == "" {
			info.KubernetesVersion, _ = labels["acmlab.redhat.com/kubernetes-version"]
		}
	}
	return info
}

func getLabels(obj map[string]interface{}) (map[string]string, bool) {
	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	rawLabels, ok := meta["labels"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	labels := make(map[string]string, len(rawLabels))
	for k, v := range rawLabels {
		labels[k], _ = v.(string)
	}
	return labels, true
}
