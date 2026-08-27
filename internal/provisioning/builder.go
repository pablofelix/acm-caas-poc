package provisioning

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func buildNamespace(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

func buildCredentialsSecret(namespace, apiKey string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-ibmcloud-creds",
				"namespace": namespace,
			},
			"type": "Opaque",
			"data": map[string]interface{}{
				"ibmcloud_api_key": base64.StdEncoding.EncodeToString([]byte(apiKey)),
			},
		},
	}
}

func buildPullSecret(namespace, pullSecret string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-pull-secret",
				"namespace": namespace,
			},
			"type": "kubernetes.io/dockerconfigjson",
			"data": map[string]interface{}{
				".dockerconfigjson": base64.StdEncoding.EncodeToString([]byte(pullSecret)),
			},
		},
	}
}

func buildInstallConfigSecret(namespace string, opts ClusterOpts) *unstructured.Unstructured {
	installConfig := generateInstallConfig(opts)
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-install-config",
				"namespace": namespace,
			},
			"type": "Opaque",
			"data": map[string]interface{}{
				"install-config.yaml": base64.StdEncoding.EncodeToString([]byte(installConfig)),
			},
		},
	}
}

func generateInstallConfig(opts ClusterOpts) string {
	platformBlock := fmt.Sprintf("  %s:\n    region: %s", opts.Platform, opts.Region)

	controlPlanePlatform := fmt.Sprintf("    %s:\n      type: %s", opts.Platform, opts.MasterType)
	computePlatform := fmt.Sprintf("    %s:\n      type: %s", opts.Platform, opts.WorkerType)

	credentialsMode := ""
	if opts.Platform == "ibmcloud" {
		credentialsMode = "credentialsMode: Manual\n"
	}

	return fmt.Sprintf(`apiVersion: v1
metadata:
  name: %s
baseDomain: %s
platform:
%s
controlPlane:
  name: master
  replicas: %d
  platform:
%s
compute:
- name: worker
  replicas: %d
  platform:
%s
networking:
  networkType: OVNKubernetes
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  serviceNetwork:
  - 172.30.0.0/16
%spullSecret: ""
sshKey: %s
`, opts.Name, opts.BaseDomain, platformBlock, opts.MasterReplicas, controlPlanePlatform,
		opts.WorkerReplicas, computePlatform, credentialsMode, opts.SSHKey)
}

func buildClusterDeployment(opts ClusterOpts) *unstructured.Unstructured {
	provisioning := map[string]interface{}{
		"imageSetRef": map[string]interface{}{
			"name": opts.ImageSet,
		},
		"installConfigSecretRef": map[string]interface{}{
			"name": opts.Name + "-install-config",
		},
	}
	if opts.Platform == "ibmcloud" {
		provisioning["manifestsSecretRef"] = map[string]interface{}{
			"name": opts.Name + "-manifests",
		}
	}
	if opts.SSHPrivateKey != "" {
		provisioning["sshPrivateKeySecretRef"] = map[string]interface{}{
			"name": opts.Name + "-ssh-private-key",
		}
	}

	platformSpec := map[string]interface{}{
		"region": opts.Region,
	}
	if opts.Platform == "ibmcloud" {
		platformSpec["credentialsSecretRef"] = map[string]interface{}{
			"name": opts.Name + "-ibmcloud-creds",
		}
	}

	cd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hive.openshift.io/v1",
			"kind":       "ClusterDeployment",
			"metadata": map[string]interface{}{
				"name":      opts.Name,
				"namespace": opts.Name,
				"labels": map[string]interface{}{
					"acmlab.redhat.com/managed": "true",
				},
			},
			"spec": map[string]interface{}{
				"clusterName": opts.Name,
				"baseDomain":  opts.BaseDomain,
				"platform": map[string]interface{}{
					opts.Platform: platformSpec,
				},
				"provisioning": provisioning,
				"pullSecretRef": map[string]interface{}{
					"name": opts.Name + "-pull-secret",
				},
			},
		},
	}
	return cd
}

func buildManifestsSecret(namespace, manifestsDir string) (*unstructured.Unstructured, error) {
	data := map[string]interface{}{}
	entries, err := os.ReadDir(manifestsDir)
	if err != nil {
		return nil, fmt.Errorf("reading manifests dir %s: %w", manifestsDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(manifestsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading manifest %s: %w", entry.Name(), err)
		}
		data[entry.Name()] = base64.StdEncoding.EncodeToString(content)
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-manifests",
				"namespace": namespace,
			},
			"type": "Opaque",
			"data": data,
		},
	}, nil
}

func buildManifestsSecretFromYAMLs(namespace string, yamls map[string]string) *unstructured.Unstructured {
	data := map[string]interface{}{}
	for filename, content := range yamls {
		data[filename] = base64.StdEncoding.EncodeToString([]byte(content))
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-manifests",
				"namespace": namespace,
			},
			"type": "Opaque",
			"data": data,
		},
	}
}

func buildManagedCluster(name, platform string) *unstructured.Unstructured {
	cloudLabel := "Other"
	switch platform {
	case "ibmcloud":
		cloudLabel = "IBM"
	case "aws":
		cloudLabel = "Amazon"
	case "gcp":
		cloudLabel = "Google"
	case "azure":
		cloudLabel = "Azure"
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"cloud":                                            cloudLabel,
					"vendor":                                           "OpenShift",
					"cluster.open-cluster-management.io/clusterset":    "default",
					"acmlab.redhat.com/managed":                        "true",
				},
			},
			"spec": map[string]interface{}{
				"hubAcceptsClient": true,
			},
		},
	}
}

func buildKlusterletAddonConfig(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": name,
			},
			"spec": map[string]interface{}{
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": true,
				},
				"policyController": map[string]interface{}{
					"enabled": true,
				},
				"searchCollector": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}
}

func buildSSHPrivateKeySecret(namespace, sshPrivateKey string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      namespace + "-ssh-private-key",
				"namespace": namespace,
			},
			"type": "Opaque",
			"data": map[string]interface{}{
				"ssh-privatekey": base64.StdEncoding.EncodeToString([]byte(sshPrivateKey)),
			},
		},
	}
}
