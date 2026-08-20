package observability

import (
	"context"
	"encoding/base64"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

func (m *Manager) ensureNamespace(ctx context.Context) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(client.GVRNamespace.GroupVersion().WithKind("Namespace"))
	obj.SetName(Namespace)
	return m.createIfNotExists(ctx, client.GVRNamespace, "", obj)
}

func (m *Manager) deleteNamespace(ctx context.Context) error {
	return m.deleteIfExists(ctx, client.GVRNamespace, "", Namespace)
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
	return m.createIfNotExists(ctx, client.GVRPersistentVolumeClaim, Namespace, obj)
}

func (m *Manager) deleteMinioPVC(ctx context.Context) error {
	return m.deleteIfExists(ctx, client.GVRPersistentVolumeClaim, Namespace, MinIOName)
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
								"args":  []interface{}{"server", "/data"},
								"env": []interface{}{
									map[string]interface{}{
										"name":  "MINIO_ROOT_USER",
										"value": MinIOAccessKey,
									},
									map[string]interface{}{
										"name":  "MINIO_ROOT_PASSWORD",
										"value": MinIOSecretKey,
									},
									map[string]interface{}{
										"name":  "MINIO_DEFAULT_BUCKETS",
										"value": MinioBucket,
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
	return m.createIfNotExists(ctx, client.GVRDeployment, Namespace, obj)
}

func (m *Manager) deleteMinioDeployment(ctx context.Context) error {
	return m.deleteIfExists(ctx, client.GVRDeployment, Namespace, MinIOName)
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
	return m.createIfNotExists(ctx, client.GVRService, Namespace, obj)
}

func (m *Manager) deleteMinioService(ctx context.Context) error {
	return m.deleteIfExists(ctx, client.GVRService, Namespace, MinIOName)
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
	return m.createIfNotExists(ctx, client.GVRSecret, Namespace, obj)
}

func (m *Manager) deleteSecret(ctx context.Context) error {
	return m.deleteIfExists(ctx, client.GVRSecret, Namespace, SecretName)
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
	return m.createIfNotExists(ctx, client.GVRMultiClusterObservability, "", obj)
}

func (m *Manager) deleteMCO(ctx context.Context) error {
	return m.deleteIfExists(ctx, client.GVRMultiClusterObservability, "", MCOName)
}
