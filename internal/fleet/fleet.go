package fleet

import (
	"context"
	"fmt"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

type Condition struct {
	Type    string
	Status  string
	Message string
}

type ClusterInfo struct {
	Name       string
	Labels     map[string]string
	Available  bool
	Joined     bool
	Accepted   bool
	Version    string
	Conditions []Condition
}

type Inspector struct {
	client *client.Client
	cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Inspector {
	return &Inspector{client: c, cfg: cfg}
}

func (i *Inspector) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	list, err := i.client.List(ctx, client.GVRManagedCluster, "", "")
	if err != nil {
		return nil, fmt.Errorf("listing managed clusters: %w", err)
	}
	clusters := make([]ClusterInfo, 0, len(list.Items))
	for _, item := range list.Items {
		clusters = append(clusters, parseClusterInfo(item.Object))
	}
	return clusters, nil
}

func (i *Inspector) GetCluster(ctx context.Context, name string) (*ClusterInfo, error) {
	obj, err := i.client.Get(ctx, client.GVRManagedCluster, "", name)
	if err != nil {
		return nil, fmt.Errorf("getting managed cluster %s: %w", name, err)
	}
	info := parseClusterInfo(obj.Object)
	return &info, nil
}

func parseClusterInfo(obj map[string]interface{}) ClusterInfo {
	info := ClusterInfo{}

	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		info.Name, _ = meta["name"].(string)
		if labels, ok := meta["labels"].(map[string]interface{}); ok {
			info.Labels = make(map[string]string, len(labels))
			for k, v := range labels {
				info.Labels[k], _ = v.(string)
			}
		}
	}

	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return info
	}

	if ver, ok := status["version"].(map[string]interface{}); ok {
		info.Version, _ = ver["kubernetes"].(string)
	}

	conditions, _ := status["conditions"].([]interface{})
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		c := Condition{}
		c.Type, _ = cond["type"].(string)
		c.Status, _ = cond["status"].(string)
		c.Message, _ = cond["message"].(string)
		info.Conditions = append(info.Conditions, c)

		switch c.Type {
		case "ManagedClusterConditionAvailable":
			info.Available = c.Status == "True"
		case "ManagedClusterJoined":
			info.Joined = c.Status == "True"
		case "HubAcceptedManagedCluster":
			info.Accepted = c.Status == "True"
		}
	}

	return info
}
