package provisioning

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckCredentialsAWSMissing(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkCredentials(ClusterOpts{Platform: "aws"})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for missing AWS creds, got %+v", results)
	}
	if results[0].Check != "aws-credentials" {
		t.Errorf("check = %s, want aws-credentials", results[0].Check)
	}
}

func TestCheckCredentialsAWSPresent(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkCredentials(ClusterOpts{
		Platform:           "aws",
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
	})
	if len(results) != 1 || results[0].Status != "PASS" {
		t.Fatalf("expected PASS for present AWS creds, got %+v", results)
	}
}

func TestCheckCredentialsIBMMissing(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkCredentials(ClusterOpts{Platform: "ibmcloud"})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for missing IBM creds, got %+v", results)
	}
	if results[0].Check != "ibmcloud-credentials" {
		t.Errorf("check = %s, want ibmcloud-credentials", results[0].Check)
	}
}

func TestCheckCredentialsIBMPresent(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkCredentials(ClusterOpts{
		Platform:       "ibmcloud",
		IBMCloudAPIKey: "test-api-key",
	})
	if len(results) != 1 || results[0].Status != "PASS" {
		t.Fatalf("expected PASS for present IBM creds, got %+v", results)
	}
}

func TestCheckCredentialsUnknownPlatform(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkCredentials(ClusterOpts{Platform: "gcp"})
	if results != nil {
		t.Fatalf("expected nil for unknown platform, got %+v", results)
	}
}

func TestCheckPullSecretEmpty(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkPullSecret(ClusterOpts{PullSecret: ""})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for empty pull secret, got %+v", results)
	}
	if !strings.Contains(results[0].Detail, "empty") {
		t.Errorf("detail should mention empty, got %s", results[0].Detail)
	}
}

func TestCheckPullSecretInvalidJSON(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkPullSecret(ClusterOpts{PullSecret: "not-json"})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for invalid JSON, got %+v", results)
	}
	if !strings.Contains(results[0].Detail, "not valid JSON") {
		t.Errorf("detail should mention invalid JSON, got %s", results[0].Detail)
	}
}

func TestCheckPullSecretValid(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkPullSecret(ClusterOpts{PullSecret: `{"auths":{}}`})
	if len(results) != 1 || results[0].Status != "PASS" {
		t.Fatalf("expected PASS for valid pull secret, got %+v", results)
	}
}

func TestCheckClusterImageSetNotFound(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkClusterImageSet(context.Background(), ClusterOpts{ImageSet: "nonexistent"})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for missing image set, got %+v", results)
	}
}

func TestCheckClusterImageSetEmpty(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkClusterImageSet(context.Background(), ClusterOpts{ImageSet: ""})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for empty image set, got %+v", results)
	}
}

func TestCheckClusterImageSetFound(t *testing.T) {
	imgset := &unstructured.Unstructured{}
	imgset.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	imgset.SetName("img4.20.0-multi-appsub")

	c := fakeClient(imgset)
	m := New(c, testConfig(), discardLogger)

	results := m.checkClusterImageSet(context.Background(), ClusterOpts{ImageSet: "img4.20.0-multi-appsub"})
	if len(results) != 1 || results[0].Status != "PASS" {
		t.Fatalf("expected PASS for existing image set, got %+v", results)
	}
	if results[0].Detail != "img4.20.0-multi-appsub" {
		t.Errorf("detail = %s, want img4.20.0-multi-appsub", results[0].Detail)
	}
}

func TestCheckNameConflictExists(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("spoke1")
	cd.SetNamespace("spoke1")

	c := fakeClient(cd)
	m := New(c, testConfig(), discardLogger)

	results := m.checkNameConflict(context.Background(), ClusterOpts{Name: "spoke1"})
	if len(results) != 1 || results[0].Status != "FAIL" {
		t.Fatalf("expected FAIL for existing cluster, got %+v", results)
	}
	if !strings.Contains(results[0].Detail, "already exists") {
		t.Errorf("detail should mention already exists, got %s", results[0].Detail)
	}
}

func TestCheckNameConflictNotExists(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkNameConflict(context.Background(), ClusterOpts{Name: "new-cluster"})
	if len(results) != 1 || results[0].Status != "PASS" {
		t.Fatalf("expected PASS for non-existing cluster, got %+v", results)
	}
}

