package mcp

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// extractToolResultPair returns (text, isError) from a tool response.
func extractToolResultPair(t *testing.T, resp mcplib.JSONRPCMessage) (string, bool) {
	t.Helper()
	rpcResp, ok := resp.(mcplib.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	data, _ := json.Marshal(rpcResp.Result)
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	text := ""
	if len(result.Content) > 0 {
		text = result.Content[0].Text
	}
	return text, result.IsError
}

// --- Helper constructors ---

func mkClusterDeployment(name string, installed bool) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	obj.SetName(name)
	obj.SetNamespace(name)
	obj.SetLabels(map[string]string{"acmlab.redhat.com/managed": "true"})
	obj.Object["spec"] = map[string]interface{}{
		"baseDomain": "example.com",
		"platform": map[string]interface{}{
			"ibmcloud": map[string]interface{}{"region": "us-south"},
		},
		"provisioning": map[string]interface{}{
			"imageSetRef": map[string]interface{}{"name": "ocp-4.15"},
		},
		"powerState": "Running",
	}
	conditions := []interface{}{}
	if installed {
		conditions = append(conditions, map[string]interface{}{
			"type": "Provisioned", "status": "True",
		})
	}
	obj.Object["status"] = map[string]interface{}{
		"installed":  installed,
		"powerState": "Running",
		"conditions": conditions,
	}
	return obj
}

func mkClusterDeploymentWithPower(name, specPower, statusPower string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterDeployment",
	})
	obj.SetName(name)
	obj.SetNamespace(name)
	spec := map[string]interface{}{}
	if specPower != "" {
		spec["powerState"] = specPower
	}
	obj.Object["spec"] = spec
	status := map[string]interface{}{}
	if statusPower != "" {
		status["powerState"] = statusPower
	}
	obj.Object["status"] = status
	return obj
}

func mkClusterImageSet(name, releaseImage string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "ClusterImageSet",
	})
	obj.SetName(name)
	obj.Object["spec"] = map[string]interface{}{
		"releaseImage": releaseImage,
	}
	return obj
}

func mkMachinePool(clusterName string, replicas int64) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "hive.openshift.io", Version: "v1", Kind: "MachinePool",
	})
	obj.SetName(clusterName + "-worker")
	obj.SetNamespace(clusterName)
	obj.Object["spec"] = map[string]interface{}{
		"clusterDeploymentRef": map[string]interface{}{"name": clusterName},
		"name":                 "worker",
		"replicas":             replicas,
		"platform": map[string]interface{}{
			"ibmcloud": map[string]interface{}{"type": "bx2-8x32"},
		},
	}
	return obj
}

// --- splitTrim ---

func TestSplitTrimBasic(t *testing.T) {
	got := splitTrim("a, b, c")
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("splitTrim(\"a, b, c\") = %v", got)
	}
}

func TestSplitTrimEmpty(t *testing.T) {
	if got := splitTrim(""); len(got) != 0 {
		t.Errorf("splitTrim(\"\") = %v, want empty", got)
	}
}

func TestSplitTrimBlanks(t *testing.T) {
	if got := splitTrim("a,,b, ,c"); len(got) != 3 {
		t.Errorf("splitTrim with blanks = %v, want 3 elements", got)
	}
}

func TestSplitTrimOnlyCommas(t *testing.T) {
	if got := splitTrim(",,,"); len(got) != 0 {
		t.Errorf("splitTrim(\",,,\") = %v, want empty", got)
	}
}

// --- Provisioning tools ---

func TestProvisionCreateViaMCP(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-access-key")
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_provision_create", map[string]interface{}{
		"name":        "test-cluster",
		"platform":    "aws",
		"pull_secret": `{"auths":{}}`,
	})
	text := extractToolText(t, resp)
	if text == "" {
		t.Error("expected non-empty response")
	}
}

func TestProvisionDestroyViaMCP(t *testing.T) {
	cd := mkClusterDeployment("my-cluster", true)
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_provision_destroy", map[string]interface{}{
		"name": "my-cluster",
	})
	text := extractToolText(t, resp)
	if text != "Cluster my-cluster destruction initiated" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestProvisionDestroyNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_provision_destroy", map[string]interface{}{
		"name": "nonexistent",
	})
	text, isErr := extractToolResult(t, resp)
	if !isErr {
		t.Errorf("expected error for nonexistent cluster, got success: %s", text)
	}
}

