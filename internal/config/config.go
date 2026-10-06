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

	Platform       string
	IBMCloudAPIKey string
	IBMCloudRegion string
	AWSRegion     string
	AWSBaseDomain string

	BaseDomain          string
	ClusterImageSet     string
	DefaultWorkerType   string
	DefaultMasterType   string
	DefaultWorkerReplicas int
	DefaultMasterReplicas int

	ProvisionTimeout time.Duration
	OperationTimeout time.Duration

	MCPLogLevel string

	ClusterSpoke1 string
	ClusterSpoke2 string
	ClusterHub    string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		Kubeconfig:     envOr("KUBECONFIG", ""),
		HubContext:     envOr("ACM_HUB_CONTEXT", ""),
		Platform:       envOr("ACM_PLATFORM", "ibmcloud"),
		IBMCloudAPIKey: envOr("IBMCLOUD_API_KEY", ""),
		IBMCloudRegion: envOr("IBMCLOUD_REGION", "us-south"),
		AWSRegion:      envOr("AWS_REGION", "us-east-1"),
		AWSBaseDomain:  envOr("ACM_AWS_BASE_DOMAIN", ""),
		BaseDomain:     envOr("ACM_BASE_DOMAIN", ""),
		ClusterImageSet: envOr("ACM_CLUSTER_IMAGE_SET", "img4.22.9-multi-appsub"),
		DefaultWorkerType: envOr("ACM_DEFAULT_WORKER_TYPE", "bx2-4x16"),
		DefaultMasterType: envOr("ACM_DEFAULT_MASTER_TYPE", "bx2-8x32"),
		MCPLogLevel:       envOr("ACM_MCP_LOG_LEVEL", "info"),
		ClusterSpoke1:     envOr("ACM_CLUSTER_SPOKE1", "spoke1"),
		ClusterSpoke2:     envOr("ACM_CLUSTER_SPOKE2", "spoke2"),
		ClusterHub:        envOr("ACM_CLUSTER_HUB", "infraops1"),
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

func (c Config) ResolveCluster(name string) string {
	switch name {
	case "spoke1":
		return c.ClusterSpoke1
	case "spoke2":
		return c.ClusterSpoke2
	case "infraops1":
		return c.ClusterHub
	default:
		return name
	}
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
