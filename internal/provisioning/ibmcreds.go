package provisioning

import (
	"fmt"
)

type componentCredential struct {
	SecretName string
	Namespace  string
	APIKey     string
}

func generateIBMCloudCredentials(apiKey, clusterName string) ([]componentCredential, error) {
	iam := newIAMClient(apiKey)
	if err := iam.authenticate(); err != nil {
		return nil, fmt.Errorf("authenticating with IBM Cloud: %w", err)
	}

	var creds []componentCredential
	var createdServiceIDs []string

	for _, cr := range ibmCloudCredentialRequests {
		sidName := fmt.Sprintf("%s-%s", clusterName, cr.Namespace)

		sid, err := iam.createServiceID(sidName, fmt.Sprintf("Service ID for %s in cluster %s", cr.Namespace, clusterName))
		if err != nil {
			cleanupServiceIDs(iam, createdServiceIDs)
			return nil, err
		}
		createdServiceIDs = append(createdServiceIDs, sid.ID)

		for _, policy := range cr.Policies {
			if err := iam.createPolicy(sid.IAMID, policy); err != nil {
				cleanupServiceIDs(iam, createdServiceIDs)
				return nil, err
			}
		}

		key, err := iam.createAPIKey(sidName+"-key", sid.IAMID)
		if err != nil {
			cleanupServiceIDs(iam, createdServiceIDs)
			return nil, err
		}

		creds = append(creds, componentCredential{
			SecretName: cr.SecretName,
			Namespace:  cr.Namespace,
			APIKey:     key,
		})
	}

	return creds, nil
}

func cleanupIBMCloudCredentials(apiKey, clusterName string) error {
	iam := newIAMClient(apiKey)
	if err := iam.authenticate(); err != nil {
		return fmt.Errorf("authenticating with IBM Cloud: %w", err)
	}

	for _, cr := range ibmCloudCredentialRequests {
		sidName := fmt.Sprintf("%s-%s", clusterName, cr.Namespace)
		sids, err := iam.listServiceIDs(sidName)
		if err != nil {
			continue
		}
		for _, sid := range sids {
			iam.deleteServiceID(sid.ID)
		}
	}
	return nil
}

func cleanupServiceIDs(iam *iamClient, ids []string) {
	for _, id := range ids {
		iam.deleteServiceID(id)
	}
}

func buildManifestYAMLs(creds []componentCredential) map[string]string {
	manifests := make(map[string]string)
	for _, cred := range creds {
		yaml := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
stringData:
  ibm-credentials.env: |-
    IBMCLOUD_AUTHTYPE=iam
    IBMCLOUD_APIKEY=%s
  ibmcloud_api_key: %s
type: Opaque
`, cred.SecretName, cred.Namespace, cred.APIKey, cred.APIKey)
		filename := fmt.Sprintf("%s-%s-credentials.yaml", cred.Namespace, cred.SecretName)
		manifests[filename] = yaml
	}
	return manifests
}