func TestProvisionStatusViaMCP(t *testing.T) {
	cd := mkClusterDeployment("my-cluster", true)
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_provision_status", map[string]interface{}{
		"name": "my-cluster",
	})
	text := extractToolText(t, resp)
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info["Installed"] != true {
		t.Errorf("Installed = %v, want true", info["Installed"])
	}
}

func TestProvisionStatusNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_provision_status", map[string]interface{}{
		"name": "nonexistent",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestProvisionListViaMCP(t *testing.T) {
	cd := mkClusterDeployment("my-cluster", true)
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_provision_list", nil)
	text := extractToolText(t, resp)
	var clusters []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &clusters); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(clusters) != 1 {
		t.Errorf("got %d clusters, want 1", len(clusters))
	}
}

func TestProvisionListEmpty(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_provision_list", nil)
	text := extractToolText(t, resp)
	var clusters []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &clusters); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(clusters) != 0 {
		t.Errorf("got %d clusters, want 0", len(clusters))
	}
}

func TestListImageSetsViaMCP(t *testing.T) {
	is := mkClusterImageSet("ocp-4.15", "quay.io/openshift-release-dev/ocp-release:4.15.0-x86_64")
	c := fakeClientWithClusters(is)
	resp := callTool(t, c, "acm_list_image_sets", nil)
	text := extractToolText(t, resp)
	var sets []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &sets); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sets) != 1 {
		t.Errorf("got %d sets, want 1", len(sets))
	}
}

func TestListImageSetsEmpty(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_list_image_sets", nil)
	text := extractToolText(t, resp)
	var sets []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &sets); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sets) != 0 {
		t.Errorf("got %d sets, want 0", len(sets))
	}
}

// --- Lifecycle tools ---

func TestHibernateClusterViaMCP(t *testing.T) {
	cd := mkClusterDeploymentWithPower("cluster-1", "Running", "Running")
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_hibernate_cluster", map[string]interface{}{
		"name": "cluster-1",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "hibernate" {
		t.Errorf("action = %v, want hibernate", result["action"])
	}
	if result["status"] != "initiated" {
		t.Errorf("status = %v, want initiated", result["status"])
	}
}

func TestHibernateClusterNotSupported(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_hibernate_cluster", map[string]interface{}{
		"name": "imported-cluster",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for unsupported cluster")
	}
}

func TestHibernateWithNamespaceOverride(t *testing.T) {
	cd := mkClusterDeploymentWithPower("cluster-1", "Running", "Running")
	cd.SetNamespace("custom-ns")
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_hibernate_cluster", map[string]interface{}{
		"name":      "cluster-1",
		"namespace": "custom-ns",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["cluster"] != "custom-ns/cluster-1" {
		t.Errorf("cluster = %v, want custom-ns/cluster-1", result["cluster"])
	}
}

func TestResumeClusterViaMCP(t *testing.T) {
	cd := mkClusterDeploymentWithPower("cluster-1", "Hibernating", "Hibernating")
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_resume_cluster", map[string]interface{}{
		"name": "cluster-1",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "resume" {
		t.Errorf("action = %v, want resume", result["action"])
	}
}

func TestResumeClusterNotSupported(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_resume_cluster", map[string]interface{}{
		"name": "imported-cluster",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for unsupported cluster")
	}
}

func TestLifecycleStatusViaMCP(t *testing.T) {
	cd := mkClusterDeploymentWithPower("cluster-1", "Running", "Running")
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_lifecycle_status", map[string]interface{}{
		"name": "cluster-1",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["desiredState"] != "Running" {
		t.Errorf("desiredState = %v, want Running", result["desiredState"])
	}
	if result["transitioning"] != false {
		t.Errorf("transitioning = %v, want false", result["transitioning"])
	}
}

