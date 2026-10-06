package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type PreflightResult struct {
	Check   string `json:"check"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

func (m *Manager) Preflight(ctx context.Context, opts ClusterOpts) ([]PreflightResult, error) {
	m.logger.Info("provisioning.Preflight", "platform", opts.Platform)
	m.applyDefaults(&opts)

	var results []PreflightResult

	results = append(results, m.checkCredentials(opts)...)
	results = append(results, m.checkClusterImageSet(ctx, opts)...)
	results = append(results, m.checkPullSecret(opts)...)
	results = append(results, m.checkNameConflict(ctx, opts)...)

	switch opts.Platform {
	case "aws":
		results = append(results, m.checkAWSPreflight(ctx, opts)...)
	case "ibmcloud":
		results = append(results, m.checkIBMCloudPreflight(ctx, opts)...)
	}

	return results, nil
}

func (m *Manager) checkCredentials(opts ClusterOpts) []PreflightResult {
	switch opts.Platform {
	case "aws":
		if opts.AWSAccessKeyID == "" || opts.AWSSecretAccessKey == "" {
			return []PreflightResult{{
				Check:  "aws-credentials",
				Status: "FAIL",
				Detail: "AWS credentials missing — set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY or place in ~/.aws/credentials",
			}}
		}
		return []PreflightResult{{Check: "aws-credentials", Status: "PASS"}}
	case "ibmcloud":
		if opts.IBMCloudAPIKey == "" {
			return []PreflightResult{{
				Check:  "ibmcloud-credentials",
				Status: "FAIL",
				Detail: "IBM Cloud API key missing — set IBMCLOUD_API_KEY or use ACM credential store",
			}}
		}
		return []PreflightResult{{Check: "ibmcloud-credentials", Status: "PASS"}}
	}
	return nil
}

func (m *Manager) checkClusterImageSet(ctx context.Context, opts ClusterOpts) []PreflightResult {
	if opts.ImageSet == "" {
		return []PreflightResult{{
			Check:  "cluster-image-set",
			Status: "FAIL",
			Detail: "No ClusterImageSet specified — set ACM_CLUSTER_IMAGE_SET",
		}}
	}
	_, err := m.client.Get(ctx, client.GVRClusterImageSet, "", opts.ImageSet)
	if err != nil {
		return []PreflightResult{{
			Check:  "cluster-image-set",
			Status: "FAIL",
			Detail: fmt.Sprintf("ClusterImageSet %q not found on hub", opts.ImageSet),
		}}
	}
	return []PreflightResult{{
		Check:  "cluster-image-set",
		Status: "PASS",
		Detail: opts.ImageSet,
	}}
}

func (m *Manager) checkPullSecret(opts ClusterOpts) []PreflightResult {
	if opts.PullSecret == "" {
		return []PreflightResult{{
			Check:  "pull-secret",
			Status: "FAIL",
			Detail: "Pull secret is empty",
		}}
	}
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(opts.PullSecret), &js); err != nil {
		return []PreflightResult{{
			Check:  "pull-secret",
			Status: "FAIL",
			Detail: "Pull secret is not valid JSON",
		}}
	}
	return []PreflightResult{{Check: "pull-secret", Status: "PASS"}}
}

func (m *Manager) checkNameConflict(ctx context.Context, opts ClusterOpts) []PreflightResult {
	_, err := m.client.Get(ctx, client.GVRClusterDeployment, opts.Name, opts.Name)
	if err == nil {
		return []PreflightResult{{
			Check:  "name-conflict",
			Status: "FAIL",
			Detail: fmt.Sprintf("ClusterDeployment %q already exists", opts.Name),
		}}
	}
	return []PreflightResult{{Check: "name-conflict", Status: "PASS"}}
}

func (m *Manager) checkAWSPreflight(ctx context.Context, opts ClusterOpts) []PreflightResult {
	var results []PreflightResult

	// Validate credentials with STS
	stsResult := m.checkAWSSTS(opts)
	results = append(results, stsResult)

	// Check vCPU quota via Service Quotas API
	if stsResult.Status == "PASS" {
		results = append(results, m.checkAWSvCPUQuota(opts)...)
	}

	return results
}

func (m *Manager) checkAWSSTS(opts ClusterOpts) PreflightResult {
	cmd := exec.Command("aws", "sts", "get-caller-identity",
		"--region", opts.Region,
		"--output", "json")
	cmd.Env = append(cmd.Environ(),
		"AWS_ACCESS_KEY_ID="+opts.AWSAccessKeyID,
		"AWS_SECRET_ACCESS_KEY="+opts.AWSSecretAccessKey,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return PreflightResult{
			Check:  "aws-auth",
			Status: "FAIL",
			Detail: fmt.Sprintf("STS GetCallerIdentity failed: %s", strings.TrimSpace(string(out))),
		}
	}

	var identity map[string]interface{}
	if json.Unmarshal(out, &identity) == nil {
		if arn, ok := identity["Arn"].(string); ok {
			return PreflightResult{
				Check:  "aws-auth",
				Status: "PASS",
				Detail: arn,
			}
		}
	}
	return PreflightResult{Check: "aws-auth", Status: "PASS"}
}

func (m *Manager) checkAWSvCPUQuota(opts ClusterOpts) []PreflightResult {
	// Standard On-Demand vCPU quota (L-1216C47A)
	cmd := exec.Command("aws", "service-quotas", "get-service-quota",
		"--service-code", "ec2",
		"--quota-code", "L-1216C47A",
		"--region", opts.Region,
		"--output", "json")
	cmd.Env = append(cmd.Environ(),
		"AWS_ACCESS_KEY_ID="+opts.AWSAccessKeyID,
		"AWS_SECRET_ACCESS_KEY="+opts.AWSSecretAccessKey,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return []PreflightResult{{
			Check:  "aws-vcpu-quota",
			Status: "WARN",
			Detail: "Could not check vCPU quota — verify manually",
		}}
	}

	var resp struct {
		Quota struct {
			Value float64 `json:"Value"`
		} `json:"Quota"`
	}
	if json.Unmarshal(out, &resp) != nil {
		return []PreflightResult{{
			Check:  "aws-vcpu-quota",
			Status: "WARN",
			Detail: "Could not parse quota response",
		}}
	}

	// OCP needs ~28 vCPUs minimum (3 masters x 4 + 2 workers x 2 + overhead)
	needed := int64(28)
	if resp.Quota.Value < float64(needed) {
		return []PreflightResult{{
			Check:  "aws-vcpu-quota",
			Status: "FAIL",
			Detail: fmt.Sprintf("On-Demand vCPU quota is %.0f, need at least %d for OCP cluster", resp.Quota.Value, needed),
		}}
	}

	return []PreflightResult{{
		Check:  "aws-vcpu-quota",
		Status: "PASS",
		Detail: fmt.Sprintf("%.0f vCPUs available (need ~%d)", resp.Quota.Value, needed),
	}}
}

func (m *Manager) checkIBMCloudPreflight(ctx context.Context, opts ClusterOpts) []PreflightResult {
	var results []PreflightResult

	// Validate API key with IAM token exchange
	iamResult, iamToken := m.checkIBMCloudIAM(opts)
	results = append(results, iamResult)

	if iamResult.Status == "PASS" && iamToken != "" {
		results = append(results, m.checkIBMCloudVCPUQuota(opts, iamToken)...)
	}

	return results
}

func (m *Manager) ibmIAMURL() string {
	if m.iamURL != "" {
		return m.iamURL
	}
	return "https://iam.cloud.ibm.com"
}

func (m *Manager) ibmVPCURL(region string) string {
	if m.vpcURL != "" {
		return m.vpcURL
	}
	return fmt.Sprintf("https://%s.iaas.cloud.ibm.com", region)
}

func (m *Manager) checkIBMCloudIAM(opts ClusterOpts) (PreflightResult, string) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	body := strings.NewReader("grant_type=urn:ibm:params:oauth:grant-type:apikey&apikey=" + opts.IBMCloudAPIKey)
	req, _ := http.NewRequest("POST", m.ibmIAMURL()+"/identity/token", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return PreflightResult{
			Check:  "ibmcloud-auth",
			Status: "FAIL",
			Detail: fmt.Sprintf("IAM token exchange failed: %v", err),
		}, ""
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return PreflightResult{
			Check:  "ibmcloud-auth",
			Status: "FAIL",
			Detail: fmt.Sprintf("IAM returned %d", resp.StatusCode),
		}, ""
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(respBody, &token) != nil || token.AccessToken == "" {
		return PreflightResult{
			Check:  "ibmcloud-auth",
			Status: "FAIL",
			Detail: "IAM response did not contain access_token",
		}, ""
	}

	return PreflightResult{
		Check:  "ibmcloud-auth",
		Status: "PASS",
		Detail: "authenticated",
	}, token.AccessToken
}

func (m *Manager) checkIBMCloudVCPUQuota(opts ClusterOpts, iamToken string) []PreflightResult {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/v1/instances?version=2024-06-04&generation=2&limit=100", m.ibmVPCURL(opts.Region))
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+iamToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return []PreflightResult{{
			Check:  "ibmcloud-vcpu-quota",
			Status: "WARN",
			Detail: fmt.Sprintf("Could not query VPC instances: %v", err),
		}}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []PreflightResult{{
			Check:  "ibmcloud-vcpu-quota",
			Status: "WARN",
			Detail: fmt.Sprintf("VPC API returned %d", resp.StatusCode),
		}}
	}

	respBody, _ := io.ReadAll(resp.Body)
	var instanceList struct {
		Instances []struct {
			VCPU struct {
				Count int `json:"count"`
			} `json:"vcpu"`
			Status string `json:"status"`
		} `json:"instances"`
		TotalCount int `json:"total_count"`
	}
	if json.Unmarshal(respBody, &instanceList) != nil {
		return []PreflightResult{{
			Check:  "ibmcloud-vcpu-quota",
			Status: "WARN",
			Detail: "Could not parse VPC instances response",
		}}
	}

	usedVCPUs := 0
	for _, inst := range instanceList.Instances {
		if inst.Status == "running" || inst.Status == "starting" || inst.Status == "stopping" {
			usedVCPUs += inst.VCPU.Count
		}
	}

	// Default IBM Cloud quota is 200 vCPUs per region for most accounts
	// The QE account has 1800 — we report usage and let the user judge
	return []PreflightResult{{
		Check:  "ibmcloud-vcpu-usage",
		Status: "PASS",
		Detail: fmt.Sprintf("%d vCPUs in use across %d instances (quota check: verify %d + ~28 needed < account limit)",
			usedVCPUs, instanceList.TotalCount, usedVCPUs),
	}}
}

func FormatPreflightResults(results []PreflightResult) string {
	var b strings.Builder
	b.WriteString("\n  ┌─ Preflight checks\n")

	passed, failed, warned := 0, 0, 0
	for _, r := range results {
		icon := "✓"
		switch r.Status {
		case "FAIL":
			icon = "✗"
			failed++
		case "WARN":
			icon = "!"
			warned++
		default:
			passed++
		}
		b.WriteString(fmt.Sprintf("  │  %s %-25s %s\n", icon, r.Check, r.Detail))
	}

	summary := fmt.Sprintf("  └─ %d passed, %d failed, %d warnings\n",
		passed, failed, warned)
	b.WriteString(summary)

	return b.String()
}

func PreflightPassed(results []PreflightResult) bool {
	for _, r := range results {
		if r.Status == "FAIL" {
			return false
		}
	}
	return true
}

// estimateVCPUs calculates approximate vCPUs needed for a cluster
func estimateVCPUs(opts ClusterOpts) int64 {
	masterVCPUs := instanceTypeVCPUs(opts.MasterType)
	workerVCPUs := instanceTypeVCPUs(opts.WorkerType)
	masters := opts.MasterReplicas
	if masters == 0 {
		masters = 3
	}
	workers := opts.WorkerReplicas
	if workers == 0 {
		workers = 2
	}
	// +1 bootstrap node (same size as master, destroyed after install)
	return (masters+1)*masterVCPUs + workers*workerVCPUs
}

func instanceTypeVCPUs(instanceType string) int64 {
	// AWS instance types
	awsVCPUs := map[string]int64{
		"m5.large":    2,
		"m5.xlarge":   4,
		"m5.2xlarge":  8,
		"m5.4xlarge":  16,
		"m6i.large":   2,
		"m6i.xlarge":  4,
		"m6i.2xlarge": 8,
	}
	if v, ok := awsVCPUs[instanceType]; ok {
		return v
	}
	// IBM Cloud profiles: bx2-NxM where N is vCPUs
	if strings.HasPrefix(instanceType, "bx2-") || strings.HasPrefix(instanceType, "cx2-") {
		parts := strings.Split(instanceType, "-")
		if len(parts) >= 2 {
			dims := strings.Split(parts[1], "x")
			if len(dims) >= 1 {
				if v, err := strconv.ParseInt(dims[0], 10, 64); err == nil {
					return v
				}
			}
		}
	}
	return 4 // safe default
}