func TestFormatPreflightResults(t *testing.T) {
	results := []PreflightResult{
		{Check: "credentials", Status: "PASS", Detail: ""},
		{Check: "image-set", Status: "FAIL", Detail: "not found"},
		{Check: "quota", Status: "WARN", Detail: "could not check"},
	}

	output := FormatPreflightResults(results)
	if !strings.Contains(output, "Preflight checks") {
		t.Error("output should contain header")
	}
	if !strings.Contains(output, "1 passed, 1 failed, 1 warnings") {
		t.Errorf("unexpected summary in output: %s", output)
	}
	if !strings.Contains(output, "✓") {
		t.Error("output should contain checkmark for PASS")
	}
	if !strings.Contains(output, "✗") {
		t.Error("output should contain cross for FAIL")
	}
	if !strings.Contains(output, "!") {
		t.Error("output should contain exclamation for WARN")
	}
}

func TestPreflightPassedAllPass(t *testing.T) {
	results := []PreflightResult{
		{Check: "a", Status: "PASS"},
		{Check: "b", Status: "PASS"},
		{Check: "c", Status: "WARN"},
	}
	if !PreflightPassed(results) {
		t.Error("expected true when no FAIL results")
	}
}

func TestPreflightPassedOneFail(t *testing.T) {
	results := []PreflightResult{
		{Check: "a", Status: "PASS"},
		{Check: "b", Status: "FAIL"},
	}
	if PreflightPassed(results) {
		t.Error("expected false when FAIL result present")
	}
}

func TestPreflightPassedEmpty(t *testing.T) {
	if !PreflightPassed(nil) {
		t.Error("expected true for empty results")
	}
}

func TestInstanceTypeVCPUsAWS(t *testing.T) {
	tests := []struct {
		instanceType string
		want         int64
	}{
		{"m5.large", 2},
		{"m5.xlarge", 4},
		{"m5.2xlarge", 8},
		{"m5.4xlarge", 16},
		{"m6i.large", 2},
		{"m6i.xlarge", 4},
		{"m6i.2xlarge", 8},
	}
	for _, tt := range tests {
		got := instanceTypeVCPUs(tt.instanceType)
		if got != tt.want {
			t.Errorf("instanceTypeVCPUs(%s) = %d, want %d", tt.instanceType, got, tt.want)
		}
	}
}

func TestInstanceTypeVCPUsIBM(t *testing.T) {
	tests := []struct {
		instanceType string
		want         int64
	}{
		{"bx2-4x16", 4},
		{"bx2-8x32", 8},
		{"cx2-16x32", 16},
		{"bx2-2x8", 2},
	}
	for _, tt := range tests {
		got := instanceTypeVCPUs(tt.instanceType)
		if got != tt.want {
			t.Errorf("instanceTypeVCPUs(%s) = %d, want %d", tt.instanceType, got, tt.want)
		}
	}
}

func TestInstanceTypeVCPUsUnknown(t *testing.T) {
	got := instanceTypeVCPUs("unknown-type")
	if got != 4 {
		t.Errorf("instanceTypeVCPUs(unknown-type) = %d, want 4 (default)", got)
	}
}

func TestEstimateVCPUsDefaults(t *testing.T) {
	opts := ClusterOpts{
		MasterType: "m5.xlarge",
		WorkerType: "m5.large",
	}
	got := estimateVCPUs(opts)
	// 0 replicas → defaults: 3 masters + 1 bootstrap = 4 x 4 vCPUs = 16, 2 workers x 2 vCPUs = 4 → total 20
	if got != 20 {
		t.Errorf("estimateVCPUs = %d, want 20", got)
	}
}

func TestEstimateVCPUsCustomReplicas(t *testing.T) {
	opts := ClusterOpts{
		MasterType:     "m5.xlarge",
		WorkerType:     "m5.xlarge",
		MasterReplicas: 3,
		WorkerReplicas: 3,
	}
	got := estimateVCPUs(opts)
	// (3+1) masters * 4 + 3 workers * 4 = 16 + 12 = 28
	if got != 28 {
		t.Errorf("estimateVCPUs = %d, want 28", got)
	}
}