func TestLifecycleStatusTransitioning(t *testing.T) {
	cd := mkClusterDeploymentWithPower("cluster-1", "Hibernating", "Running")
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_lifecycle_status", map[string]interface{}{
		"name": "cluster-1",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["transitioning"] != true {
		t.Errorf("transitioning = %v, want true", result["transitioning"])
	}
}

func TestLifecycleStatusNotSupported(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_lifecycle_status", map[string]interface{}{
		"name": "imported-cluster",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for unsupported cluster")
	}
}

func TestLifecycleDiagnoseViaMCP(t *testing.T) {
	cd := mkClusterDeploymentWithPower("cluster-1", "Running", "Running")
	mc := newManagedCluster("cluster-1", true)
	c := fakeClientWithClusters(cd, mc)
	resp := callTool(t, c, "acm_lifecycle_diagnose", map[string]interface{}{
		"name": "cluster-1",
	})
	text := extractToolText(t, resp)
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report["hivePowerSpec"] != "Running" {
		t.Errorf("hivePowerSpec = %v, want Running", report["hivePowerSpec"])
	}
}

func TestLifecycleDiagnoseNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_lifecycle_diagnose", map[string]interface{}{
		"name": "nonexistent",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestLifecycleDiagnoseNoManagedCluster(t *testing.T) {
	cd := mkClusterDeploymentWithPower("orphan", "Running", "Running")
	c := fakeClientWithClusters(cd)
	resp := callTool(t, c, "acm_lifecycle_diagnose", map[string]interface{}{
		"name": "orphan",
	})
	text := extractToolText(t, resp)
	var report map[string]interface{}
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if report["acmAvailable"] != "not found" {
		t.Errorf("acmAvailable = %v, want 'not found'", report["acmAvailable"])
	}
}

func TestLifecycleRecoverCertsNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_lifecycle_recover_certs", map[string]interface{}{
		"name": "nonexistent",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestListLifecycleClustersViaMCP(t *testing.T) {
	cd1 := mkClusterDeploymentWithPower("cluster-1", "Running", "Running")
	cd2 := mkClusterDeploymentWithPower("cluster-2", "Hibernating", "Hibernating")
	c := fakeClientWithClusters(cd1, cd2)
	resp := callTool(t, c, "acm_list_lifecycle_clusters", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 2 {
		t.Errorf("count = %v, want 2", result["count"])
	}
}

func TestListLifecycleClustersEmpty(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_list_lifecycle_clusters", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 0 {
		t.Errorf("count = %v, want 0", result["count"])
	}
}

// --- Import tools ---

func TestImportClusterViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_cluster", map[string]interface{}{
		"name": "ext-cluster",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["name"] != "ext-cluster" {
		t.Errorf("name = %v, want ext-cluster", result["name"])
	}
	if result["autoImport"] != false {
		t.Errorf("autoImport = %v, want false", result["autoImport"])
	}
}

func TestImportClusterWithKubeconfigViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	kubeconfig := base64.StdEncoding.EncodeToString([]byte("apiVersion: v1\nkind: Config\nclusters: []\n"))
	resp := callTool(t, c, "acm_import_cluster", map[string]interface{}{
		"name":       "ext-cluster",
		"kubeconfig": kubeconfig,
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["autoImport"] != true {
		t.Errorf("autoImport = %v, want true", result["autoImport"])
	}
}

func TestImportClusterWithLabelsViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_cluster", map[string]interface{}{
		"name":   "ext-cluster",
		"labels": "env=dev,tier=edge",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["name"] != "ext-cluster" {
		t.Errorf("name = %v, want ext-cluster", result["name"])
	}
}

func TestImportClusterWithClusterSetViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_cluster", map[string]interface{}{
		"name":        "ext-cluster",
		"cluster_set": "production",
	})
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["name"] != "ext-cluster" {
		t.Errorf("name = %v, want ext-cluster", result["name"])
	}
}

func TestImportClusterInvalidKubeconfigViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_cluster", map[string]interface{}{
		"name":       "ext-cluster",
		"kubeconfig": "not-valid-base64!!!",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for invalid base64 kubeconfig")
	}
}

func TestImportClusterMissingNameViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_cluster", map[string]interface{}{})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for missing name")
	}
}

