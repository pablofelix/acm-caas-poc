package provisioning

import (
	"context"
	"encoding/base64"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

const (
	acmCredentialsNamespace = "open-cluster-management"
	acmCredentialsLabel     = "cluster.open-cluster-management.io/credentials"
	acmTypeLabel            = "cluster.open-cluster-management.io/type"
)

type CentralCredential struct {
	Name       string
	Provider   string
	BaseDomain string
	APIKey     string
	PullSecret string
	SSHKey     string
}

func (m *Manager) EnsureCentralCredential(ctx context.Context, cred CentralCredential) error {
	m.logger.Info("provisioning.EnsureCentralCredential", "name", cred.Name, "provider", cred.Provider)

	secret := buildACMCredentialSecret(cred)
	existing, err := m.client.Get(ctx, client.GVRSecret, acmCredentialsNamespace, cred.Name)
	if err != nil {
		if err := m.client.CreateIfNotExists(ctx, client.GVRSecret, acmCredentialsNamespace, secret); err != nil {
			return fmt.Errorf("creating ACM credential %s: %w", cred.Name, err)
		}
		return nil
	}

	existing.Object["data"] = secret.Object["data"]
	existing.Object["metadata"].(map[string]interface{})["labels"] = secret.Object["metadata"].(map[string]interface{})["labels"]
	if _, err := m.client.Update(ctx, client.GVRSecret, acmCredentialsNamespace, existing); err != nil {
		return fmt.Errorf("updating ACM credential %s: %w", cred.Name, err)
	}
	return nil
}

func (m *Manager) GetCentralCredential(ctx context.Context, name string) (*CentralCredential, error) {
	obj, err := m.client.Get(ctx, client.GVRSecret, acmCredentialsNamespace, name)
	if err != nil {
		return nil, fmt.Errorf("getting ACM credential %s: %w", name, err)
	}
	return parseCentralCredential(obj)
}

func (m *Manager) ListCentralCredentials(ctx context.Context) ([]CentralCredential, error) {
	list, err := m.client.List(ctx, client.GVRSecret, acmCredentialsNamespace, acmCredentialsLabel)
	if err != nil {
		return nil, fmt.Errorf("listing ACM credentials: %w", err)
	}
	var creds []CentralCredential
	for _, item := range list.Items {
		cred, err := parseCentralCredential(&item)
		if err != nil {
			continue
		}
		creds = append(creds, *cred)
	}
	return creds, nil
}

func (m *Manager) DeleteCentralCredential(ctx context.Context, name string) error {
	m.logger.Info("provisioning.DeleteCentralCredential", "name", name)
	return m.client.DeleteIfExists(ctx, client.GVRSecret, acmCredentialsNamespace, name)
}

func buildACMCredentialSecret(cred CentralCredential) *unstructured.Unstructured {
	data := map[string]interface{}{}

	switch cred.Provider {
	case "ibm":
		if cred.APIKey != "" {
			data["ibmcloud_api_key"] = base64.StdEncoding.EncodeToString([]byte(cred.APIKey))
		}
	case "aws":
		if cred.APIKey != "" {
			data["aws_access_key_id"] = base64.StdEncoding.EncodeToString([]byte(cred.APIKey))
		}
	}
	if cred.BaseDomain != "" {
		data["baseDomain"] = base64.StdEncoding.EncodeToString([]byte(cred.BaseDomain))
	}
	if cred.PullSecret != "" {
		data["pullSecret"] = base64.StdEncoding.EncodeToString([]byte(cred.PullSecret))
	}
	if cred.SSHKey != "" {
		data["ssh-privatekey"] = base64.StdEncoding.EncodeToString([]byte(cred.SSHKey))
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      cred.Name,
				"namespace": acmCredentialsNamespace,
				"labels": map[string]interface{}{
					acmCredentialsLabel: "",
					acmTypeLabel:        cred.Provider,
				},
			},
			"type": "Opaque",
			"data": data,
		},
	}
}

func parseCentralCredential(obj *unstructured.Unstructured) (*CentralCredential, error) {
	cred := &CentralCredential{}

	meta, _ := obj.Object["metadata"].(map[string]interface{})
	if meta != nil {
		cred.Name, _ = meta["name"].(string)
		if labels, ok := meta["labels"].(map[string]interface{}); ok {
			cred.Provider, _ = labels[acmTypeLabel].(string)
		}
	}

	data, _ := obj.Object["data"].(map[string]interface{})
	if data == nil {
		return cred, nil
	}

	cred.BaseDomain = decodeBase64Field(data, "baseDomain")

	switch cred.Provider {
	case "ibm":
		cred.APIKey = decodeBase64Field(data, "ibmcloud_api_key")
	case "aws":
		cred.APIKey = decodeBase64Field(data, "aws_access_key_id")
	}

	return cred, nil
}

func decodeBase64Field(data map[string]interface{}, key string) string {
	val, ok := data[key].(string)
	if !ok {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return val
	}
	return string(decoded)
}