func TestPreflightFullFlowWithFakeClient(t *testing.T) {
	imgset := &unstructured.Unstructured{}
	imgset.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	imgset.SetName("img4.20.0-multi-appsub")

	c := fakeClient(imgset)
	cfg := testConfig()
	cfg.Platform = "aws"
	m := New(c, cfg, discardLogger)

	opts := ClusterOpts{
		Name:               "test-cluster",
		Platform:           "aws",
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
		PullSecret:         `{"auths":{}}`,
		ImageSet:           "img4.20.0-multi-appsub",
		Region:             "us-east-1",
	}

	results, err := m.Preflight(context.Background(), opts)
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}

	// Should have: aws-credentials PASS, cluster-image-set PASS, pull-secret PASS, name-conflict PASS
	// Plus AWS-specific checks (which will fail because no real AWS CLI — that's expected)
	passCount := 0
	for _, r := range results {
		if r.Status == "PASS" {
			passCount++
		}
	}
	if passCount < 4 {
		t.Errorf("expected at least 4 PASS results (k8s checks), got %d passes out of %d total", passCount, len(results))
	}
}

func TestPreflightIBMCloudWithFakeClient(t *testing.T) {
	imgset := &unstructured.Unstructured{}
	imgset.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	imgset.SetName("img4.20.0-multi-appsub")

	c := fakeClient(imgset)
	m := New(c, testConfig(), discardLogger)

	opts := ClusterOpts{
		Name:           "test-ibm",
		Platform:       "ibmcloud",
		IBMCloudAPIKey: "test-api-key",
		PullSecret:     `{"auths":{}}`,
		ImageSet:       "img4.20.0-multi-appsub",
		Region:         "us-south",
	}

	results, err := m.Preflight(context.Background(), opts)
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}

	// k8s checks should pass; IBM cloud checks will fail (no real IAM)
	foundCredPass := false
	for _, r := range results {
		if r.Check == "ibmcloud-credentials" && r.Status == "PASS" {
			foundCredPass = true
		}
	}
	if !foundCredPass {
		t.Error("expected ibmcloud-credentials PASS")
	}
}

func TestPreflightNameConflictDetected(t *testing.T) {
	cd := &unstructured.Unstructured{}
	cd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	cd.SetName("existing")
	cd.SetNamespace("existing")

	imgset := &unstructured.Unstructured{}
	imgset.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	imgset.SetName("img4.20.0-multi-appsub")

	c := fakeClient(cd, imgset)
	m := New(c, testConfig(), discardLogger)

	opts := ClusterOpts{
		Name:           "existing",
		Platform:       "ibmcloud",
		IBMCloudAPIKey: "test-api-key",
		PullSecret:     `{"auths":{}}`,
		ImageSet:       "img4.20.0-multi-appsub",
	}

	results, err := m.Preflight(context.Background(), opts)
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}

	if PreflightPassed(results) {
		t.Error("preflight should fail when cluster already exists")
	}

	foundConflict := false
	for _, r := range results {
		if r.Check == "name-conflict" && r.Status == "FAIL" {
			foundConflict = true
		}
	}
	if !foundConflict {
		t.Error("expected name-conflict FAIL")
	}
}

func TestCheckAWSPreflightNoAwsCLI(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	opts := ClusterOpts{
		Platform:           "aws",
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
		Region:             "us-east-1",
	}

	results := m.checkAWSPreflight(context.Background(), opts)
	// Without real AWS CLI, STS check should fail
	if len(results) == 0 {
		t.Fatal("expected at least one result from AWS preflight")
	}
	if results[0].Check != "aws-auth" {
		t.Errorf("first check = %s, want aws-auth", results[0].Check)
	}
}

func TestCheckIBMCloudPreflightNoRealAPI(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	opts := ClusterOpts{
		Platform:       "ibmcloud",
		IBMCloudAPIKey: "test-api-key",
		Region:         "us-south",
	}

	results := m.checkIBMCloudPreflight(context.Background(), opts)
	if len(results) == 0 {
		t.Fatal("expected at least one result from IBM preflight")
	}
	if results[0].Check != "ibmcloud-auth" {
		t.Errorf("first check = %s, want ibmcloud-auth", results[0].Check)
	}
}