func TestDetachClusterViaMCP(t *testing.T) {
	mc := newManagedCluster("ext-cluster", true)
	c := fakeClientWithClusters(mc)
	resp := callTool(t, c, "acm_detach_cluster", map[string]interface{}{
		"name": "ext-cluster",
	})
	text := extractToolText(t, resp)
	if text != "Cluster ext-cluster detached from ACM" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestDetachClusterMissingNameViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_detach_cluster", map[string]interface{}{})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for missing name")
	}
}

func TestImportStatusViaMCP(t *testing.T) {
	mc := newManagedCluster("ext-cluster", true)
	c := fakeClientWithClusters(mc)
	resp := callTool(t, c, "acm_import_status", map[string]interface{}{
		"name": "ext-cluster",
	})
	text := extractToolText(t, resp)
	var status map[string]interface{}
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if status["name"] != "ext-cluster" {
		t.Errorf("name = %v, want ext-cluster", status["name"])
	}
}

func TestImportStatusMissingNameViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_status", map[string]interface{}{})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for missing name")
	}
}

func TestImportStatusNotFoundViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_import_status", map[string]interface{}{
		"name": "nonexistent",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestListImportedClustersViaMCP(t *testing.T) {
	mc := newManagedCluster("ext-cluster", true)
	c := fakeClientWithClusters(mc)
	resp := callTool(t, c, "acm_list_imported_clusters", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 1 {
		t.Errorf("count = %v, want 1", result["count"])
	}
}

func TestListImportedClustersEmptyViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_list_imported_clusters", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 0 {
		t.Errorf("count = %v, want 0", result["count"])
	}
}

func TestListImportedExcludesHiveClustersViaMCP(t *testing.T) {
	mc := newManagedCluster("hive-cluster", true)
	cd := mkClusterDeployment("hive-cluster", true)
	c := fakeClientWithClusters(mc, cd)
	resp := callTool(t, c, "acm_list_imported_clusters", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 0 {
		t.Errorf("count = %v, want 0 (hive cluster should be excluded)", result["count"])
	}
}

// --- Registry tools ---

func TestRegistryListImagesViaMCP(t *testing.T) {
	mw := &unstructured.Unstructured{}
	mw.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "work.open-cluster-management.io", Version: "v1", Kind: "ManifestWork",
	})
	mw.SetName("test-mw")
	mw.SetNamespace("my-cluster")
	c := fakeClientWithClusters(mw)
	resp := callTool(t, c, "acm_registry_list_images", map[string]interface{}{
		"cluster": "my-cluster",
	})
	text := extractToolText(t, resp)
	if text == "" {
		t.Error("expected non-empty response")
	}
}

func TestRegistryConfigureMirrorViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_registry_configure_mirror", map[string]interface{}{
		"cluster": "my-cluster",
		"mirror":  "quay.io/myorg",
	})
	text := extractToolText(t, resp)
	if text != "Image registry mirror configured for cluster my-cluster (mirror: quay.io/myorg)" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestRegistryMirrorStatusNotConfiguredViaMCP(t *testing.T) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster",
	})
	mc.SetName("my-cluster")
	c := fakeClientWithClusters(mc)
	resp := callTool(t, c, "acm_registry_mirror_status", map[string]interface{}{
		"cluster": "my-cluster",
	})
	text := extractToolText(t, resp)
	var status map[string]interface{}
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if status["configured"] != false {
		t.Errorf("configured = %v, want false", status["configured"])
	}
}

func TestRegistryMirrorStatusNonexistentClusterViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_registry_mirror_status", map[string]interface{}{
		"cluster": "nonexistent",
	})
	text, isError := extractToolResult(t, resp)
	if !isError {
		t.Error("expected error for nonexistent cluster")
	}
	if !strings.Contains(text, "not found") {
		t.Errorf("expected not-found error, got: %s", text)
	}
}

func TestRegistryGenerateMirrorScriptViaMCP(t *testing.T) {
	mw := &unstructured.Unstructured{}
	mw.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "work.open-cluster-management.io", Version: "v1", Kind: "ManifestWork",
	})
	mw.SetName("test-mw")
	mw.SetNamespace("my-cluster")
	c := fakeClientWithClusters(mw)
	resp := callTool(t, c, "acm_registry_generate_mirror_script", map[string]interface{}{
		"cluster": "my-cluster",
		"target":  "quay.io/myorg",
	})
	text := extractToolText(t, resp)
	if text == "" {
		t.Error("expected non-empty script")
	}
}

