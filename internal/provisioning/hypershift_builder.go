package provisioning

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func generateClusterID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func buildHostedCluster(opts HyperShiftOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": map[string]interface{}{
				"release": map[string]interface{}{
					"image": opts.ReleaseImage,
				},
				"pullSecret": map[string]interface{}{
					"name": opts.Name + "-pull-secret",
				},
				"platform": map[string]interface{}{
					"type": platformToHyperShiftType(opts.Platform),
				},
				"infraID":   opts.InfraID,
				"clusterID": generateClusterID(),
				"dns": map[string]interface{}{
					"baseDomain": opts.BaseDomain,
				},
				"services": []interface{}{
					map[string]interface{}{
						"service":            "APIServer",
						"servicePublishingStrategy": map[string]interface{}{
							"type": "LoadBalancer",
						},
					},
					map[string]interface{}{
						"service":            "OAuthServer",
						"servicePublishingStrategy": map[string]interface{}{
							"type": "Route",
						},
					},
					map[string]interface{}{
						"service":            "Konnectivity",
						"servicePublishingStrategy": map[string]interface{}{
							"type": "Route",
						},
					},
					map[string]interface{}{
						"service":            "Ignition",
						"servicePublishingStrategy": map[string]interface{}{
							"type": "Route",
						},
					},
				},
				"networking": map[string]interface{}{
					"clusterNetwork": []interface{}{
						map[string]interface{}{
							"cidr": "10.132.0.0/14",
						},
					},
					"serviceNetwork": []interface{}{
						map[string]interface{}{
							"cidr": "172.31.0.0/16",
						},
					},
				},
				"etcd": map[string]interface{}{
					"managementType": "Managed",
					"managed": map[string]interface{}{
						"storage": map[string]interface{}{
							"persistentVolume": map[string]interface{}{
								"size": "8Gi",
							},
							"type": "PersistentVolume",
						},
					},
				},
			},
		},
	}
}

func buildNodePool(opts HyperShiftOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Namespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": map[string]interface{}{
				"clusterName": opts.Name,
				"replicas":    opts.NodePoolReplicas,
				"release": map[string]interface{}{
					"image": opts.ReleaseImage,
				},
				"platform": map[string]interface{}{
					"type": platformToHyperShiftType(opts.Platform),
				},
				"management": map[string]interface{}{
					"upgradeType": "Replace",
				},
			},
		},
	}
}

func buildHyperShiftPullSecret(opts HyperShiftOpts) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      opts.Name + "-pull-secret",
				"namespace": opts.Namespace,
			},
			"type": "kubernetes.io/dockerconfigjson",
			"data": map[string]interface{}{
				".dockerconfigjson": base64.StdEncoding.EncodeToString([]byte(opts.PullSecret)),
			},
		},
	}
}

func buildHyperShiftManagedCluster(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"vendor":                                        "OpenShift",
					"acmlab.redhat.com/managed":                     "true",
					"acmlab.redhat.com/provisioning-type":           "hypershift",
					"cluster.open-cluster-management.io/clusterset": "default",
				},
				"annotations": map[string]interface{}{
					"import.open-cluster-management.io/hosting-cluster-name": "local-cluster",
					"import.open-cluster-management.io/klusterlet-deploy-mode": "Hosted",
				},
			},
			"spec": map[string]interface{}{
				"hubAcceptsClient": true,
			},
		},
	}
}

func platformToHyperShiftType(platform string) string {
	switch platform {
	case "aws":
		return "AWS"
	case "azure":
		return "Azure"
	case "ibmcloud":
		return "IBMCloud"
	case "kubevirt":
		return "KubeVirt"
	default:
		return "None"
	}
}
