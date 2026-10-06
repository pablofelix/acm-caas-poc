package virtualization

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func buildVMManifestWork(cluster, name string, opts VMOpts) *unstructured.Unstructured {
	vm := map[string]interface{}{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"running": true,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"kubevirt.io/vm": name,
					},
				},
				"spec": map[string]interface{}{
					"domain": map[string]interface{}{
						"cpu": map[string]interface{}{
							"cores": mustParseInt(opts.CPU),
						},
						"memory": map[string]interface{}{
							"guest": opts.Memory,
						},
						"devices": map[string]interface{}{
							"disks": []interface{}{
								map[string]interface{}{
									"name": "rootdisk",
									"disk": map[string]interface{}{
										"bus": "virtio",
									},
								},
								map[string]interface{}{
									"name": "cloudinitdisk",
									"disk": map[string]interface{}{
										"bus": "virtio",
									},
								},
							},
							"interfaces": []interface{}{
								map[string]interface{}{
									"name":       "default",
									"masquerade": map[string]interface{}{},
								},
							},
						},
					},
					"networks": []interface{}{
						map[string]interface{}{
							"name": "default",
							"pod":  map[string]interface{}{},
						},
					},
					"volumes": []interface{}{
						map[string]interface{}{
							"name": "rootdisk",
							"containerDisk": map[string]interface{}{
								"image": opts.Image,
							},
						},
						map[string]interface{}{
							"name": "cloudinitdisk",
							"cloudInitNoCloud": map[string]interface{}{
								"userData": "#cloud-config\npassword: acmlab\nchpasswd: { expire: False }\n",
							},
						},
					},
				},
			},
		},
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "work.open-cluster-management.io/v1",
			"kind":       "ManifestWork",
			"metadata": map[string]interface{}{
				"name":      mwName(name, cluster),
				"namespace": cluster,
				"labels": map[string]interface{}{
					LabelVM:     "true",
					LabelVMName: name,
				},
				"annotations": map[string]interface{}{
					"acmlab.redhat.com/vm-image": opts.Image,
					"acmlab.redhat.com/vm-disk":  opts.DiskSize,
				},
			},
			"spec": map[string]interface{}{
				"workload": map[string]interface{}{
					"manifests": []interface{}{vm},
				},
			},
		},
	}
}

func buildCNVPolicy(cluster string) *unstructured.Unstructured {
	name := cnvPolicyName(cluster)
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy.open-cluster-management.io/v1",
			"kind":       "Policy",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": PolicyNamespace,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
					"acmlab.redhat.com/cnv":     "true",
				},
			},
			"spec": map[string]interface{}{
				"disabled":          false,
				"remediationAction": "enforce",
				"policy-templates": []interface{}{
					map[string]interface{}{
						"objectDefinition": map[string]interface{}{
							"apiVersion": "policy.open-cluster-management.io/v1",
							"kind":       "ConfigurationPolicy",
							"metadata": map[string]interface{}{
								"name": name + "-prereqs",
							},
							"spec": map[string]interface{}{
								"remediationAction":   "enforce",
								"severity":            "high",
								"pruneObjectBehavior": "DeleteIfCreated",
								"object-templates": []interface{}{
									map[string]interface{}{
										"complianceType": "musthave",
										"objectDefinition": map[string]interface{}{
											"apiVersion": "v1",
											"kind":       "Namespace",
											"metadata": map[string]interface{}{
												"name": CNVNamespace,
											},
										},
									},
									map[string]interface{}{
										"complianceType": "musthave",
										"objectDefinition": map[string]interface{}{
											"apiVersion": "operators.coreos.com/v1",
											"kind":       "OperatorGroup",
											"metadata": map[string]interface{}{
												"name":      CNVOperatorName + "-group",
												"namespace": CNVNamespace,
											},
											"spec": map[string]interface{}{
												"targetNamespaces": []interface{}{CNVNamespace},
											},
										},
									},
									map[string]interface{}{
										"complianceType": "musthave",
										"objectDefinition": map[string]interface{}{
											"apiVersion": "operators.coreos.com/v1alpha1",
											"kind":       "Subscription",
											"metadata": map[string]interface{}{
												"name":      CNVOperatorName,
												"namespace": CNVNamespace,
											},
											"spec": map[string]interface{}{
												"channel":             CNVOperatorChannel,
												"name":                CNVOperatorName,
												"source":              "redhat-operators",
												"sourceNamespace":     "openshift-marketplace",
												"installPlanApproval": "Automatic",
											},
										},
									},
								},
							},
						},
					},
					map[string]interface{}{
						"objectDefinition": map[string]interface{}{
							"apiVersion": "policy.open-cluster-management.io/v1",
							"kind":       "ConfigurationPolicy",
							"metadata": map[string]interface{}{
								"name": name + "-hyperconverged",
							},
							"spec": map[string]interface{}{
								"remediationAction":   "enforce",
								"severity":            "high",
								"pruneObjectBehavior": "DeleteIfCreated",
								"object-templates": []interface{}{
									map[string]interface{}{
										"complianceType": "musthave",
										"objectDefinition": map[string]interface{}{
											"apiVersion": "hco.kubevirt.io/v1beta1",
											"kind":       "HyperConverged",
											"metadata": map[string]interface{}{
												"name":      CNVOperatorName,
												"namespace": CNVNamespace,
											},
											"spec": map[string]interface{}{},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildCNVPlacement(cluster string) *unstructured.Unstructured {
	name := cnvPolicyName(cluster)
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1beta1",
			"kind":       "Placement",
			"metadata": map[string]interface{}{
				"name":      name + "-placement",
				"namespace": PolicyNamespace,
			},
			"spec": map[string]interface{}{
				"predicates": []interface{}{
					map[string]interface{}{
						"requiredClusterSelector": map[string]interface{}{
							"labelSelector": map[string]interface{}{
								"matchExpressions": []interface{}{
									map[string]interface{}{
										"key":      "name",
										"operator": "In",
										"values":   []interface{}{cluster},
									},
								},
							},
						},
					},
				},
				"tolerations": []interface{}{
					map[string]interface{}{
						"key":      "cluster.open-cluster-management.io/unreachable",
						"operator": "Exists",
					},
					map[string]interface{}{
						"key":      "cluster.open-cluster-management.io/unavailable",
						"operator": "Exists",
					},
				},
			},
		},
	}
}

func buildCNVPlacementBinding(cluster string) *unstructured.Unstructured {
	name := cnvPolicyName(cluster)
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy.open-cluster-management.io/v1",
			"kind":       "PlacementBinding",
			"metadata": map[string]interface{}{
				"name":      name + "-placement-binding",
				"namespace": PolicyNamespace,
			},
			"placementRef": map[string]interface{}{
				"apiGroup": "cluster.open-cluster-management.io",
				"kind":     "Placement",
				"name":     name + "-placement",
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"apiGroup": "policy.open-cluster-management.io",
					"kind":     "Policy",
					"name":     name,
				},
			},
		},
	}
}

func mustParseInt(s string) int64 {
	var n int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int64(c-'0')
		}
	}
	if n == 0 {
		return 2
	}
	return n
}
