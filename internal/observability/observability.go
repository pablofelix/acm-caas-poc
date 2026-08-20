package observability

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

const (
	Namespace      = "open-cluster-management-observability"
	MinIOName      = "minio"
	MinIOPort      = 9000
	ThanosCfgKey   = "thanos.yaml"
	SecretName     = "thanos-object-storage"
	MCOName        = "observability"
	StorageClass   = "ibmc-vpc-block-10iops-tier"
	MinIOPVCSize   = "20Gi"
	MinIOAccessKey = "minio"
	MinIOSecretKey = "minio123"
	MinioBucket    = "thanos"
)

type Manager struct {
	client *client.Client
	cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Manager {
	return &Manager{client: c, cfg: cfg}
}

func (m *Manager) Setup(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"namespace", m.ensureNamespace},
		{"minio-pvc", m.ensureMinioPVC},
		{"minio-deployment", m.ensureMinioDeployment},
		{"minio-service", m.ensureMinioService},
		{"thanos-secret", m.ensureThanosSecret},
		{"multiclusterobservability", m.ensureMCO},
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("setup %s: %w", s.name, err)
		}
	}
	return nil
}

func (m *Manager) Teardown(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"multiclusterobservability", m.deleteMCO},
		{"thanos-secret", m.deleteSecret},
		{"minio-service", m.deleteMinioService},
		{"minio-deployment", m.deleteMinioDeployment},
		{"minio-pvc", m.deleteMinioPVC},
		{"namespace", m.deleteNamespace},
	}
	for _, s := range steps {
		if err := s.fn(ctx); err != nil {
			return fmt.Errorf("teardown %s: %w", s.name, err)
		}
	}
	return nil
}

func (m *Manager) Status(ctx context.Context) (string, error) {
	obj, err := m.client.Get(ctx, client.GVRMultiClusterObservability, "", MCOName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "NotInstalled", nil
		}
		return "", fmt.Errorf("getting MCO: %w", err)
	}
	status, _ := obj.Object["status"].(map[string]interface{})
	if status == nil {
		return "Pending", nil
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "Ready" && cond["status"] == "True" {
			return "Ready", nil
		}
	}
	return "Progressing", nil
}

func (m *Manager) createIfNotExists(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
	_, err := m.client.Get(ctx, gvr, namespace, obj.GetName())
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	_, err = m.client.Create(ctx, gvr, namespace, obj)
	return err
}

func (m *Manager) deleteIfExists(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	err := m.client.Delete(ctx, gvr, namespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