func TestCheckAWSSTSNoRealAWS(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	result := m.checkAWSSTS(ClusterOpts{
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
		Region:             "us-east-1",
	})
	// Will FAIL or PASS depending on whether aws CLI is available — just check it returns a result
	if result.Check != "aws-auth" {
		t.Errorf("check = %s, want aws-auth", result.Check)
	}
}

func TestCheckAWSvCPUQuotaNoRealAWS(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)

	results := m.checkAWSvCPUQuota(ClusterOpts{
		AWSAccessKeyID:     "test-access-key-id",
		AWSSecretAccessKey: "test-secret-access-key",
		Region:             "us-east-1",
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Check != "aws-vcpu-quota" {
		t.Errorf("check = %s, want aws-vcpu-quota", results[0].Check)
	}
}

func TestPreflightUnknownPlatformSkipsCloudChecks(t *testing.T) {
	imgset := &unstructured.Unstructured{}
	imgset.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	imgset.SetName("img4.20.0-multi-appsub")

	c := fakeClient(imgset)
	cfg := testConfig()
	cfg.Platform = "gcp"
	m := New(c, cfg, discardLogger)

	opts := ClusterOpts{
		Name:       "gcp-cluster",
		Platform:   "gcp",
		PullSecret: `{"auths":{}}`,
		ImageSet:   "img4.20.0-multi-appsub",
	}

	results, err := m.Preflight(context.Background(), opts)
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}

	// Should only have k8s checks (no cloud-specific checks for gcp)
	for _, r := range results {
		if strings.HasPrefix(r.Check, "aws-") || strings.HasPrefix(r.Check, "ibmcloud-") {
			t.Errorf("unexpected cloud check for gcp platform: %s", r.Check)
		}
	}
}

func TestCheckIBMCloudIAMSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity/token" {
			json.NewEncoder(w).Encode(map[string]string{
				"access_token": "mock-iam-token-12345",
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.iamURL = server.URL

	result, token := m.checkIBMCloudIAM(ClusterOpts{
		IBMCloudAPIKey: "test-api-key",
	})

	if result.Status != "PASS" {
		t.Errorf("expected PASS, got %s: %s", result.Status, result.Detail)
	}
	if token != "mock-iam-token-12345" {
		t.Errorf("token = %s, want mock-iam-token-12345", token)
	}
	if result.Detail != "authenticated" {
		t.Errorf("detail = %s, want authenticated", result.Detail)
	}
}

func TestCheckIBMCloudIAMAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"errorCode":"BXNIM0415E"}`))
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.iamURL = server.URL

	result, token := m.checkIBMCloudIAM(ClusterOpts{
		IBMCloudAPIKey: "bad-api-key",
	})

	if result.Status != "FAIL" {
		t.Errorf("expected FAIL, got %s", result.Status)
	}
	if token != "" {
		t.Errorf("expected empty token for auth failure, got %s", token)
	}
	if !strings.Contains(result.Detail, "401") {
		t.Errorf("detail should mention 401, got %s", result.Detail)
	}
}

func TestCheckIBMCloudIAMEmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "",
		})
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.iamURL = server.URL

	result, token := m.checkIBMCloudIAM(ClusterOpts{IBMCloudAPIKey: "test"})
	if result.Status != "FAIL" {
		t.Errorf("expected FAIL for empty token, got %s", result.Status)
	}
	if token != "" {
		t.Error("expected empty token")
	}
}

func TestCheckIBMCloudVCPUQuotaSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"instances": []map[string]interface{}{
				{"vcpu": map[string]int{"count": 4}, "status": "running"},
				{"vcpu": map[string]int{"count": 8}, "status": "running"},
				{"vcpu": map[string]int{"count": 4}, "status": "stopped"},
			},
			"total_count": 3,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.vpcURL = server.URL

	results := m.checkIBMCloudVCPUQuota(ClusterOpts{
		Region: "us-south",
	}, "mock-token")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "PASS" {
		t.Errorf("expected PASS, got %s: %s", results[0].Status, results[0].Detail)
	}
	// running: 4+8=12, stopped doesn't count
	if !strings.Contains(results[0].Detail, "12 vCPUs") {
		t.Errorf("expected 12 vCPUs in detail, got %s", results[0].Detail)
	}
}

func TestCheckIBMCloudVCPUQuotaServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.vpcURL = server.URL

	results := m.checkIBMCloudVCPUQuota(ClusterOpts{Region: "us-south"}, "mock-token")
	if len(results) != 1 || results[0].Status != "WARN" {
		t.Errorf("expected WARN for server error, got %+v", results)
	}
}

func TestCheckIBMCloudVCPUQuotaBadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.vpcURL = server.URL

	results := m.checkIBMCloudVCPUQuota(ClusterOpts{Region: "us-south"}, "mock-token")
	if len(results) != 1 || results[0].Status != "WARN" {
		t.Errorf("expected WARN for bad JSON, got %+v", results)
	}
}

func TestCheckIBMCloudIAMConnectionError(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.iamURL = "http://127.0.0.1:1"

	result, token := m.checkIBMCloudIAM(ClusterOpts{IBMCloudAPIKey: "test"})
	if result.Status != "FAIL" {
		t.Errorf("expected FAIL for connection error, got %s", result.Status)
	}
	if token != "" {
		t.Error("expected empty token")
	}
	if !strings.Contains(result.Detail, "failed") {
		t.Errorf("detail should mention failed, got %s", result.Detail)
	}
}

func TestCheckIBMCloudIAMBadJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.iamURL = server.URL

	result, _ := m.checkIBMCloudIAM(ClusterOpts{IBMCloudAPIKey: "test"})
	if result.Status != "FAIL" {
		t.Errorf("expected FAIL for bad JSON, got %s", result.Status)
	}
}

func TestCheckIBMCloudVCPUQuotaWithStartingInstances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"instances": []map[string]interface{}{
				{"vcpu": map[string]int{"count": 4}, "status": "starting"},
				{"vcpu": map[string]int{"count": 8}, "status": "stopping"},
			},
			"total_count": 2,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.vpcURL = server.URL

	results := m.checkIBMCloudVCPUQuota(ClusterOpts{Region: "us-south"}, "mock-token")
	if results[0].Status != "PASS" {
		t.Errorf("expected PASS, got %s", results[0].Status)
	}
	// starting: 4, stopping: 8 = 12 vCPUs
	if !strings.Contains(results[0].Detail, "12 vCPUs") {
		t.Errorf("expected 12 vCPUs, got %s", results[0].Detail)
	}
}

func TestCheckIBMCloudVCPUQuotaConnectionError(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.vpcURL = "http://127.0.0.1:1"

	results := m.checkIBMCloudVCPUQuota(ClusterOpts{Region: "us-south"}, "mock-token")
	if len(results) != 1 || results[0].Status != "WARN" {
		t.Errorf("expected WARN for connection error, got %+v", results)
	}
}

func TestIBMVPCURLDefault(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	url := m.ibmVPCURL("us-south")
	if url != "https://us-south.iaas.cloud.ibm.com" {
		t.Errorf("ibmVPCURL = %s, want default production URL", url)
	}
}

func TestIBMIAMURLDefault(t *testing.T) {
	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	url := m.ibmIAMURL()
	if url != "https://iam.cloud.ibm.com" {
		t.Errorf("ibmIAMURL = %s, want default production URL", url)
	}
}

func TestCheckIBMCloudPreflightWithMockServers(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "mock-token"})
	}))
	defer iamServer.Close()

	vpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"instances":   []interface{}{},
			"total_count": 0,
		})
	}))
	defer vpcServer.Close()

	c := fakeClient()
	m := New(c, testConfig(), discardLogger)
	m.iamURL = iamServer.URL
	m.vpcURL = vpcServer.URL

	results := m.checkIBMCloudPreflight(context.Background(), ClusterOpts{
		Platform:       "ibmcloud",
		IBMCloudAPIKey: "test-api-key",
		Region:         "us-south",
	})

	// Should have ibmcloud-auth PASS and ibmcloud-vcpu-usage PASS
	passCount := 0
	for _, r := range results {
		if r.Status == "PASS" {
			passCount++
		}
	}
	if passCount != 2 {
		t.Errorf("expected 2 PASS results, got %d: %+v", passCount, results)
	}
}
