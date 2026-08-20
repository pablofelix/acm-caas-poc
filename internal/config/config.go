package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Kubeconfig string
	HubContext string

	IBMCloudAPIKey string
	IBMCloudRegion string

	BaseDomain          string
	ClusterImageSet     string
	DefaultWorkerType   string
	DefaultMasterType   string
	DefaultWorkerReplicas int
	DefaultMasterReplicas int

	ProvisionTimeout time.Duration
	OperationTimeout time.Duration

	MCPLogLevel string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		Kubeconfig:     envOr("KUBECONFIG", ""),
		HubContext:     envOr("ACM_HUB_CONTEXT", ""),
		IBMCloudAPIKey: envOr("IBMCLOUD_API_KEY", ""),
		IBMCloudRegion: envOr("IBMCLOUD_REGION", "us-south"),
		BaseDomain:     envOr("ACM_BASE_DOMAIN", "infraops1.ibm.rh-ods.com"),
		ClusterImageSet: envOr("ACM_CLUSTER_IMAGE_SET", "img4.22.9-multi-appsub"),
		DefaultWorkerType: envOr("ACM_DEFAULT_WORKER_TYPE", "bx2-4x16"),
		DefaultMasterType: envOr("ACM_DEFAULT_MASTER_TYPE", "bx2-8x32"),
		MCPLogLevel:       envOr("ACM_MCP_LOG_LEVEL", "info"),
	}

	var err error
	cfg.DefaultWorkerReplicas, err = envInt("ACM_DEFAULT_WORKER_REPLICAS", 2)
	if err != nil {
		return cfg, fmt.Errorf("parsing ACM_DEFAULT_WORKER_REPLICAS: %w", err)
	}
	cfg.DefaultMasterReplicas, err = envInt("ACM_DEFAULT_MASTER_REPLICAS", 3)
	if err != nil {
		return cfg, fmt.Errorf("parsing ACM_DEFAULT_MASTER_REPLICAS: %w", err)
	}
	cfg.ProvisionTimeout, err = envDuration("ACM_PROVISION_TIMEOUT", 45*time.Minute)
	if err != nil {
		return cfg, fmt.Errorf("parsing ACM_PROVISION_TIMEOUT: %w", err)
	}
	cfg.OperationTimeout, err = envDuration("ACM_OPERATION_TIMEOUT", 5*time.Minute)
	if err != nil {
		return cfg, fmt.Errorf("parsing ACM_OPERATION_TIMEOUT: %w", err)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}
