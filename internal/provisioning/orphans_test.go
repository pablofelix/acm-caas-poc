package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/pablofelix/acm-caas-poc/internal/config"
)

func TestCaptureInfraIDWithInfraID(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")
	cd.Object["spec"] = map[string]interface{}{
		"clusterMetadata": map[string]interface{}{
			"infraID": "spoke1-abc12",
		},
		"platform": map[string]interface{}{
			"aws": map[string]interface{}{
				"region": "us-east-1",
			},
		},
	}

	c := fakeClient(cd)
	m := New(c, testConfig(), discardLogger)

	infraID, platform, region, err := m.CaptureInfraID(context.Background(), "spoke1")
	if err != nil {
		t.Fatalf("CaptureInfraID failed: %v", err)
	}
	if infraID != "spoke1-abc12" {
		t.Errorf("infraID = %s, want spoke1-abc12", infraID)
	}
	if platform != "aws" {
		t.Errorf("platform = %s, want aws", platform)
	}
	if region != "us-east-1" {
		t.Errorf("region = %s, want us-east-1", region)
	}
}

func TestCaptureInfraIDWithoutInfraID(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke2")
	cd.SetNamespace("spoke2")
	cd.Object["spec"] = map[string]interface{}{
		"platform": map[string]interface{}{
			"ibmcloud": map[string]interface{}{
				"region": "us-south",
			},
		},
	}

	c := fakeClient(cd)
	m := New(c, testConfig(), discardLogger)

	infraID, platform, region, err := m.CaptureInfraID(context.Background(), "spoke2")
	if err != nil {
		t.Fatalf("CaptureInfraID failed: %v", err)
	}
	if infraID != "spoke2" {
		t.Errorf("infraID = %s, want spoke2 (fallback to name)", infraID)
	}
	if platform != "ibmcloud" {
		t.Errorf("platform = %s, want ibmcloud", platform)
	}
	if region != "us-south" {
		t.Errorf("region = %s, want us-south", region)
	}
}

func TestCaptureInfraIDNotFound(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	_, _, _, err := m.CaptureInfraID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}

func TestFormatOrphanCheckResultClean(t *testing.T) {
	result := &OrphanCheckResult{
		InfraID:  "spoke1-abc12",
		Platform: "aws",
		Region:   "us-east-1",
		Clean:    true,
	}

	output := FormatOrphanCheckResult(result)
	if !strings.Contains(output, "spoke1-abc12") {
		t.Error("output should contain infraID")
	}
	if !strings.Contains(output, "Clean") {
		t.Error("output should say Clean")
	}
	if !strings.Contains(output, "No orphaned resources") {
		t.Error("output should say no orphans")
	}
}

func TestFormatOrphanCheckResultWithOrphans(t *testing.T) {
	result := &OrphanCheckResult{
		InfraID:  "spoke1-abc12",
		Platform: "aws",
		Region:   "us-east-1",
		Clean:    false,
		Orphans: []OrphanedResource{
			{Type: "ec2-instance", Name: "spoke1-master-0", ID: "i-12345", Status: "running"},
			{Type: "vpc", Name: "", ID: "vpc-67890"},
		},
	}

	output := FormatOrphanCheckResult(result)
	if !strings.Contains(output, "2 orphaned resources") {
		t.Error("output should mention 2 orphans")
	}
	if !strings.Contains(output, "ec2-instance") {
		t.Error("output should contain ec2-instance type")
	}
	if !strings.Contains(output, "i-12345") {
		t.Error("output should contain instance ID")
	}
	if !strings.Contains(output, "[running]") {
		t.Error("output should contain status in brackets")
	}
	if !strings.Contains(output, "manual cleanup") {
		t.Error("output should mention manual cleanup")
	}
}

func TestFormatOrphanCheckResultStatusEmpty(t *testing.T) {
	result := &OrphanCheckResult{
		InfraID:  "test-infra",
		Platform: "aws",
		Region:   "us-east-1",
		Clean:    false,
		Orphans: []OrphanedResource{
			{Type: "vpc", Name: "test-vpc", ID: "vpc-123", Status: ""},
		},
	}
	output := FormatOrphanCheckResult(result)
	if strings.Contains(output, "[]") {
		t.Error("output should not contain empty status brackets")
	}
}

