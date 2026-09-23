package provisioning

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func fakeClientForCreds(objs ...runtime.Object) *client.Client {
	scheme := runtime.NewScheme()
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			client.GVRSecret: "SecretList",
		}, objs...)
	return &client.Client{Dynamic: fake}
}

func TestEnsureCentralCredentialCreates(t *testing.T) {
	c := fakeClientForCreds()
	m := New(c, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	cred := CentralCredential{
		Name:       "ibm-prod-creds",
		Provider:   "ibm",
		BaseDomain: "example.com",
		APIKey:     "test-api-key",
		PullSecret: `{"auths":{}}`,
	}

	err := m.EnsureCentralCredential(context.Background(), cred)
	if err != nil {
		t.Fatalf("EnsureCentralCredential failed: %v", err)
	}

	obj, err := c.Get(context.Background(), client.GVRSecret, acmCredentialsNamespace, "ibm-prod-creds")
	if err != nil {
		t.Fatalf("credential not found: %v", err)
	}

	labels := obj.GetLabels()
	if labels[acmCredentialsLabel] != "" {
		t.Errorf("missing credentials label")
	}
	if labels[acmTypeLabel] != "ibm" {
		t.Errorf("type label = %q, want %q", labels[acmTypeLabel], "ibm")
	}
}

func TestEnsureCentralCredentialUpdates(t *testing.T) {
	existing := buildACMCredentialSecret(CentralCredential{
		Name:     "ibm-prod-creds",
		Provider: "ibm",
		APIKey:   "old-key",
	})
	c := fakeClientForCreds(existing)
	m := New(c, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := m.EnsureCentralCredential(context.Background(), CentralCredential{
		Name:     "ibm-prod-creds",
		Provider: "ibm",
		APIKey:   "new-key",
	})
	if err != nil {
		t.Fatalf("EnsureCentralCredential update failed: %v", err)
	}

	obj, err := c.Get(context.Background(), client.GVRSecret, acmCredentialsNamespace, "ibm-prod-creds")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := parseCentralCredential(obj)
	if parsed.APIKey != "new-key" {
		t.Errorf("APIKey = %q, want %q", parsed.APIKey, "new-key")
	}
}

func TestListCentralCredentials(t *testing.T) {
	s1 := buildACMCredentialSecret(CentralCredential{Name: "ibm-creds", Provider: "ibm", APIKey: "key1"})
	s2 := buildACMCredentialSecret(CentralCredential{Name: "aws-creds", Provider: "aws", APIKey: "key2"})
	c := fakeClientForCreds(s1, s2)
	m := New(c, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	creds, err := m.ListCentralCredentials(context.Background())
	if err != nil {
		t.Fatalf("ListCentralCredentials failed: %v", err)
	}
	if len(creds) != 2 {
		t.Errorf("got %d credentials, want 2", len(creds))
	}
}

func TestDeleteCentralCredential(t *testing.T) {
	s := buildACMCredentialSecret(CentralCredential{Name: "to-delete", Provider: "ibm"})
	c := fakeClientForCreds(s)
	m := New(c, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := m.DeleteCentralCredential(context.Background(), "to-delete")
	if err != nil {
		t.Fatalf("DeleteCentralCredential failed: %v", err)
	}

	_, err = c.Get(context.Background(), client.GVRSecret, acmCredentialsNamespace, "to-delete")
	if err == nil {
		t.Error("credential should be deleted")
	}
}

func TestGetCentralCredential(t *testing.T) {
	s := buildACMCredentialSecret(CentralCredential{Name: "ibm-get", Provider: "ibm", APIKey: "get-key", BaseDomain: "test.com"})
	c := fakeClientForCreds(s)
	m := New(c, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	cred, err := m.GetCentralCredential(context.Background(), "ibm-get")
	if err != nil {
		t.Fatalf("GetCentralCredential failed: %v", err)
	}
	if cred.Provider != "ibm" {
		t.Errorf("Provider = %q, want %q", cred.Provider, "ibm")
	}
	if cred.APIKey != "get-key" {
		t.Errorf("APIKey = %q, want %q", cred.APIKey, "get-key")
	}
	if cred.BaseDomain != "test.com" {
		t.Errorf("BaseDomain = %q, want %q", cred.BaseDomain, "test.com")
	}
}

func TestGetCentralCredentialNotFound(t *testing.T) {
	c := fakeClientForCreds()
	m := New(c, config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := m.GetCentralCredential(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent credential, got nil")
	}
}

func TestBuildACMCredentialSecretIBM(t *testing.T) {
	secret := buildACMCredentialSecret(CentralCredential{
		Name:       "test-ibm",
		Provider:   "ibm",
		APIKey:     "my-api-key",
		BaseDomain: "test.example.com",
		PullSecret: `{"auths":{}}`,
		SSHKey:     "ssh-rsa AAAA",
	})

	if secret.GetNamespace() != acmCredentialsNamespace {
		t.Errorf("namespace = %q, want %q", secret.GetNamespace(), acmCredentialsNamespace)
	}

	data, _ := secret.Object["data"].(map[string]interface{})
	if data["ibmcloud_api_key"] == nil {
		t.Error("missing ibmcloud_api_key in data")
	}
	if data["baseDomain"] == nil {
		t.Error("missing baseDomain in data")
	}
	if data["pullSecret"] == nil {
		t.Error("missing pullSecret in data")
	}
	if data["ssh-privatekey"] == nil {
		t.Error("missing ssh-privatekey in data")
	}
}

func TestDecodeBase64Field(t *testing.T) {
	data := map[string]interface{}{
		"key1": "dGVzdA==",
		"key2": 123,
	}
	if got := decodeBase64Field(data, "key1"); got != "test" {
		t.Errorf("decodeBase64Field(key1) = %q, want %q", got, "test")
	}
	if got := decodeBase64Field(data, "key2"); got != "" {
		t.Errorf("decodeBase64Field(key2) = %q, want empty", got)
	}
	if got := decodeBase64Field(data, "missing"); got != "" {
		t.Errorf("decodeBase64Field(missing) = %q, want empty", got)
	}
}
