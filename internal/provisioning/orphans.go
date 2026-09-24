package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type OrphanedResource struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
}

type OrphanCheckResult struct {
	InfraID  string             `json:"infra_id"`
	Platform string             `json:"platform"`
	Region   string             `json:"region"`
	Orphans  []OrphanedResource `json:"orphans"`
	Clean    bool               `json:"clean"`
}

// CaptureInfraID reads the infraID from a ClusterDeployment before it is deleted.
func (m *Manager) CaptureInfraID(ctx context.Context, name string) (infraID, platform, region string, err error) {
	obj, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err != nil {
		return "", "", "", fmt.Errorf("getting ClusterDeployment %s: %w", name, err)
	}

	infraID, _, _ = unstructured.NestedString(obj.Object, "spec", "clusterMetadata", "infraID")
	if infraID == "" {
		infraID = name
	}

	spec, _, _ := unstructured.NestedMap(obj.Object, "spec", "platform")
	for p := range spec {
		platform = p
		platformMap, ok := spec[p].(map[string]interface{})
		if ok {
			region, _ = platformMap["region"].(string)
		}
		break
	}

	return infraID, platform, region, nil
}

// CheckOrphans queries the cloud provider for resources matching the infraID.
func (m *Manager) CheckOrphans(ctx context.Context, infraID, platform, region string) (*OrphanCheckResult, error) {
	m.logger.Info("provisioning.CheckOrphans", "infraID", infraID, "platform", platform, "region", region)

	result := &OrphanCheckResult{
		InfraID:  infraID,
		Platform: platform,
		Region:   region,
	}

	switch platform {
	case "aws":
		orphans, err := m.checkAWSOrphans(infraID, region)
		if err != nil {
			return nil, err
		}
		result.Orphans = orphans
	case "ibmcloud":
		orphans, err := m.checkIBMCloudOrphans(infraID, region)
		if err != nil {
			return nil, err
		}
		result.Orphans = orphans
	default:
		return nil, fmt.Errorf("orphan check: unsupported platform %q", platform)
	}

	result.Clean = len(result.Orphans) == 0
	return result, nil
}