func TestCheckOrphansIBMWithMockServers(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	defer iamServer.Close()

	vpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp interface{}
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			resp = map[string]interface{}{"instances": []map[string]interface{}{
				{"name": "myinfra-node-0", "id": "i-001", "status": "running"},
			}}
		default:
			resp = map[string]interface{}{}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer vpcServer.Close()

	c := fakeClient()
	cfg := testConfig()
	m := New(c, cfg, discardLogger)
	m.iamURL = iamServer.URL
	m.vpcURL = vpcServer.URL

	result, err := m.CheckOrphans(context.Background(), "myinfra", "ibmcloud", "us-south")
	if err != nil {
		t.Fatalf("CheckOrphans failed: %v", err)
	}
	if result.Clean {
		t.Error("expected non-clean result with orphans")
	}
	if len(result.Orphans) != 1 {
		t.Errorf("expected 1 orphan, got %d", len(result.Orphans))
	}
}

func TestCheckOrphansUnknownPlatform(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	_, err := m.CheckOrphans(context.Background(), "test-infra", "gcp", "us-central1")
	if err == nil {
		t.Fatal("expected error for unknown platform")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("expected unsupported platform error, got: %v", err)
	}
}

func TestQueryIBMCloudVPCWithOrphans(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(401)
			return
		}
		resp := map[string]interface{}{
			"instances": []map[string]interface{}{
				{"name": "test-infra-master-0", "id": "i-001", "status": "running"},
				{"name": "test-infra-worker-0", "id": "i-002", "status": "running"},
				{"name": "other-cluster-node", "id": "i-003", "status": "running"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "instance")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}

	if len(orphans) != 2 {
		t.Fatalf("expected 2 orphans, got %d", len(orphans))
	}
	if orphans[0].Name != "test-infra-master-0" {
		t.Errorf("orphan[0].Name = %s, want test-infra-master-0", orphans[0].Name)
	}
	if orphans[0].Type != "instance" {
		t.Errorf("orphan[0].Type = %s, want instance", orphans[0].Type)
	}
}

func TestQueryIBMCloudVPCNoOrphans(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"instances": []map[string]interface{}{
				{"name": "other-cluster-node", "id": "i-003", "status": "running"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "instance")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}

	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestQueryIBMCloudVPCServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	httpClient := server.Client()
	_, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "instance")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected HTTP 500 in error, got: %v", err)
	}
}

func TestQueryIBMCloudVPCForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer server.Close()

	httpClient := server.Client()
	_, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "instance")
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("expected HTTP 403 in error, got: %v", err)
	}
}

func TestQueryIBMCloudVPCInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	httpClient := server.Client()
	_, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "instance")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected invalid JSON in error, got: %v", err)
	}
}

func TestQueryIBMCloudVPCLoadBalancers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"load_balancers": []map[string]interface{}{
				{"name": "test-infra-kube-api", "id": "lb-001", "status": "active"},
				{"name": "unrelated-lb", "id": "lb-002", "status": "active"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "load-balancer")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan LB, got %d", len(orphans))
	}
	if orphans[0].Type != "load-balancer" {
		t.Errorf("type = %s, want load-balancer", orphans[0].Type)
	}
}

func TestQueryIBMCloudVPCSubnets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"subnets": []map[string]interface{}{
				{"name": "test-infra-subnet-1", "id": "sn-001", "status": "available"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "subnet")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan subnet, got %d", len(orphans))
	}
}

func TestQueryIBMCloudVPCVPCs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"vpcs": []map[string]interface{}{
				{"name": "test-infra-vpc", "id": "vpc-001", "status": "available"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "vpc")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan VPC, got %d", len(orphans))
	}
}

func TestQueryIBMCloudVPCFloatingIPs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"floating_ips": []map[string]interface{}{
				{"name": "test-infra-fip-1", "id": "fip-001", "status": "available"},
				{"name": "test-infra-fip-2", "id": "fip-002", "status": "available"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "floating-ip")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}

	if len(orphans) != 2 {
		t.Fatalf("expected 2 orphan floating IPs, got %d", len(orphans))
	}
}

