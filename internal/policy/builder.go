package policy

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type PolicyOpts struct {
	Name              string
	Namespace         string
	RemediationAction string
	ClusterLabels     map[string]string
	AllowedRegistries []string
}

func (m *Manager) ensurePolicy(ctx context.Context, namespace string, opts PolicyOpts) error {
	remediation := opts.RemediationAction
	if remediation == "" {
		remediation = "inform"
	}

	objectTemplates := buildObjectTemplates(opts)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy.open-cluster-management.io/v1",
			"kind":       "Policy",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"disabled":          false,
				"remediationAction": remediation,
				"policy-templates": []interface{}{
					map[string]interface{}{
						"objectDefinition": map[string]interface{}{
							"apiVersion": "policy.open-cluster-management.io/v1",
							"kind":       "ConfigurationPolicy",
							"metadata": map[string]interface{}{
								"name": opts.Name + "-config",
							},
							"spec": map[string]interface{}{
								"remediationAction":  remediation,
								"severity":           "medium",
								"object-templates":   objectTemplates,
								"pruneObjectBehavior": "None",
							},
						},
					},
				},
			},
		},
	}
	return m.createIfNotExists(ctx, client.GVRPolicy, namespace, obj)
}

func (m *Manager) ensurePlacement(ctx context.Context, namespace string, opts PolicyOpts) error {
	predicates := []interface{}{}
	if len(opts.ClusterLabels) > 0 {
		matchExpressions := []interface{}{}
		for k, v := range opts.ClusterLabels {
			matchExpressions = append(matchExpressions, map[string]interface{}{
				"key":      k,
				"operator": "In",
				"values":   []interface{}{v},
			})
		}
		predicates = append(predicates, map[string]interface{}{
			"requiredClusterSelector": map[string]interface{}{
				"labelSelector": map[string]interface{}{
					"matchExpressions": matchExpressions,
				},
			},
		})
	}

	spec := map[string]interface{}{
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
	}
	if len(predicates) > 0 {
		spec["predicates"] = predicates
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1beta1",
			"kind":       "Placement",
			"metadata": map[string]interface{}{
				"name":      opts.Name + "-placement",
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
	return m.createIfNotExists(ctx, client.GVRPlacement, namespace, obj)
}

func (m *Manager) ensurePlacementBinding(ctx context.Context, namespace string, opts PolicyOpts) error {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "policy.open-cluster-management.io/v1",
			"kind":       "PlacementBinding",
			"metadata": map[string]interface{}{
				"name":      opts.Name + "-placement-binding",
				"namespace": namespace,
			},
			"placementRef": map[string]interface{}{
				"apiGroup": "cluster.open-cluster-management.io",
				"kind":     "Placement",
				"name":     opts.Name + "-placement",
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"apiGroup": "policy.open-cluster-management.io",
					"kind":     "Policy",
					"name":     opts.Name,
				},
			},
		},
	}
	return m.createIfNotExists(ctx, client.GVRPlacementBinding, namespace, obj)
}

func buildObjectTemplates(opts PolicyOpts) []interface{} {
	if len(opts.AllowedRegistries) > 0 {
		return buildRegistryRestrictionTemplates(opts.AllowedRegistries)
	}
	return buildNamespaceTemplate(opts.Name)
}

func buildNamespaceTemplate(name string) []interface{} {
	return []interface{}{
		map[string]interface{}{
			"complianceType": "musthave",
			"objectDefinition": map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]interface{}{
					"name": name,
				},
			},
		},
	}
}

func buildRegistryRestrictionTemplates(registries []string) []interface{} {
	allowedList := make([]interface{}, len(registries))
	for i, r := range registries {
		allowedList[i] = r
	}

	return []interface{}{
		map[string]interface{}{
			"complianceType": "musthave",
			"objectDefinition": map[string]interface{}{
				"apiVersion": "config.openshift.io/v1",
				"kind":       "Image",
				"metadata": map[string]interface{}{
					"name": "cluster",
				},
				"spec": map[string]interface{}{
					"registrySources": map[string]interface{}{
						"allowedRegistries": allowedList,
					},
				},
			},
		},
	}
}
