package provisioning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAWSCredentialsFromEnv(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-access-key")

	creds, err := LoadAWSCredentials("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "test-access-key-id" {
		t.Errorf("AccessKeyID = %q, want %q", creds.AccessKeyID, "test-access-key-id")
	}
	if creds.SecretAccessKey != "test-secret-access-key" {
		t.Errorf("SecretAccessKey = %q, want %q", creds.SecretAccessKey, "test-secret-access-key")
	}
}

func TestLoadAWSCredentialsMissingSecret(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	_, err := LoadAWSCredentials("")
	if err == nil {
		t.Error("expected error when secret is missing, got nil")
	}
}

func TestLoadAWSCredentialsFromFile(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	dir := t.TempDir()
	content := `[default]
aws_access_key_id = test-file-key-id
aws_secret_access_key = test-file-secret-key

[other]
aws_access_key_id = other-key-id
aws_secret_access_key = other-secret-key
`
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	creds, err := parseAWSCredentialsFile(path, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "test-file-key-id" {
		t.Errorf("AccessKeyID = %q, want %q", creds.AccessKeyID, "test-file-key-id")
	}
	if creds.SecretAccessKey != "test-file-secret-key" {
		t.Errorf("SecretAccessKey = %q, want %q", creds.SecretAccessKey, "test-file-secret-key")
	}
}

func TestParseAWSCredentialsFileNamedProfile(t *testing.T) {
	dir := t.TempDir()
	content := `[default]
aws_access_key_id = default-key
aws_secret_access_key = default-secret

[staging]
aws_access_key_id = staging-key
aws_secret_access_key = staging-secret
`
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	creds, err := parseAWSCredentialsFile(path, "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "staging-key" {
		t.Errorf("AccessKeyID = %q, want %q", creds.AccessKeyID, "staging-key")
	}
}

func TestParseAWSCredentialsFileMissing(t *testing.T) {
	_, err := parseAWSCredentialsFile("/nonexistent/path", "")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestParseAWSCredentialsFileIncompleteProfile(t *testing.T) {
	dir := t.TempDir()
	content := `[default]
aws_access_key_id = only-key
`
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := parseAWSCredentialsFile(path, "")
	if err == nil {
		t.Error("expected error for incomplete profile, got nil")
	}
}