func TestCheckIBMCloudOrphansWithMockServers(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	defer iamServer.Close()

	vpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp interface{}
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			resp = map[string]interface{}{"instances": []interface{}{}}
		case strings.Contains(r.URL.Path, "/load_balancers"):
			resp = map[string]interface{}{"load_balancers": []interface{}{}}
		case strings.Contains(r.URL.Path, "/subnets"):
			resp = map[string]interface{}{"subnets": []interface{}{}}
		case strings.Contains(r.URL.Path, "/vpcs"):
			resp = map[string]interface{}{"vpcs": []interface{}{}}
		case strings.Contains(r.URL.Path, "/floating_ips"):
			resp = map[string]interface{}{"floating_ips": []interface{}{}}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer vpcServer.Close()

	c := fakeClient()
	cfg := testConfig()
	m := New(c, cfg, discardLogger)
	m.iamURL = iamServer.URL
	m.vpcURL = vpcServer.URL

	orphans, err := m.checkIBMCloudOrphans("test-infra", "us-south")
	if err != nil {
		t.Fatalf("checkIBMCloudOrphans failed: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestCheckIBMCloudOrphansIAMFailure(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer iamServer.Close()

	c := fakeClient()
	cfg := testConfig()
	m := New(c, cfg, discardLogger)
	m.iamURL = iamServer.URL

	_, err := m.checkIBMCloudOrphans("test-infra", "us-south")
	if err == nil {
		t.Fatal("expected error for IAM failure")
	}
	if !strings.Contains(err.Error(), "no IBM Cloud credentials") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckIBMCloudOrphansNoCredentials(t *testing.T) {
	c := fakeClient()
	cfg := testConfig()
	cfg.IBMCloudAPIKey = ""
	m := New(c, cfg, discardLogger)

	_, err := m.checkIBMCloudOrphans("test-infra", "us-south")
	if err == nil {
		t.Fatal("expected error with no credentials")
	}
	if !strings.Contains(err.Error(), "no IBM Cloud credentials") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckOrphansAWSExecFailure(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.awsExec = func(args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exec: aws not found")
	}

	_, err := m.CheckOrphans(context.Background(), "test-infra", "aws", "us-east-1")
	if err == nil {
		t.Fatal("expected error when AWS CLI fails")
	}
	if !strings.Contains(err.Error(), "aws") {
		t.Errorf("expected aws-related error, got: %v", err)
	}
}

func TestCheckOrphansAWSClean(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.awsExec = func(args ...string) ([]byte, error) {
		if args[0] == "elbv2" {
			return []byte(`{"LoadBalancers":[]}`), nil
		}
		return []byte(`[]`), nil
	}

	result, err := m.CheckOrphans(context.Background(), "test-infra", "aws", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Clean {
		t.Error("expected clean result for empty AWS responses")
	}
}

func TestCheckOrphansAWSWithOrphans(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.awsExec = func(args ...string) ([]byte, error) {
		if args[0] == "ec2" && args[1] == "describe-instances" {
			return []byte(`[["i-1234","running","test-node"]]`), nil
		}
		if args[0] == "ec2" && args[1] == "describe-vpcs" {
			return []byte(`[["vpc-5678","available"]]`), nil
		}
		if args[0] == "elbv2" {
			return []byte(`{"LoadBalancers":[{"LoadBalancerArn":"arn:aws:test","LoadBalancerName":"test-infra-lb"}]}`), nil
		}
		return []byte(`[]`), nil
	}

	result, err := m.CheckOrphans(context.Background(), "test-infra", "aws", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Clean {
		t.Error("expected orphans")
	}
	if len(result.Orphans) != 3 {
		t.Errorf("expected 3 orphans (instance + vpc + lb), got %d", len(result.Orphans))
	}
}

func TestCheckOrphansAWSInvalidJSON(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.awsExec = func(args ...string) ([]byte, error) {
		return []byte(`not-json`), nil
	}

	_, err := m.CheckOrphans(context.Background(), "test-infra", "aws", "us-east-1")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected invalid JSON error, got: %v", err)
	}
}

func TestCheckOrphansAWSTerminatedFiltered(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.awsExec = func(args ...string) ([]byte, error) {
		if args[0] == "ec2" && args[1] == "describe-instances" {
			return []byte(`[["i-1234","terminated","old-node"]]`), nil
		}
		if args[0] == "elbv2" {
			return []byte(`{"LoadBalancers":[]}`), nil
		}
		return []byte(`[]`), nil
	}

	result, err := m.CheckOrphans(context.Background(), "test-infra", "aws", "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Clean {
		t.Error("terminated instances should be filtered out")
	}
}

func TestDestroyWithOrphanCheckClusterNotFound(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	_, err := m.DestroyWithOrphanCheck(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster")
	}
}

func TestCheckOrphansIBMPlatformNoCredentials(t *testing.T) {
	c := fakeClient()
	cfg := config.Config{}
	m := New(c, cfg, discardLogger)

	_, err := m.CheckOrphans(context.Background(), "test-infra", "ibmcloud", "us-south")
	if err == nil {
		t.Fatal("expected error for IBM orphan check without credentials")
	}
}

func TestDestroyWithOrphanCheckHappyPath(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	defer iamServer.Close()

	vpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{}
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			resp["instances"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/load_balancers"):
			resp["load_balancers"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/subnets"):
			resp["subnets"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/vpcs"):
			resp["vpcs"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/floating_ips"):
			resp["floating_ips"] = []interface{}{}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer vpcServer.Close()

	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")
	cd.Object["spec"] = map[string]interface{}{
		"clusterMetadata": map[string]interface{}{
			"infraID": "spoke1-xyz99",
		},
		"platform": map[string]interface{}{
			"ibmcloud": map[string]interface{}{
				"region": "us-south",
			},
		},
	}

	c := fakeClient(cd)
	cfg := testConfig()
	m := New(c, cfg, discardLogger)
	m.iamURL = iamServer.URL
	m.vpcURL = vpcServer.URL

	result, err := m.DestroyWithOrphanCheck(context.Background(), "spoke1")
	if err != nil {
		t.Fatalf("DestroyWithOrphanCheck failed: %v", err)
	}
	if result.InfraID != "spoke1-xyz99" {
		t.Errorf("infraID = %s, want spoke1-xyz99", result.InfraID)
	}
	if result.Platform != "ibmcloud" {
		t.Errorf("platform = %s, want ibmcloud", result.Platform)
	}
	if !result.Clean {
		t.Error("expected clean result with no orphans")
	}
}

func TestDestroyWithOrphanCheckContextCancelled(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	defer iamServer.Close()

	vpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{}
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			resp["instances"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/load_balancers"):
			resp["load_balancers"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/subnets"):
			resp["subnets"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/vpcs"):
			resp["vpcs"] = []interface{}{}
		case strings.Contains(r.URL.Path, "/floating_ips"):
			resp["floating_ips"] = []interface{}{}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer vpcServer.Close()

	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("stuck")
	cd.SetNamespace("stuck")
	cd.Object["spec"] = map[string]interface{}{
		"platform": map[string]interface{}{
			"ibmcloud": map[string]interface{}{"region": "us-south"},
		},
	}

	c := fakeClient(cd)
	cfg := testConfig()
	m := New(c, cfg, discardLogger)
	m.iamURL = iamServer.URL
	m.vpcURL = vpcServer.URL

	result, err := m.DestroyWithOrphanCheck(context.Background(), "stuck")
	if err != nil {
		t.Fatalf("DestroyWithOrphanCheck failed: %v", err)
	}
	if result.InfraID != "stuck" {
		t.Errorf("infraID = %s, want stuck (fallback)", result.InfraID)
	}
}

func TestCaptureInfraIDNoPlatform(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("minimal")
	cd.SetNamespace("minimal")
	cd.Object["spec"] = map[string]interface{}{}

	c := fakeClient(cd)
	m := New(c, testConfig(), discardLogger)

	infraID, platform, region, err := m.CaptureInfraID(context.Background(), "minimal")
	if err != nil {
		t.Fatalf("CaptureInfraID failed: %v", err)
	}
	if infraID != "minimal" {
		t.Errorf("infraID = %s, want minimal (fallback)", infraID)
	}
	if platform != "" {
		t.Errorf("platform = %s, want empty", platform)
	}
	if region != "" {
		t.Errorf("region = %s, want empty", region)
	}
}

func TestQueryIBMCloudOrphansWithHTTPTest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(401)
			return
		}
		var resp interface{}
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			resp = map[string]interface{}{
				"instances": []map[string]interface{}{
					{"name": "myinfra-master-0", "id": "i-001", "status": "running"},
					{"name": "other-node", "id": "i-002", "status": "running"},
				},
			}
		case strings.Contains(r.URL.Path, "/load_balancers"):
			resp = map[string]interface{}{
				"load_balancers": []map[string]interface{}{
					{"name": "myinfra-kube-api", "id": "lb-001", "status": "active"},
				},
			}
		case strings.Contains(r.URL.Path, "/subnets"):
			resp = map[string]interface{}{
				"subnets": []map[string]interface{}{},
			}
		case strings.Contains(r.URL.Path, "/vpcs"):
			resp = map[string]interface{}{
				"vpcs": []map[string]interface{}{
					{"name": "myinfra-vpc", "id": "vpc-001", "status": "available"},
				},
			}
		case strings.Contains(r.URL.Path, "/floating_ips"):
			resp = map[string]interface{}{
				"floating_ips": []map[string]interface{}{},
			}
		default:
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	orphans, err := queryIBMCloudOrphans(server.Client(), "test-token", server.URL+"/v1", "myinfra")
	if err != nil {
		t.Fatalf("queryIBMCloudOrphans failed: %v", err)
	}
	if len(orphans) != 3 {
		t.Errorf("expected 3 orphans, got %d: %+v", len(orphans), orphans)
	}

	types := map[string]bool{}
	for _, o := range orphans {
		types[o.Type] = true
	}
	if !types["instance"] {
		t.Error("expected instance orphan")
	}
	if !types["load-balancer"] {
		t.Error("expected load-balancer orphan")
	}
	if !types["vpc"] {
		t.Error("expected vpc orphan")
	}
}

func TestQueryIBMCloudOrphansClean(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{}
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			resp["instances"] = []map[string]interface{}{}
		case strings.Contains(r.URL.Path, "/load_balancers"):
			resp["load_balancers"] = []map[string]interface{}{}
		case strings.Contains(r.URL.Path, "/subnets"):
			resp["subnets"] = []map[string]interface{}{}
		case strings.Contains(r.URL.Path, "/vpcs"):
			resp["vpcs"] = []map[string]interface{}{}
		case strings.Contains(r.URL.Path, "/floating_ips"):
			resp["floating_ips"] = []map[string]interface{}{}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	orphans, err := queryIBMCloudOrphans(server.Client(), "test-token", server.URL+"/v1", "myinfra")
	if err != nil {
		t.Fatalf("queryIBMCloudOrphans failed: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestQueryIBMCloudOrphansPartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/instances"):
			json.NewEncoder(w).Encode(map[string]interface{}{"instances": []interface{}{}})
		case strings.Contains(r.URL.Path, "/load_balancers"):
			w.WriteHeader(500)
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	defer server.Close()

	_, err := queryIBMCloudOrphans(server.Client(), "test-token", server.URL+"/v1", "myinfra")
	if err == nil {
		t.Fatal("expected error when one query fails")
	}
	if !strings.Contains(err.Error(), "load-balancer") {
		t.Errorf("expected load-balancer in error, got: %v", err)
	}
}

func TestQueryIBMCloudVPCEmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"instances": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	httpClient := server.Client()
	orphans, err := queryIBMCloudVPC(httpClient, "test-token", server.URL, "test-infra", "instance")
	if err != nil {
		t.Fatalf("queryIBMCloudVPC failed: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans for empty list, got %d", len(orphans))
	}
}
