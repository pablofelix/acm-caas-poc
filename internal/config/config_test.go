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

func TestLoadFromEnvReadsAWSFields(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("ACM_AWS_BASE_DOMAIN", "my-zone.example.com")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AWSRegion != "eu-west-1" {
		t.Errorf("AWSRegion = %q, want %q", cfg.AWSRegion, "eu-west-1")
	}
	if cfg.AWSBaseDomain != "my-zone.example.com" {
		t.Errorf("AWSBaseDomain = %q, want %q", cfg.AWSBaseDomain, "my-zone.example.com")
	}
}

func TestLoadFromEnvAWSRegionDefault(t *testing.T) {
	t.Setenv("AWS_REGION", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AWSRegion != "us-east-1" {
		t.Errorf("default AWSRegion = %q, want %q", cfg.AWSRegion, "us-east-1")
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

func TestResolveCluster(t *testing.T) {
	cfg := Config{
		ClusterSpoke1: "my-spoke1",
		ClusterSpoke2: "my-spoke2",
		ClusterHub:    "my-hub",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"spoke1", "my-spoke1"},
		{"spoke2", "my-spoke2"},
		{"infraops1", "my-hub"},
		{"other-cluster", "other-cluster"},
	}

	for _, tt := range tests {
		got := cfg.ResolveCluster(tt.input)
		if got != tt.want {
			t.Errorf("ResolveCluster(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadFromEnvReadsClusterMapping(t *testing.T) {
	t.Setenv("ACM_CLUSTER_SPOKE1", "custom-spoke1")
	t.Setenv("ACM_CLUSTER_SPOKE2", "custom-spoke2")
	t.Setenv("ACM_CLUSTER_HUB", "custom-hub")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClusterSpoke1 != "custom-spoke1" {
		t.Errorf("ClusterSpoke1 = %q, want %q", cfg.ClusterSpoke1, "custom-spoke1")
	}
	if cfg.ClusterSpoke2 != "custom-spoke2" {
		t.Errorf("ClusterSpoke2 = %q, want %q", cfg.ClusterSpoke2, "custom-spoke2")
	}
	if cfg.ClusterHub != "custom-hub" {
		t.Errorf("ClusterHub = %q, want %q", cfg.ClusterHub, "custom-hub")
	}
}