// --- Scaling tools ---

func TestScalingGetViaMCP(t *testing.T) {
	mp := mkMachinePool("my-cluster", 3)
	c := fakeClientWithClusters(mp)
	resp := callTool(t, c, "acm_scaling_get", map[string]interface{}{
		"cluster": "my-cluster",
	})
	text := extractToolText(t, resp)
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info["name"] != "my-cluster-worker" {
		t.Errorf("name = %v, want my-cluster-worker", info["name"])
	}
}

func TestScalingGetNotFoundViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_scaling_get", map[string]interface{}{
		"cluster": "nonexistent",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestScalingSetViaMCP(t *testing.T) {
	mp := mkMachinePool("my-cluster", 3)
	c := fakeClientWithClusters(mp)
	resp := callTool(t, c, "acm_scaling_set", map[string]interface{}{
		"cluster":  "my-cluster",
		"replicas": float64(5),
	})
	text := extractToolText(t, resp)
	if text != "Cluster my-cluster worker replicas set to 5" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestScalingSetNotFoundViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_scaling_set", map[string]interface{}{
		"cluster":  "nonexistent",
		"replicas": float64(3),
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestScalingAutoViaMCP(t *testing.T) {
	mp := mkMachinePool("my-cluster", 3)
	c := fakeClientWithClusters(mp)
	resp := callTool(t, c, "acm_scaling_auto", map[string]interface{}{
		"cluster": "my-cluster",
		"min":     float64(2),
		"max":     float64(5),
	})
	text := extractToolText(t, resp)
	if text != "Cluster my-cluster MachinePool autoscaling enabled (min=2 max=5)" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestScalingAutoMinGeMaxViaMCP(t *testing.T) {
	mp := mkMachinePool("my-cluster", 3)
	c := fakeClientWithClusters(mp)
	resp := callTool(t, c, "acm_scaling_auto", map[string]interface{}{
		"cluster": "my-cluster",
		"min":     float64(5),
		"max":     float64(3),
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error when min >= max")
	}
}

func TestScalingAutoNotFoundViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_scaling_auto", map[string]interface{}{
		"cluster": "nonexistent",
		"min":     float64(1),
		"max":     float64(3),
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestScalingListViaMCP(t *testing.T) {
	mp1 := mkMachinePool("cluster-1", 3)
	mp2 := mkMachinePool("cluster-2", 5)
	c := fakeClientWithClusters(mp1, mp2)
	resp := callTool(t, c, "acm_scaling_list", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 2 {
		t.Errorf("count = %v, want 2", result["count"])
	}
}

func TestScalingListEmptyViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_scaling_list", nil)
	text := extractToolText(t, resp)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(result["count"].(float64)) != 0 {
		t.Errorf("count = %v, want 0", result["count"])
	}
}

// --- Additional coverage for policy with registries ---

func TestApplyPolicyWithRegistriesViaMCP(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_apply_policy", map[string]interface{}{
		"name":        "registry-policy",
		"registries":  "quay.io, registry.redhat.io",
		"remediation": "enforce",
	})
	text := extractToolText(t, resp)
	if text != "Policy registry-policy applied successfully" {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_get_policy", map[string]interface{}{
		"name": "nonexistent",
	})
	_, isErr := extractToolResultPair(t, resp)
	if !isErr {
		t.Error("expected error for nonexistent policy")
	}
}

func TestDeployTenantWithAllOpts(t *testing.T) {
	c := fakeClientWithClusters()
	resp := callTool(t, c, "acm_deploy_tenant", map[string]interface{}{
		"name":    "team-beta",
		"cluster": "spoke1",
		"team":    "backend-team",
		"cpu":     "8",
		"memory":  "16Gi",
	})
	text := extractToolText(t, resp)
	if text != "Tenant team-beta deployed to spoke1" {
		t.Errorf("unexpected response: %s", text)
	}
}
