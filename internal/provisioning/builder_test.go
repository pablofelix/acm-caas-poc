package provisioning

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildManifestsSecretFromYAMLs(t *testing.T) {
	yamls := map[string]string{
		"cred1.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: cred1\n",
		"cred2.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: cred2\n",
	}
	obj := buildManifestsSecretFromYAMLs("spoke1", yamls)

	if obj.GetName() != "spoke1-manifests" {
		t.Errorf("name = %s, want spoke1-manifests", obj.GetName())
	}
	if obj.GetNamespace() != "spoke1" {
		t.Errorf("namespace = %s, want spoke1", obj.GetNamespace())
	}

	data, ok := obj.Object["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data map")
	}
	if len(data) != 2 {
		t.Fatalf("data has %d entries, want 2", len(data))
	}
	for filename, content := range yamls {
		encoded, ok := data[filename].(string)
		if !ok {
			t.Errorf("missing data key %s", filename)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Errorf("invalid base64 for %s: %v", filename, err)
			continue
		}
		if string(decoded) != content {
			t.Errorf("data[%s] = %s, want %s", filename, decoded, content)
		}
	}
}

func TestBuildManifestsSecretFromYAMLs_Empty(t *testing.T) {
	obj := buildManifestsSecretFromYAMLs("ns1", map[string]string{})
	data, _ := obj.Object["data"].(map[string]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d entries", len(data))
	}
}

func TestBuildManagedCluster_AllPlatforms(t *testing.T) {
	tests := []struct {
		platform      string
		expectedCloud string
	}{
		{"ibmcloud", "IBM"},
		{"aws", "Amazon"},
		{"gcp", "Google"},
		{"azure", "Azure"},
		{"other", "Other"},
		{"unknown", "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			mc := buildManagedCluster("test-cluster", tt.platform)
			labels := mc.GetLabels()
			if labels["cloud"] != tt.expectedCloud {
				t.Errorf("cloud label = %s, want %s", labels["cloud"], tt.expectedCloud)
			}
			if labels["vendor"] != "OpenShift" {
				t.Errorf("vendor = %s, want OpenShift", labels["vendor"])
			}
			if labels["acmlab.redhat.com/managed"] != "true" {
				t.Error("expected acmlab.redhat.com/managed=true label")
			}
			if mc.GetName() != "test-cluster" {
				t.Errorf("name = %s, want test-cluster", mc.GetName())
			}
		})
	}
}

