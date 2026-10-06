package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

type CAPISetupOpts struct {
	InfraProviders []string
	Region         string
	SSHKeyName     string
	WaitTimeout    time.Duration
}

type CAPISetupResult struct {
	ControllersInstalled bool
	CRDsUpgraded         bool
	WebhooksUpgraded     bool
	SCCsGranted          bool
	SSHKeyCreated        bool
	SSHKeyName           string
	PodsReady            bool
	Errors               []string
}

type CAPIControllerStatus struct {
	Namespace  string
	Deployment string
	Ready      bool
	Image      string
}

var capiNamespaces = map[string]struct {
	Deployment     string
	ServiceAccount string
}{
	"capi-system":                         {"capi-controller-manager", "capi-manager"},
	"capi-kubeadm-bootstrap-system":       {"capi-kubeadm-bootstrap-controller-manager", "capi-kubeadm-bootstrap-manager"},
	"capi-kubeadm-control-plane-system":   {"capi-kubeadm-control-plane-controller-manager", "capi-kubeadm-control-plane-manager"},
}

var capiInfraNamespaces = map[string]struct {
	Namespace      string
	Deployment     string
	ServiceAccount string
}{
	"aws": {"capa-system", "capa-controller-manager", "capa-controller-manager"},
}

func (m *Manager) SetupCAPIControllers(ctx context.Context, opts CAPISetupOpts) (*CAPISetupResult, error) {
	m.logger.Info("provisioning.SetupCAPIControllers", "providers", opts.InfraProviders)
	result := &CAPISetupResult{}

	if len(opts.InfraProviders) == 0 {
		opts.InfraProviders = []string{"aws"}
	}
	if opts.WaitTimeout == 0 {
		opts.WaitTimeout = 5 * time.Minute
	}

	installed, err := m.areCAPIControllersInstalled(ctx)
	if err != nil {
		return result, fmt.Errorf("checking CAPI controllers: %w", err)
	}
	if installed {
		m.logger.Info("CAPI controllers already installed, skipping clusterctl init")
		result.ControllersInstalled = true
	} else {
		if err := m.installCAPIControllers(ctx, opts); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("controller install: %s", err))
			return result, fmt.Errorf("installing CAPI controllers: %w", err)
		}
		result.ControllersInstalled = true
	}

	upgraded, err := m.upgradeCAPICRDs(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("CRD upgrade: %s", err))
		m.logger.Warn("failed to upgrade CRDs to v1beta2", "error", err)
	} else {
		result.CRDsUpgraded = upgraded
	}

	whUpgraded, err := m.upgradeCAPIWebhooks(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("webhook upgrade: %s", err))
		m.logger.Warn("failed to upgrade webhooks to v1beta2", "error", err)
	} else {
		result.WebhooksUpgraded = whUpgraded
	}

	if err := m.grantCAPISCCs(ctx, opts.InfraProviders); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("SCC grant: %s", err))
		m.logger.Warn("failed to grant SCCs", "error", err)
	} else {
		result.SCCsGranted = true
	}

	if err := m.restartCAPIDeployments(ctx, opts.InfraProviders); err != nil {
		m.logger.Warn("failed to restart deployments", "error", err)
	}

	if err := m.waitForCAPIPods(ctx, opts); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("pods not ready: %s", err))
	} else {
		result.PodsReady = true
	}

	if opts.SSHKeyName != "" {
		for _, provider := range opts.InfraProviders {
			if provider == "aws" {
				created, err := m.EnsureAWSSSHKeyPair(ctx, opts.Region, opts.SSHKeyName)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("SSH key: %s", err))
				} else {
					result.SSHKeyCreated = created
					result.SSHKeyName = opts.SSHKeyName
				}
			}
		}
	}

	return result, nil
}

func (m *Manager) areCAPIControllersInstalled(ctx context.Context) (bool, error) {
	for ns, info := range capiNamespaces {
		_, err := m.client.Get(ctx, client.GVRDeployment, ns, info.Deployment)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
	}
	return true, nil
}

func (m *Manager) installCAPIControllers(ctx context.Context, opts CAPISetupOpts) error {
	args := []string{"init", "--bootstrap", "kubeadm", "--control-plane", "kubeadm"}
	for _, provider := range opts.InfraProviders {
		args = append(args, "--infrastructure", provider)
	}
	m.logger.Info("running clusterctl init", "args", args)
	out, err := m.runClusterctl(args...)
	if err != nil {
		return fmt.Errorf("clusterctl init failed: %w\noutput: %s", err, string(out))
	}
	m.logger.Info("clusterctl init completed", "output", string(out))
	return nil
}

