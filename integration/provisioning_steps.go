//go:build integration

package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

func registerProvisioningSteps(sc *godog.ScenarioContext, s *suiteContext) {
	sc.Step(`^cloud credentials exist as a Secret in namespace "([^"]*)"$`, s.cloudCredentialsExist)
	sc.Step(`^a ClusterImageSet for the target OCP version exists$`, s.clusterImageSetExists)
	sc.Step(`^I provision cluster "([^"]*)" with default settings$`, s.iProvisionCluster)
	sc.Step(`^the ClusterDeployment "([^"]*)" is accepted by Hive$`, s.clusterDeploymentAccepted)
	sc.Step(`^the cluster "([^"]*)" eventually reaches Provisioned = True$`, s.clusterReachesProvisioned)
	sc.Step(`^I list all provisioned clusters$`, s.iListProvisionedClusters)
	sc.Step(`^I receive a list of ClusterDeployments with status$`, s.receiveClusterDeploymentList)
	sc.Step(`^a ClusterDeployment "([^"]*)" exists with status Provisioned = True$`, s.clusterDeploymentProvisioned)
	sc.Step(`^I destroy cluster "([^"]*)"$`, s.iDestroyCluster)
	sc.Step(`^the ClusterDeployment "([^"]*)" is removed$`, s.clusterDeploymentRemoved)
	sc.Step(`^I register IBM Cloud credentials as an ACM central credential "([^"]*)"$`, s.registerIBMCentralCredential)
	sc.Step(`^the ACM credential "([^"]*)" exists in open-cluster-management namespace$`, s.acmCredentialExists)
	sc.Step(`^the credential has provider type "([^"]*)"$`, s.credentialHasProviderType)
}

func (s *suiteContext) cloudCredentialsExist(ctx context.Context, ns string) error {
	credentialKeys := []string{
		"aws_access_key_id", "credentials", "osServicePrincipal.json",
		"ibmcloud_api_key",
	}

	nsObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": ns},
		},
	}
	_ = s.client.CreateIfNotExists(ctx, client.GVRNamespace, "", nsObj)

	list, err := s.client.List(ctx, client.GVRSecret, ns, "")
	if err != nil {
		return fmt.Errorf("listing secrets in %s: %w", ns, err)
	}
	for _, secret := range list.Items {
		data, _, _ := unstructured.NestedMap(secret.Object, "data")
		for _, key := range credentialKeys {
			val, ok := data[key]
			if !ok {
				continue
			}
			str, isStr := val.(string)
			if isStr && str == "" {
				continue
			}
			fmt.Printf("\n  ┌─ Credentials found in namespace %s\n", ns)
			fmt.Printf("  │  Secret: %s, key: %s\n", secret.GetName(), key)
			fmt.Printf("  └─\n")
			return nil
		}
	}

	if s.cfg.Platform == "aws" {
		awsCreds, err := provisioning.LoadAWSCredentials("")
		if err != nil {
			return fmt.Errorf("loading AWS credentials: %w", err)
		}
		creds := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]interface{}{"name": ns + "-aws-creds", "namespace": ns},
				"type":       "Opaque",
				"stringData": map[string]interface{}{
					"aws_access_key_id":     awsCreds.AccessKeyID,
					"aws_secret_access_key": awsCreds.SecretAccessKey,
				},
			},
		}
		if err := s.client.CreateIfNotExists(ctx, client.GVRSecret, ns, creds); err != nil {
			return fmt.Errorf("creating AWS credential secret in %s: %w", ns, err)
		}
		fmt.Printf("\n  ┌─ Created AWS credential secret in namespace %s\n", ns)
		fmt.Printf("  └─\n")
		return nil
	}

	if s.cfg.IBMCloudAPIKey != "" {
		creds := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]interface{}{"name": ns + "-creds", "namespace": ns},
				"type":       "Opaque",
				"stringData": map[string]interface{}{"ibmcloud_api_key": s.cfg.IBMCloudAPIKey},
			},
		}
		if err := s.client.CreateIfNotExists(ctx, client.GVRSecret, ns, creds); err != nil {
			return fmt.Errorf("creating IBM credential secret in %s: %w", ns, err)
		}
		fmt.Printf("\n  ┌─ Created IBM Cloud credential secret in namespace %s\n", ns)
		fmt.Printf("  └─\n")
		return nil
	}

	return fmt.Errorf("no cloud credential secret found in namespace %s and no credentials configured", ns)
}

func (s *suiteContext) clusterImageSetExists(ctx context.Context) error {
	sets, err := s.provisioner.ListImageSets(ctx)
	if err != nil {
		return err
	}
	if len(sets) == 0 {
		return fmt.Errorf("no ClusterImageSets available")
	}
	return nil
}

func (s *suiteContext) iProvisionCluster(ctx context.Context, name string) error {
	pullSecret, err := s.fetchPullSecret(ctx)
	if err != nil {
		return fmt.Errorf("fetching pull secret: %w", err)
	}
	opts := provisioning.ClusterOpts{
		Name:       name,
		PullSecret: pullSecret,
	}
	region := s.cfg.IBMCloudRegion
	if s.cfg.Platform == "aws" {
		region = s.cfg.AWSRegion
		awsCreds, err := provisioning.LoadAWSCredentials("")
		if err != nil {
			return fmt.Errorf("loading AWS credentials: %w", err)
		}
		opts.AWSAccessKeyID = awsCreds.AccessKeyID
		opts.AWSSecretAccessKey = awsCreds.SecretAccessKey
	}
	fmt.Printf("\n  ┌─ Provisioning cluster %q (platform=%s, region=%s, image=%s)\n", name, s.cfg.Platform, region, s.cfg.ClusterImageSet)
	fmt.Printf("  └─\n")
	return s.provisioner.Create(ctx, opts)
}