// DestroyWithOrphanCheck captures infraID, destroys, waits for Hive, then checks for orphans.
func (m *Manager) DestroyWithOrphanCheck(ctx context.Context, name string) (*OrphanCheckResult, error) {
	infraID, platform, region, err := m.CaptureInfraID(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("capturing infraID: %w", err)
	}

	m.logger.Info("provisioning.DestroyWithOrphanCheck",
		"cluster", name, "infraID", infraID, "platform", platform, "region", region)

	if err := m.Destroy(ctx, name); err != nil {
		return nil, err
	}

	// Wait for ClusterDeployment to be fully removed by Hive
	timeout := 30 * time.Minute
	interval := 15 * time.Second
	deadline := time.After(timeout)
	for {
		_, err := m.client.Get(ctx, client.GVRClusterDeployment, name, name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return nil, fmt.Errorf("checking ClusterDeployment %s removal: %w", name, err)
		}
		select {
		case <-deadline:
			return nil, fmt.Errorf("ClusterDeployment %s not removed within %v — deprovision may be stuck", name, timeout)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}

	return m.CheckOrphans(ctx, infraID, platform, region)
}

func (m *Manager) runAWS(args ...string) ([]byte, error) {
	if m.awsExec != nil {
		return m.awsExec(args...)
	}
	return exec.Command("aws", args...).CombinedOutput()
}

func (m *Manager) checkAWSOrphans(infraID, region string) ([]OrphanedResource, error) {
	tag := fmt.Sprintf("kubernetes.io/cluster/%s", infraID)
	var orphans []OrphanedResource

	out, err := m.runAWS("ec2", "describe-instances",
		"--filters", fmt.Sprintf("Name=tag:%s,Values=owned", tag),
		"--query", "Reservations[].Instances[].[InstanceId,State.Name,Tags[?Key==`Name`].Value|[0]]",
		"--region", region, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("aws ec2 describe-instances: %w", err)
	}
	var instances [][]interface{}
	if err := json.Unmarshal(out, &instances); err != nil {
		return nil, fmt.Errorf("aws ec2 describe-instances: invalid JSON: %w", err)
	}
	for _, inst := range instances {
		if len(inst) >= 2 {
			state, _ := inst[1].(string)
			if state != "terminated" {
				id, _ := inst[0].(string)
				name := ""
				if len(inst) >= 3 {
					name, _ = inst[2].(string)
				}
				orphans = append(orphans, OrphanedResource{
					Type: "ec2-instance", Name: name, ID: id, Status: state,
				})
			}
		}
	}

	out, err = m.runAWS("ec2", "describe-vpcs",
		"--filters", fmt.Sprintf("Name=tag:%s,Values=owned", tag),
		"--query", "Vpcs[].[VpcId,State]",
		"--region", region, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("aws ec2 describe-vpcs: %w", err)
	}
	var vpcs [][]interface{}
	if err := json.Unmarshal(out, &vpcs); err != nil {
		return nil, fmt.Errorf("aws ec2 describe-vpcs: invalid JSON: %w", err)
	}
	for _, vpc := range vpcs {
		if len(vpc) >= 1 {
			id, _ := vpc[0].(string)
			orphans = append(orphans, OrphanedResource{
				Type: "vpc", ID: id,
			})
		}
	}

	out, err = m.runAWS("elbv2", "describe-load-balancers",
		"--region", region, "--output", "json")
	if err != nil {
		return nil, fmt.Errorf("aws elbv2 describe-load-balancers: %w", err)
	}
	var resp struct {
		LoadBalancers []struct {
			ARN  string `json:"LoadBalancerArn"`
			Name string `json:"LoadBalancerName"`
		} `json:"LoadBalancers"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("aws elbv2 describe-load-balancers: invalid JSON: %w", err)
	}
	for _, lb := range resp.LoadBalancers {
		if strings.Contains(lb.Name, infraID) {
			orphans = append(orphans, OrphanedResource{
				Type: "load-balancer", Name: lb.Name, ID: lb.ARN,
			})
		}
	}

	return orphans, nil
}

func (m *Manager) checkIBMCloudOrphans(infraID, region string) ([]OrphanedResource, error) {
	iamToken := ""
	if m.cfg.IBMCloudAPIKey != "" {
		httpClient := &http.Client{Timeout: 10 * time.Second}
		body := strings.NewReader("grant_type=urn:ibm:params:oauth:grant-type:apikey&apikey=" + m.cfg.IBMCloudAPIKey)
		req, _ := http.NewRequest("POST", m.ibmIAMURL()+"/identity/token", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("IBM Cloud IAM auth: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var token struct {
			AccessToken string `json:"access_token"`
		}
		json.Unmarshal(respBody, &token)
		iamToken = token.AccessToken
	}
	if iamToken == "" {
		return nil, fmt.Errorf("no IBM Cloud credentials available for orphan check")
	}

	baseURL := fmt.Sprintf("%s/v1", m.ibmVPCURL(region))
	return queryIBMCloudOrphans(&http.Client{Timeout: 15 * time.Second}, iamToken, baseURL, infraID)
}

func queryIBMCloudOrphans(httpClient *http.Client, iamToken, baseURL, infraID string) ([]OrphanedResource, error) {
	version := "2024-06-04"
	var orphans []OrphanedResource

	type query struct {
		url          string
		resourceType string
	}
	queries := []query{
		{fmt.Sprintf("%s/instances?version=%s&generation=2&limit=100", baseURL, version), "instance"},
		{fmt.Sprintf("%s/load_balancers?version=%s&generation=2", baseURL, version), "load-balancer"},
		{fmt.Sprintf("%s/subnets?version=%s&generation=2", baseURL, version), "subnet"},
		{fmt.Sprintf("%s/vpcs?version=%s&generation=2", baseURL, version), "vpc"},
		{fmt.Sprintf("%s/floating_ips?version=%s&generation=2", baseURL, version), "floating-ip"},
	}

	for _, q := range queries {
		results, err := queryIBMCloudVPC(httpClient, iamToken, q.url, infraID, q.resourceType)
		if err != nil {
			return nil, fmt.Errorf("ibmcloud %s query: %w", q.resourceType, err)
		}
		orphans = append(orphans, results...)
	}

	return orphans, nil
}

func queryIBMCloudVPC(httpClient *http.Client, token, url, infraID, resourceType string) ([]OrphanedResource, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request for %s: %w", resourceType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP %d querying %s", resp.StatusCode, resourceType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response for %s: %w", resourceType, err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON for %s: %w", resourceType, err)
	}

	var items []struct {
		Name   string `json:"name"`
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	for _, key := range []string{"instances", "load_balancers", "subnets", "vpcs", "floating_ips"} {
		if data, ok := raw[key]; ok {
			if err := json.Unmarshal(data, &items); err != nil {
				return nil, fmt.Errorf("invalid JSON items for %s: %w", resourceType, err)
			}
			break
		}
	}

	var orphans []OrphanedResource
	for _, item := range items {
		if strings.HasPrefix(item.Name, infraID) {
			orphans = append(orphans, OrphanedResource{
				Type:   resourceType,
				Name:   item.Name,
				ID:     item.ID,
				Status: item.Status,
			})
		}
	}
	return orphans, nil
}

func FormatOrphanCheckResult(r *OrphanCheckResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  ┌─ Orphan check for infraID=%s (platform=%s, region=%s)\n", r.InfraID, r.Platform, r.Region))

	if r.Clean {
		b.WriteString("  │  No orphaned resources found\n")
		b.WriteString("  └─ Clean\n")
		return b.String()
	}

	for _, o := range r.Orphans {
		status := ""
		if o.Status != "" {
			status = fmt.Sprintf(" [%s]", o.Status)
		}
		b.WriteString(fmt.Sprintf("  │  %-15s %-40s %s%s\n", o.Type, o.Name, o.ID, status))
	}
	b.WriteString(fmt.Sprintf("  └─ %d orphaned resources found — manual cleanup required\n", len(r.Orphans)))
	return b.String()
}