func TestBuildManifestsSecret_Success(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("content-a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("content-b"), 0644)
	// Create a subdirectory that should be skipped
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	obj, err := buildManifestsSecret("ns1", dir)
	if err != nil {
		t.Fatalf("buildManifestsSecret failed: %v", err)
	}
	if obj.GetName() != "ns1-manifests" {
		t.Errorf("name = %s, want ns1-manifests", obj.GetName())
	}
	data, _ := obj.Object["data"].(map[string]interface{})
	if len(data) != 2 {
		t.Fatalf("expected 2 data entries, got %d", len(data))
	}
	for _, key := range []string{"a.yaml", "b.yaml"} {
		if _, ok := data[key]; !ok {
			t.Errorf("missing data key %s", key)
		}
	}
}

func TestBuildManifestsSecret_BadDir(t *testing.T) {
	_, err := buildManifestsSecret("ns1", "/nonexistent/path/to/dir")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestBuildManifestsSecret_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	obj, err := buildManifestsSecret("ns1", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := obj.Object["data"].(map[string]interface{})
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d entries", len(data))
	}
}

func TestBuildClusterDeployment_AWSPlatform(t *testing.T) {
	opts := ClusterOpts{
		Name:           "aws-cluster",
		Platform:       "aws",
		BaseDomain:     "example.com",
		Region:         "us-east-1",
		ImageSet:       "img4.20",
		WorkerType:     "m5.xlarge",
		MasterType:     "m5.2xlarge",
		WorkerReplicas: 3,
		MasterReplicas: 3,
		SSHKey:         "ssh-rsa AAAA...",
	}
	cd := buildClusterDeployment(opts)

	spec, _ := cd.Object["spec"].(map[string]interface{})
	platform, _ := spec["platform"].(map[string]interface{})
	aws, ok := platform["aws"].(map[string]interface{})
	if !ok {
		t.Fatal("expected aws platform spec")
	}
	if aws["region"] != "us-east-1" {
		t.Errorf("region = %v, want us-east-1", aws["region"])
	}
	credsRef, ok := aws["credentialsSecretRef"].(map[string]interface{})
	if !ok {
		t.Fatal("expected credentialsSecretRef for aws")
	}
	if credsRef["name"] != "aws-cluster-aws-creds" {
		t.Errorf("credentialsSecretRef name = %v, want aws-cluster-aws-creds", credsRef["name"])
	}

	prov, _ := spec["provisioning"].(map[string]interface{})
	// AWS should not have manifestsSecretRef
	if _, ok := prov["manifestsSecretRef"]; ok {
		t.Error("AWS should not have manifestsSecretRef")
	}
	// No SSHPrivateKey set, so no sshPrivateKeySecretRef
	if _, ok := prov["sshPrivateKeySecretRef"]; ok {
		t.Error("should not have sshPrivateKeySecretRef when SSHPrivateKey is empty")
	}
}

func TestBuildClusterDeployment_IBMCloudPlatform(t *testing.T) {
	opts := ClusterOpts{
		Name:           "ibm-cluster",
		Platform:       "ibmcloud",
		BaseDomain:     "example.com",
		Region:         "us-south",
		ImageSet:       "img4.20",
		SSHPrivateKey:  "private-key-data",
	}
	cd := buildClusterDeployment(opts)

	spec, _ := cd.Object["spec"].(map[string]interface{})
	platform, _ := spec["platform"].(map[string]interface{})
	ibm, ok := platform["ibmcloud"].(map[string]interface{})
	if !ok {
		t.Fatal("expected ibmcloud platform spec")
	}
	credsRef, ok := ibm["credentialsSecretRef"].(map[string]interface{})
	if !ok {
		t.Fatal("expected credentialsSecretRef for ibmcloud")
	}
	if credsRef["name"] != "ibm-cluster-ibmcloud-creds" {
		t.Errorf("credentialsSecretRef name = %v", credsRef["name"])
	}

	prov, _ := spec["provisioning"].(map[string]interface{})
	manifestsRef, ok := prov["manifestsSecretRef"].(map[string]interface{})
	if !ok {
		t.Fatal("expected manifestsSecretRef for ibmcloud")
	}
	if manifestsRef["name"] != "ibm-cluster-manifests" {
		t.Errorf("manifestsSecretRef name = %v", manifestsRef["name"])
	}

	sshRef, ok := prov["sshPrivateKeySecretRef"].(map[string]interface{})
	if !ok {
		t.Fatal("expected sshPrivateKeySecretRef when SSHPrivateKey is set")
	}
	if sshRef["name"] != "ibm-cluster-ssh-private-key" {
		t.Errorf("sshPrivateKeySecretRef name = %v", sshRef["name"])
	}
}

func TestGenerateInstallConfig_IBMCloud(t *testing.T) {
	opts := ClusterOpts{
		Name:           "test",
		Platform:       "ibmcloud",
		BaseDomain:     "example.com",
		Region:         "us-south",
		MasterType:     "bx2-8x32",
		WorkerType:     "bx2-4x16",
		MasterReplicas: 3,
		WorkerReplicas: 2,
		SSHKey:         "ssh-rsa AAAA",
	}
	cfg := generateInstallConfig(opts)
	if !strings.Contains(cfg, "credentialsMode: Manual") {
		t.Error("expected credentialsMode: Manual for ibmcloud")
	}
	if !strings.Contains(cfg, "baseDomain: example.com") {
		t.Error("expected baseDomain in config")
	}
}

func TestGenerateInstallConfig_NonIBMCloud(t *testing.T) {
	opts := ClusterOpts{
		Name:           "test",
		Platform:       "aws",
		BaseDomain:     "example.com",
		Region:         "us-east-1",
		MasterType:     "m5.2xlarge",
		WorkerType:     "m5.xlarge",
		MasterReplicas: 3,
		WorkerReplicas: 3,
		SSHKey:         "ssh-rsa AAAA",
	}
	cfg := generateInstallConfig(opts)
	if strings.Contains(cfg, "credentialsMode") {
		t.Error("non-ibmcloud should not have credentialsMode")
	}
}

func TestBuildNamespace(t *testing.T) {
	ns := buildNamespace("test-ns")
	if ns.GetName() != "test-ns" {
		t.Errorf("name = %s, want test-ns", ns.GetName())
	}
	if ns.GetKind() != "Namespace" {
		t.Errorf("kind = %s, want Namespace", ns.GetKind())
	}
}

func TestBuildCredentialsSecret(t *testing.T) {
	obj := buildCredentialsSecret("spoke1", ClusterOpts{Platform: "ibmcloud", IBMCloudAPIKey: "my-api-key"})
	if obj.GetName() != "spoke1-ibmcloud-creds" {
		t.Errorf("name = %s, want spoke1-ibmcloud-creds", obj.GetName())
	}
	if obj.GetNamespace() != "spoke1" {
		t.Errorf("namespace = %s, want spoke1", obj.GetNamespace())
	}
	data, _ := obj.Object["data"].(map[string]interface{})
	decoded, _ := base64.StdEncoding.DecodeString(data["ibmcloud_api_key"].(string))
	if string(decoded) != "my-api-key" {
		t.Errorf("api key = %s, want my-api-key", decoded)
	}
}

func TestBuildPullSecret(t *testing.T) {
	obj := buildPullSecret("spoke1", `{"auths":{}}`)
	if obj.GetName() != "spoke1-pull-secret" {
		t.Errorf("name = %s, want spoke1-pull-secret", obj.GetName())
	}
	data, _ := obj.Object["data"].(map[string]interface{})
	decoded, _ := base64.StdEncoding.DecodeString(data[".dockerconfigjson"].(string))
	if string(decoded) != `{"auths":{}}` {
		t.Errorf("pull secret decoded incorrectly: %s", decoded)
	}
}

func TestBuildSSHPrivateKeySecret(t *testing.T) {
	obj := buildSSHPrivateKeySecret("spoke1", "private-key-data")
	if obj.GetName() != "spoke1-ssh-private-key" {
		t.Errorf("name = %s, want spoke1-ssh-private-key", obj.GetName())
	}
	data, _ := obj.Object["data"].(map[string]interface{})
	decoded, _ := base64.StdEncoding.DecodeString(data["ssh-privatekey"].(string))
	if string(decoded) != "private-key-data" {
		t.Errorf("ssh key = %s, want private-key-data", decoded)
	}
}

func TestBuildKlusterletAddonConfig(t *testing.T) {
	kac := buildKlusterletAddonConfig("spoke1")
	if kac.GetName() != "spoke1" {
		t.Errorf("name = %s, want spoke1", kac.GetName())
	}
	if kac.GetNamespace() != "spoke1" {
		t.Errorf("namespace = %s, want spoke1", kac.GetNamespace())
	}
	spec, _ := kac.Object["spec"].(map[string]interface{})
	for _, key := range []string{"applicationManager", "certPolicyController", "policyController", "searchCollector"} {
		sub, ok := spec[key].(map[string]interface{})
		if !ok {
			t.Errorf("spec.%s missing", key)
			continue
		}
		if sub["enabled"] != true {
			t.Errorf("spec.%s.enabled = %v, want true", key, sub["enabled"])
		}
	}
}