func (m *Manager) upgradeCAPICRDs(ctx context.Context) (bool, error) {
	crd, err := m.client.Get(ctx, client.GVRCustomResourceDefinition, "", "machines.cluster.x-k8s.io")
	if err != nil {
		return false, fmt.Errorf("getting Machine CRD: %w", err)
	}

	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	for _, v := range versions {
		vMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if name, _ := vMap["name"].(string); name == "v1beta2" {
			m.logger.Info("CRDs already serve v1beta2, skipping upgrade")
			return false, nil
		}
	}

	m.logger.Info("CRDs need v1beta2 upgrade, applying v1.14 CRDs")
	out, err := m.runKubectl("apply", "--server-side", "--force-conflicts",
		"-f", "https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.14.2/cluster-api-components.yaml",
		"--selector", "apiextensions.k8s.io/v1=CustomResourceDefinition",
	)
	if err != nil {
		out2, err2 := m.applyCRDsViaDownload(ctx)
		if err2 != nil {
			return false, fmt.Errorf("applying CRDs: %w (first attempt: %s)", err2, string(out))
		}
		m.logger.Info("CRDs applied via download fallback", "output", string(out2))
		return true, nil
	}
	m.logger.Info("CRDs upgraded to v1beta2", "output", string(out))
	return true, nil
}

func (m *Manager) applyCRDsViaDownload(ctx context.Context) ([]byte, error) {
	if m.curlExec != nil {
		if _, err := m.curlExec("-sL",
			"https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.14.2/cluster-api-components.yaml",
			"-o", "/tmp/capi-components-v1142.yaml",
		); err != nil {
			return nil, fmt.Errorf("downloading CRDs: %w", err)
		}
	} else {
		curlOut, err := exec.Command("curl", "-sL",
			"https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.14.2/cluster-api-components.yaml",
			"-o", "/tmp/capi-components-v1142.yaml",
		).CombinedOutput()
		if err != nil {
			return curlOut, fmt.Errorf("downloading CRDs: %w", err)
		}
	}

	return m.runKubectl("apply", "--server-side", "--force-conflicts", "-f", "/tmp/capi-components-v1142.yaml")
}

var capiWebhookConfigs = []struct {
	Kind string
	Name string
}{
	{"mutatingwebhookconfigurations", "capi-mutating-webhook-configuration"},
	{"mutatingwebhookconfigurations", "capi-kubeadm-bootstrap-mutating-webhook-configuration"},
	{"mutatingwebhookconfigurations", "capi-kubeadm-control-plane-mutating-webhook-configuration"},
	{"validatingwebhookconfigurations", "capi-validating-webhook-configuration"},
	{"validatingwebhookconfigurations", "capi-kubeadm-bootstrap-validating-webhook-configuration"},
	{"validatingwebhookconfigurations", "capi-kubeadm-control-plane-validating-webhook-configuration"},
}

