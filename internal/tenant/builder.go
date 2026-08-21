package tenant

import "fmt"

func buildTenantManifests(opts TenantOpts) []interface{} {
	cpuLimit := opts.CPULimit
	if cpuLimit == "" {
		cpuLimit = "4"
	}
	memoryLimit := opts.MemoryLimit
	if memoryLimit == "" {
		memoryLimit = "8Gi"
	}
	podLimit := opts.PodLimit
	if podLimit == 0 {
		podLimit = int64(20)
	}
	team := opts.Team
	if team == "" {
		team = opts.Name
	}

	return []interface{}{
		namespaceManifest(opts.Name),
		roleBindingManifest(opts.Name, team),
		networkPolicyManifest(opts.Name),
		resourceQuotaManifest(opts.Name, cpuLimit, memoryLimit, podLimit),
	}
}

func namespaceManifest(name string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": name,
			"labels": map[string]interface{}{
				"acmlab.redhat.com/tenant": name,
			},
		},
	}
}

func roleBindingManifest(tenantName, team string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "RoleBinding",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("%s-admin", tenantName),
			"namespace": tenantName,
		},
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "ClusterRole",
			"name":     "edit",
		},
		"subjects": []interface{}{
			map[string]interface{}{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Group",
				"name":     team,
			},
		},
	}
}

func networkPolicyManifest(tenantName string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]interface{}{
			"name":      "deny-cross-namespace",
			"namespace": tenantName,
		},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"ingress": []interface{}{
				map[string]interface{}{
					"from": []interface{}{
						map[string]interface{}{
							"podSelector": map[string]interface{}{},
						},
					},
				},
			},
			"policyTypes": []interface{}{"Ingress"},
		},
	}
}

func resourceQuotaManifest(tenantName, cpuLimit, memoryLimit string, podLimit int64) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ResourceQuota",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("%s-quota", tenantName),
			"namespace": tenantName,
		},
		"spec": map[string]interface{}{
			"hard": map[string]interface{}{
				"requests.cpu":    cpuLimit,
				"requests.memory": memoryLimit,
				"pods":            fmt.Sprintf("%d", podLimit),
			},
		},
	}
}