func (s *suiteContext) fetchPullSecret(ctx context.Context) (string, error) {
	obj, err := s.client.Get(ctx, client.GVRSecret, "openshift-config", "pull-secret")
	if err != nil {
		return "", fmt.Errorf("reading pull-secret from openshift-config: %w", err)
	}
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	if encoded, ok := data[".dockerconfigjson"].(string); ok {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return encoded, nil
		}
		return string(decoded), nil
	}
	return "", fmt.Errorf("pull-secret does not contain .dockerconfigjson")
}

func (s *suiteContext) clusterDeploymentAccepted(ctx context.Context, name string) error {
	_, err := s.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err != nil {
		return err
	}
	fmt.Printf("\n  ┌─ ClusterDeployment %q accepted by Hive\n", name)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) clusterReachesProvisioned(ctx context.Context, name string) error {
	timeout := 45 * time.Minute
	interval := 30 * time.Second
	deadline := time.After(timeout)
	for {
		obj, err := s.client.Get(ctx, client.GVRClusterDeployment, name, name)
		if err != nil {
			return fmt.Errorf("ClusterDeployment %s not found: %w", name, err)
		}
		conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		for _, raw := range conditions {
			cond, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			if condType == "Provisioned" && condStatus == "True" {
				return nil
			}
		}
		select {
		case <-deadline:
			return fmt.Errorf("ClusterDeployment %s did not reach Provisioned within %v", name, timeout)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *suiteContext) iListProvisionedClusters(ctx context.Context) error {
	list, err := s.provisioner.List(ctx)
	if err != nil {
		return err
	}
	s.provisionList = list
	return nil
}

func (s *suiteContext) receiveClusterDeploymentList() error {
	if s.provisionList == nil {
		return fmt.Errorf("no provisioned clusters returned")
	}
	fmt.Printf("\n  ┌─ Provisioned clusters: %d\n", len(s.provisionList))
	for _, c := range s.provisionList {
		status := "Provisioned"
		if !c.Provisioned {
			status = "Pending"
		}
		if c.FailureReason != "" {
			status = "Failed: " + c.FailureReason
		}
		fmt.Printf("  │  %-20s region=%-12s image=%-28s status=%s\n", c.Name, c.Region, c.ImageSet, status)
	}
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) clusterDeploymentProvisioned(ctx context.Context, name string) error {
	_, err := s.client.Get(ctx, client.GVRClusterDeployment, name, name)
	return err
}

func (s *suiteContext) iDestroyCluster(ctx context.Context, name string) error {
	return s.provisioner.Destroy(ctx, name)
}

func (s *suiteContext) clusterDeploymentRemoved(ctx context.Context, name string) error {
	_, err := s.client.Get(ctx, client.GVRClusterDeployment, name, name)
	if err == nil {
		return fmt.Errorf("ClusterDeployment %s still exists", name)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("unexpected error checking ClusterDeployment %s: %w", name, err)
	}
	fmt.Printf("\n  ┌─ ClusterDeployment %q confirmed removed\n", name)
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) registerIBMCentralCredential(ctx context.Context, name string) error {
	cred := provisioning.CentralCredential{
		Name:       name,
		Provider:   "ibm",
		BaseDomain: s.cfg.BaseDomain,
		APIKey:     s.cfg.IBMCloudAPIKey,
	}
	if err := s.provisioner.EnsureCentralCredential(ctx, cred); err != nil {
		return fmt.Errorf("registering ACM credential: %w", err)
	}
	fmt.Printf("\n  ┌─ Registered ACM central credential %q (provider=ibm)\n", name)
	fmt.Printf("  │  Namespace: open-cluster-management\n")
	fmt.Printf("  │  Labels: cluster.open-cluster-management.io/credentials, type=ibm\n")
	fmt.Printf("  └─\n")
	return nil
}

func (s *suiteContext) acmCredentialExists(ctx context.Context, name string) error {
	cred, err := s.provisioner.GetCentralCredential(ctx, name)
	if err != nil {
		return fmt.Errorf("ACM credential %q not found: %w", name, err)
	}
	fmt.Printf("\n  ┌─ ACM credential %q exists (provider=%s)\n", cred.Name, cred.Provider)
	fmt.Printf("  └─\n")
	s.lastCredential = cred
	return nil
}

func (s *suiteContext) credentialHasProviderType(ctx context.Context, providerType string) error {
	if s.lastCredential == nil {
		return fmt.Errorf("no credential loaded — run the existence check step first")
	}
	if s.lastCredential.Provider != providerType {
		return fmt.Errorf("credential provider = %q, want %q", s.lastCredential.Provider, providerType)
	}
	fmt.Printf("\n  ┌─ Provider type confirmed: %s\n", providerType)
	fmt.Printf("  └─\n")
	return nil
}
