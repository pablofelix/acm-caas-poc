package monitoring

import (
	"context"
	"fmt"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

type NodeInfo struct {
	Name         string
	Ready        bool
	CPUCapacity  string
	MemoryKi     string
	Sockets      string
	InstanceType string
	Region       string
	Zone         string
}

type ClusterResources struct {
	Name       string
	Nodes      []NodeInfo
	TotalNodes int
	ReadyNodes int
	TotalCPU   int
	Version    string
	ConsoleURL string
	Channel    string
	OCPVersion string
}

type Monitor struct {
	client *client.Client
	cfg    config.Config
}

func New(c *client.Client, cfg config.Config) *Monitor {
	return &Monitor{client: c, cfg: cfg}
}

func (m *Monitor) GetClusterResources(ctx context.Context, name string) (*ClusterResources, error) {
	obj, err := m.client.Get(ctx, client.GVRManagedClusterInfo, name, name)
	if err != nil {
		return nil, fmt.Errorf("getting ManagedClusterInfo %s: %w", name, err)
	}
	return parseClusterResources(name, obj.Object), nil
}

func (m *Monitor) ListClusterResources(ctx context.Context) ([]ClusterResources, error) {
	list, err := m.client.List(ctx, client.GVRManagedClusterInfo, "", "")
	if err != nil {
		return nil, fmt.Errorf("listing ManagedClusterInfo: %w", err)
	}

	var results []ClusterResources
	for _, item := range list.Items {
		name := item.GetName()
		cr := parseClusterResources(name, item.Object)
		results = append(results, *cr)
	}
	return results, nil
}

func parseClusterResources(name string, obj map[string]interface{}) *ClusterResources {
	cr := &ClusterResources{Name: name}

	status, _ := obj["status"].(map[string]interface{})
	if status == nil {
		return cr
	}

	cr.ConsoleURL, _ = status["consoleURL"].(string)

	if version, ok := status["version"].(string); ok {
		cr.Version = version
	}

	if distInfo, ok := status["distributionInfo"].(map[string]interface{}); ok {
		if ocp, ok := distInfo["ocp"].(map[string]interface{}); ok {
			cr.Channel, _ = ocp["channel"].(string)
			cr.OCPVersion, _ = ocp["version"].(string)
		}
	}

	nodeList, _ := status["nodeList"].([]interface{})
	for _, n := range nodeList {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		ni := NodeInfo{}
		ni.Name, _ = node["name"].(string)

		if capacity, ok := node["capacity"].(map[string]interface{}); ok {
			ni.CPUCapacity, _ = capacity["cpu"].(string)
			ni.MemoryKi, _ = capacity["memory"].(string)
			ni.Sockets, _ = capacity["socket"].(string)
		}

		if labels, ok := node["labels"].(map[string]interface{}); ok {
			ni.InstanceType, _ = labels["node.kubernetes.io/instance-type"].(string)
			ni.Region, _ = labels["topology.kubernetes.io/region"].(string)
			ni.Zone, _ = labels["topology.kubernetes.io/zone"].(string)
		}

		ni.Ready = isNodeReady(node)
		cr.Nodes = append(cr.Nodes, ni)

		if ni.Ready {
			cr.ReadyNodes++
		}
	}
	cr.TotalNodes = len(cr.Nodes)

	cpu := 0
	for _, n := range cr.Nodes {
		if n.CPUCapacity != "" {
			var c int
			fmt.Sscanf(n.CPUCapacity, "%d", &c)
			cpu += c
		}
	}
	cr.TotalCPU = cpu

	return cr
}

func isNodeReady(node map[string]interface{}) bool {
	conditions, _ := node["conditions"].([]interface{})
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "Ready" && cond["status"] == "True" {
			return true
		}
	}
	return false
}
