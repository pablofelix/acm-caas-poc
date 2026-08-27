package provisioning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const iamTokenURL = "https://iam.cloud.ibm.com/identity/token"
const iamBaseURL = "https://iam.cloud.ibm.com"

type iamClient struct {
	apiKey    string
	accountID string
	token     string
	http      *http.Client
}

func newIAMClient(apiKey string) *iamClient {
	return &iamClient{
		apiKey: apiKey,
		http:   &http.Client{},
	}
}

func (c *iamClient) authenticate() error {
	data := url.Values{
		"grant_type": {"urn:ibm:params:oauth:grant-type:apikey"},
		"apikey":     {c.apiKey},
	}
	req, err := http.NewRequest("POST", iamTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting IAM token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("IAM token request failed (%d): %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding token response: %w", err)
	}
	c.token = tokenResp.AccessToken

	return c.fetchAccountID()
}

func (c *iamClient) fetchAccountID() error {
	req, err := http.NewRequest("GET", iamBaseURL+"/v1/apikeys/details", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("IAM-Apikey", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetching API key details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API key details request failed (%d): %s", resp.StatusCode, body)
	}

	var details struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return fmt.Errorf("decoding API key details: %w", err)
	}
	c.accountID = details.AccountID
	return nil
}

type serviceIDResponse struct {
	ID    string `json:"id"`
	IAMID string `json:"iam_id"`
}

func (c *iamClient) createServiceID(name, description string) (*serviceIDResponse, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": description,
		"account_id":  c.accountID,
	}
	resp, err := c.doJSON("POST", iamBaseURL+"/v1/serviceids", body)
	if err != nil {
		return nil, fmt.Errorf("creating service ID %s: %w", name, err)
	}
	var result serviceIDResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("decoding service ID response: %w", err)
	}
	return &result, nil
}

func (c *iamClient) createPolicy(serviceIDIAMID string, spec iamPolicySpec) error {
	subject := map[string]interface{}{
		"attributes": []map[string]interface{}{
			{"name": "iam_id", "value": serviceIDIAMID},
		},
	}

	resource := map[string]interface{}{
		"attributes": []map[string]interface{}{
			{"name": "accountId", "value": c.accountID},
		},
	}

	if spec.ServiceName != "" {
		attrs := resource["attributes"].([]map[string]interface{})
		attrs = append(attrs, map[string]interface{}{"name": "serviceName", "value": spec.ServiceName})
		resource["attributes"] = attrs
	}
	if spec.ResourceType != "" {
		attrs := resource["attributes"].([]map[string]interface{})
		attrs = append(attrs, map[string]interface{}{"name": "resourceType", "value": spec.ResourceType})
		resource["attributes"] = attrs
	}

	roles := make([]map[string]interface{}, len(spec.Roles))
	for i, r := range spec.Roles {
		roles[i] = map[string]interface{}{"role_id": r}
	}

	policy := map[string]interface{}{
		"type":      "access",
		"subjects":  []interface{}{subject},
		"resources": []interface{}{resource},
		"roles":     roles,
	}

	_, err := c.doJSON("POST", iamBaseURL+"/v1/policies", policy)
	if err != nil {
		svcOrRes := spec.ServiceName
		if svcOrRes == "" {
			svcOrRes = spec.ResourceType
		}
		return fmt.Errorf("creating policy for %s: %w", svcOrRes, err)
	}
	return nil
}

type apiKeyResponse struct {
	APIKey string `json:"apikey"`
}

func (c *iamClient) createAPIKey(name, iamID string) (string, error) {
	body := map[string]interface{}{
		"name":       name,
		"iam_id":     iamID,
		"account_id": c.accountID,
	}
	resp, err := c.doJSON("POST", iamBaseURL+"/v1/apikeys", body)
	if err != nil {
		return "", fmt.Errorf("creating API key for %s: %w", name, err)
	}
	var result apiKeyResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("decoding API key response: %w", err)
	}
	return result.APIKey, nil
}

func (c *iamClient) deleteServiceID(id string) error {
	req, err := http.NewRequest("DELETE", iamBaseURL+"/v1/serviceids/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("deleting service ID %s: status %d", id, resp.StatusCode)
	}
	return nil
}

func (c *iamClient) listServiceIDs(namePrefix string) ([]serviceIDResponse, error) {
	u := fmt.Sprintf("%s/v1/serviceids?account_id=%s&name=%s&pagesize=100",
		iamBaseURL, c.accountID, url.QueryEscape(namePrefix))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("listing service IDs (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		ServiceIDs []serviceIDResponse `json:"serviceids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.ServiceIDs, nil
}

func (c *iamClient) doJSON(method, url string, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("IBM Cloud IAM API error (%d): %s", resp.StatusCode, respBody)
	}
	return respBody, nil
}
