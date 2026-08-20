package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnvReadsAllFields(t *testing.T) {
	env := map[string]string{
		"KUBECONFIG":                   "~/.kube/test",
		"ACM_HUB_CONTEXT":             "test-hub",
		"IBMCLOUD_API_KEY":            "test-key",
		"IBMCLOUD_REGION":             "eu-de",
		"ACM_BASE_DOMAIN":             "test.example.com",
		"ACM_CLUSTER_IMAGE_SET":       "img4.22.0",
		"ACM_DEFAULT_WORKER_TYPE":     "bx2-8x32",
		"ACM_DEFAULT_MASTER_TYPE":     "bx2-16x64",
		"ACM_DEFAULT_WORKER_REPLICAS": "3",
		"ACM_DEFAULT_MASTER_REPLICAS": "5",
		"ACM_PROVISION_TIMEOUT":       "30m",
		"ACM_OPERATION_TIMEOUT":       "10m",
		"ACM_MCP_LOG_LEVEL":           "debug",
	}
	for k, v := range env {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IBMCloudRegion != "eu-de" {
		t.Errorf("IBMCloudRegion = %q, want %q", cfg.IBMCloudRegion, "eu-de")
	}
	if cfg.DefaultWorkerReplicas != 3 {
		t.Errorf("DefaultWorkerReplicas = %d, want 3", cfg.DefaultWorkerReplicas)
	}
	if cfg.ProvisionTimeout != 30*time.Minute {
		t.Errorf("ProvisionTimeout = %v, want 30m", cfg.ProvisionTimeout)
	}
}

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	for _, k := range []string{
		"IBMCLOUD_REGION", "ACM_BASE_DOMAIN", "ACM_CLUSTER_IMAGE_SET",
		"ACM_DEFAULT_WORKER_TYPE", "ACM_DEFAULT_MASTER_TYPE",
		"ACM_DEFAULT_WORKER_REPLICAS", "ACM_DEFAULT_MASTER_REPLICAS",
		"ACM_PROVISION_TIMEOUT", "ACM_OPERATION_TIMEOUT",
	} {
		os.Unsetenv(k)
	}

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IBMCloudRegion != "us-south" {
		t.Errorf("default IBMCloudRegion = %q, want %q", cfg.IBMCloudRegion, "us-south")
	}
	if cfg.DefaultWorkerReplicas != 2 {
		t.Errorf("default DefaultWorkerReplicas = %d, want 2", cfg.DefaultWorkerReplicas)
	}
	if cfg.ProvisionTimeout != 45*time.Minute {
		t.Errorf("default ProvisionTimeout = %v, want 45m", cfg.ProvisionTimeout)
	}
}

func TestLoadFromEnvReturnsErrorForInvalidInt(t *testing.T) {
	os.Setenv("ACM_DEFAULT_WORKER_REPLICAS", "not-a-number")
	defer os.Unsetenv("ACM_DEFAULT_WORKER_REPLICAS")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("expected error for invalid int, got nil")
	}
}

func TestLoadFromEnvReturnsErrorForInvalidDuration(t *testing.T) {
	os.Setenv("ACM_PROVISION_TIMEOUT", "not-a-duration")
	defer os.Unsetenv("ACM_PROVISION_TIMEOUT")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("expected error for invalid duration, got nil")
	}
}