func (m *Manager) upgradeCAPIWebhooks(ctx context.Context) (bool, error) {
	anyUpgraded := false
	for _, wh := range capiWebhookConfigs {
		out, err := m.runKubectl("get", wh.Kind[:len(wh.Kind)-1], wh.Name, "-o", "json")
		if err != nil {
			m.logger.Info("webhook config not found, skipping", "name", wh.Name)
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(out, &data); err != nil {
			return anyUpgraded, fmt.Errorf("parsing webhook %s: %w", wh.Name, err)
		}

		changed := false
		webhooks, _ := data["webhooks"].([]interface{})
		for _, raw := range webhooks {
			webhook, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			clientConfig, _ := webhook["clientConfig"].(map[string]interface{})
			service, _ := clientConfig["service"].(map[string]interface{})
			if path, _ := service["path"].(string); strings.Contains(path, "v1beta1") {
				service["path"] = strings.ReplaceAll(path, "v1beta1", "v1beta2")
				changed = true
			}
			rules, _ := webhook["rules"].([]interface{})
			for _, rr := range rules {
				rule, ok := rr.(map[string]interface{})
				if !ok {
					continue
				}
				versions, _ := rule["apiVersions"].([]interface{})
				for i, v := range versions {
					if vs, _ := v.(string); vs == "v1beta1" {
						versions[i] = "v1beta2"
						changed = true
					}
				}
			}
		}

		if !changed {
			continue
		}

		meta, _ := data["metadata"].(map[string]interface{})
		delete(meta, "resourceVersion")
		delete(meta, "managedFields")
		delete(meta, "creationTimestamp")

		patchJSON, err := json.Marshal(data)
		if err != nil {
			return anyUpgraded, fmt.Errorf("marshaling webhook %s: %w", wh.Name, err)
		}

		tmpFile := fmt.Sprintf("/tmp/capi-webhook-%s.json", wh.Name)
		if err := os.WriteFile(tmpFile, patchJSON, 0600); err != nil {
			return anyUpgraded, fmt.Errorf("writing webhook %s: %w", wh.Name, err)
		}
		if _, err = m.runKubectl("apply", "-f", tmpFile); err != nil {
			return anyUpgraded, fmt.Errorf("applying webhook %s: %w", wh.Name, err)
		}
		os.Remove(tmpFile)
		m.logger.Info("upgraded webhook to v1beta2", "name", wh.Name)
		anyUpgraded = true
	}
	return anyUpgraded, nil
}

func (m *Manager) grantCAPISCCs(ctx context.Context, infraProviders []string) error {
	allSAs := make([]string, 0)
	for ns, info := range capiNamespaces {
		allSAs = append(allSAs, fmt.Sprintf("system:serviceaccount:%s:%s", ns, info.ServiceAccount))
	}
	for _, provider := range infraProviders {
		if infra, ok := capiInfraNamespaces[provider]; ok {
			allSAs = append(allSAs, fmt.Sprintf("system:serviceaccount:%s:%s", infra.Namespace, infra.ServiceAccount))
		}
	}

	for _, sa := range allSAs {
		out, err := m.runOC("adm", "policy", "add-scc-to-user", "privileged", sa)
		if err != nil {
			return fmt.Errorf("granting privileged SCC to %s: %w\noutput: %s", sa, err, string(out))
		}
		m.logger.Info("granted privileged SCC", "serviceAccount", sa)
	}
	return nil
}

func (m *Manager) restartCAPIDeployments(ctx context.Context, infraProviders []string) error {
	for ns, info := range capiNamespaces {
		if err := m.scaleDeployment(ctx, ns, info.Deployment); err != nil {
			m.logger.Warn("failed to restart deployment", "namespace", ns, "error", err)
		}
	}
	for _, provider := range infraProviders {
		if infra, ok := capiInfraNamespaces[provider]; ok {
			if err := m.scaleDeployment(ctx, infra.Namespace, infra.Deployment); err != nil {
				m.logger.Warn("failed to restart deployment", "namespace", infra.Namespace, "error", err)
			}
		}
	}
	return nil
}

func (m *Manager) scaleDeployment(ctx context.Context, namespace, name string) error {
	_, err := m.runKubectl("rollout", "restart", "deployment/"+name, "-n", namespace)
	return err
}

func (m *Manager) waitForCAPIPods(ctx context.Context, opts CAPISetupOpts) error {
	deadline := time.Now().Add(opts.WaitTimeout)

	namespaces := make([]string, 0)
	for ns := range capiNamespaces {
		namespaces = append(namespaces, ns)
	}
	for _, provider := range opts.InfraProviders {
		if infra, ok := capiInfraNamespaces[provider]; ok {
			namespaces = append(namespaces, infra.Namespace)
		}
	}

	for _, ns := range namespaces {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out waiting for pods in %s", ns)
		}
		m.logger.Info("waiting for pods", "namespace", ns, "timeout", remaining.Round(time.Second))
		_, err := m.runKubectl("rollout", "status", "deployment", "-n", ns,
			"--timeout", fmt.Sprintf("%ds", int(remaining.Seconds())),
		)
		if err != nil {
			return fmt.Errorf("pods not ready in %s: %w", ns, err)
		}
	}
	return nil
}

