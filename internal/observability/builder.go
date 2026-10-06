package observability

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

func (m *Manager) ensureNamespace(ctx context.Context) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(client.GVRNamespace.GroupVersion().WithKind("Namespace"))
	obj.SetName(Namespace)
	return m.client.CreateIfNotExists(ctx, client.GVRNamespace, "", obj)
}

func (m *Manager) deleteNamespace(ctx context.Context) error {
	return m.client.DeleteIfExists(ctx, client.GVRNamespace, "", Namespace)
}

func (m *Manager) ensureMinioPVC(ctx context.Context) error {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name":      MinIOName,
				"namespace": Namespace,
			},
			"spec": map[string]interface{}{
				"accessModes":      []interface{}{"ReadWriteOnce"},
				"storageClassName": StorageClass,
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{
						"storage": MinIOPVCSize,
					},
				},
			},
		},
	}
	return m.client.CreateIfNotExists(ctx, client.GVRPersistentVolumeClaim, Namespace, obj)
}

func (m *Manager) deleteMinioPVC(ctx context.Context) error {
	return m.client.DeleteIfExists(ctx, client.GVRPersistentVolumeClaim, Namespace, MinIOName)
}

func (m *Manager) ensureMinioDeployment(ctx context.Context) error {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      MinIOName,
				"namespace": Namespace,
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": MinIOName,
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": MinIOName,
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  MinIOName,
								"image": "quay.io/minio/minio:latest",
								"command": []interface{}{"/bin/sh", "-c"},
								"args": []interface{}{
									fmt.Sprintf("minio server /data &\nuntil mc alias set local http://localhost:%d %s %s 2>/dev/null; do sleep 1; done\nmc mb --ignore-existing local/%s\nwait",
										MinIOPort, MinIOAccessKey, MinIOSecretKey, MinioBucket),
								},
								"env": []interface{}{
									map[string]interface{}{
										"name":  "MINIO_ROOT_USER",
										"value": MinIOAccessKey,
									},
									map[string]interface{}{
										"name":  "MINIO_ROOT_PASSWORD",
										"value": MinIOSecretKey,
									},
								},
								"ports": []interface{}{
									map[string]interface{}{
										"containerPort": int64(MinIOPort),
									},
								},
								"volumeMounts": []interface{}{
									map[string]interface{}{
										"name":      "data",
										"mountPath": "/data",
									},
								},
							},
						},
						"volumes": []interface{}{
							map[string]interface{}{
								"name": "data",
								"persistentVolumeClaim": map[string]interface{}{
									"claimName": MinIOName,
								},
							},
						},
					},
				},
			},
		},
	}
	return m.client.CreateIfNotExists(ctx, client.GVRDeployment, Namespace, obj)
}

func (m *Manager) deleteMinioDeployment(ctx context.Context) error {
	return m.client.DeleteIfExists(ctx, client.GVRDeployment, Namespace, MinIOName)
}

func (m *Manager) ensureMinioService(ctx context.Context) error {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      MinIOName,
				"namespace": Namespace,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"app": MinIOName,
				},
				"ports": []interface{}{
					map[string]interface{}{
						"port":       int64(MinIOPort),
						"targetPort": int64(MinIOPort),
					},
				},
			},
		},
	}
	return m.client.CreateIfNotExists(ctx, client.GVRService, Namespace, obj)
}

func (m *Manager) deleteMinioService(ctx context.Context) error {
	return m.client.DeleteIfExists(ctx, client.GVRService, Namespace, MinIOName)
}

func (m *Manager) ensureThanosSecret(ctx context.Context) error {
	thanosYAML := fmt.Sprintf(`type: s3
config:
  bucket: %s
  endpoint: %s.%s.svc.cluster.local:%d
  insecure: true
  access_key: %s
  secret_key: %s
`, MinioBucket, MinIOName, Namespace, MinIOPort, MinIOAccessKey, MinIOSecretKey)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      SecretName,
				"namespace": Namespace,
			},
			"type": "Opaque",
			"data": map[string]interface{}{
				ThanosCfgKey: base64.StdEncoding.EncodeToString([]byte(thanosYAML)),
			},
		},
	}
	return m.client.CreateIfNotExists(ctx, client.GVRSecret, Namespace, obj)
}

func (m *Manager) deleteSecret(ctx context.Context) error {
	return m.client.DeleteIfExists(ctx, client.GVRSecret, Namespace, SecretName)
}

func (m *Manager) ensureMCO(ctx context.Context) error {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "observability.open-cluster-management.io/v1beta2",
			"kind":       "MultiClusterObservability",
			"metadata": map[string]interface{}{
				"name": MCOName,
			},
			"spec": map[string]interface{}{
				"instanceSize":       "minimal",
				"enableDownsampling": false,
				"observabilityAddonSpec": map[string]interface{}{
					"enableMetrics": true,
					"interval":      int64(300),
				},
				"storageConfig": map[string]interface{}{
					"metricObjectStorage": map[string]interface{}{
						"name": SecretName,
						"key":  ThanosCfgKey,
					},
					"storageClass": StorageClass,
				},
			},
		},
	}
	return m.client.CreateIfNotExists(ctx, client.GVRMultiClusterObservability, "", obj)
}

func (m *Manager) deleteMCO(ctx context.Context) error {
	return m.client.DeleteIfExists(ctx, client.GVRMultiClusterObservability, "", MCOName)
}

func buildOBC(name, ns, storageClass, bucketName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "objectbucket.io/v1alpha1",
			"kind":       "ObjectBucketClaim",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
			},
			"spec": map[string]interface{}{
				"generateBucketName": bucketName,
				"storageClassName":   storageClass,
			},
		},
	}
}

func buildCustomRulesConfigMap(ns, rules string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      CustomRulesCM,
				"namespace": ns,
			},
			"data": map[string]interface{}{
				"custom_rules.yaml": rules,
			},
		},
	}
}

func buildDashboardConfigMap(ns, name, dashJSON string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
				"labels": map[string]interface{}{
					DashboardLabelKey: DashboardLabelValue,
				},
			},
			"data": map[string]interface{}{
				name + ".json": dashJSON,
			},
		},
	}
}


func metricsToYAML(metrics []string) string {
	var sb strings.Builder
	for _, m := range metrics {
		sb.WriteString("  - " + m + "\n")
	}
	return sb.String()
}