func (m *Manager) CAPIControllerStatus(ctx context.Context) ([]CAPIControllerStatus, error) {
	var statuses []CAPIControllerStatus

	allNamespaces := make(map[string]string)
	for ns, info := range capiNamespaces {
		allNamespaces[ns] = info.Deployment
	}
	for _, infra := range capiInfraNamespaces {
		allNamespaces[infra.Namespace] = infra.Deployment
	}

	for ns, dep := range allNamespaces {
		status := CAPIControllerStatus{Namespace: ns, Deployment: dep}
		obj, err := m.client.Get(ctx, client.GVRDeployment, ns, dep)
		if err != nil {
			if apierrors.IsNotFound(err) {
				statuses = append(statuses, status)
				continue
			}
			return nil, err
		}

		readyReplicas, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
		status.Ready = readyReplicas > 0

		containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
		if len(containers) > 0 {
			if c, ok := containers[0].(map[string]interface{}); ok {
				status.Image, _ = c["image"].(string)
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (m *Manager) EnsureAWSSSHKeyPair(ctx context.Context, region, keyName string) (bool, error) {
	if region == "" {
		region = m.cfg.AWSRegion
	}
	if keyName == "" {
		return false, fmt.Errorf("SSH key name is required")
	}

	out, err := m.runAWS("ec2", "describe-key-pairs",
		"--key-names", keyName,
		"--region", region,
		"--output", "json",
	)
	if err == nil {
		var result struct {
			KeyPairs []struct {
				KeyName string `json:"KeyName"`
			} `json:"KeyPairs"`
		}
		if json.Unmarshal(out, &result) == nil && len(result.KeyPairs) > 0 {
			m.logger.Info("SSH key pair already exists", "name", keyName, "region", region)
			return false, nil
		}
	}

	m.logger.Info("creating SSH key pair", "name", keyName, "region", region)
	out, err = m.runAWS("ec2", "create-key-pair",
		"--key-name", keyName,
		"--region", region,
		"--query", "KeyPairId",
		"--output", "text",
	)
	if err != nil {
		return false, fmt.Errorf("creating SSH key pair %s: %w\noutput: %s", keyName, err, sanitizeOutput(out))
	}
	m.logger.Info("SSH key pair created", "name", keyName, "region", region, "id", strings.TrimSpace(string(out)))
	return true, nil
}

func (m *Manager) DeleteAWSSSHKeyPair(ctx context.Context, region, keyName string) error {
	if region == "" {
		region = m.cfg.AWSRegion
	}
	out, err := m.runAWS("ec2", "delete-key-pair",
		"--key-name", keyName,
		"--region", region,
	)
	if err != nil {
		return fmt.Errorf("deleting SSH key pair %s: %w\noutput: %s", keyName, err, sanitizeOutput(out))
	}
	m.logger.Info("SSH key pair deleted", "name", keyName, "region", region)
	return nil
}

func (m *Manager) runClusterctl(args ...string) ([]byte, error) {
	if m.clusterctlExec != nil {
		return m.clusterctlExec(args...)
	}
	return exec.Command("clusterctl", args...).CombinedOutput()
}

func (m *Manager) runOC(args ...string) ([]byte, error) {
	if m.ocExec != nil {
		return m.ocExec(args...)
	}
	return exec.Command("oc", args...).CombinedOutput()
}

func (m *Manager) runKubectl(args ...string) ([]byte, error) {
	if m.kubectlExec != nil {
		return m.kubectlExec(args...)
	}
	return exec.Command("kubectl", args...).CombinedOutput()
}

func sanitizeOutput(out []byte) string {
	s := string(out)
	for _, keyword := range []string{"key", "secret", "password", "token", "credential"} {
		if strings.Contains(strings.ToLower(s), keyword) {
			return "[output redacted - may contain sensitive data]"
		}
	}
	return s
}

func (r *CAPISetupResult) Summary() string {
	var lines []string
	lines = append(lines, "CAPI Setup Result:")
	lines = append(lines, fmt.Sprintf("  Controllers installed: %v", r.ControllersInstalled))
	lines = append(lines, fmt.Sprintf("  CRDs upgraded to v1beta2: %v", r.CRDsUpgraded))
	lines = append(lines, fmt.Sprintf("  Webhooks upgraded to v1beta2: %v", r.WebhooksUpgraded))
	lines = append(lines, fmt.Sprintf("  SCCs granted: %v", r.SCCsGranted))
	lines = append(lines, fmt.Sprintf("  Pods ready: %v", r.PodsReady))
	if r.SSHKeyName != "" {
		lines = append(lines, fmt.Sprintf("  SSH key: %s (created: %v)", r.SSHKeyName, r.SSHKeyCreated))
	}
	if len(r.Errors) > 0 {
		lines = append(lines, "  Errors:")
		for _, e := range r.Errors {
			lines = append(lines, "    - "+e)
		}
	}
	return strings.Join(lines, "\n")
}

func FormatCAPIControllerStatus(statuses []CAPIControllerStatus) string {
	var lines []string
	lines = append(lines, "CAPI Controller Status:")
	for _, s := range statuses {
		readyStr := "not ready"
		if s.Ready {
			readyStr = "ready"
		}
		lines = append(lines, fmt.Sprintf("  %s/%s: %s", s.Namespace, s.Deployment, readyStr))
		if s.Image != "" {
			lines = append(lines, fmt.Sprintf("    image: %s", s.Image))
		}
	}
	return strings.Join(lines, "\n")
}
