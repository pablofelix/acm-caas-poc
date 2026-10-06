package provisioning

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

func LoadAWSCredentials(profile string) (*AWSCredentials, error) {
	if id := os.Getenv("AWS_ACCESS_KEY_ID"); id != "" {
		secret := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if secret == "" {
			return nil, fmt.Errorf("AWS_ACCESS_KEY_ID is set but AWS_SECRET_ACCESS_KEY is not")
		}
		return &AWSCredentials{AccessKeyID: id, SecretAccessKey: secret}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	path := filepath.Join(home, ".aws", "credentials")
	return parseAWSCredentialsFile(path, profile)
}

func parseAWSCredentialsFile(path, profile string) (*AWSCredentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	if profile == "" {
		profile = "default"
	}
	target := "[" + profile + "]"

	var creds AWSCredentials
	inProfile := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inProfile = line == target
			continue
		}
		if !inProfile {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "aws_access_key_id":
			creds.AccessKeyID = val
		case "aws_secret_access_key":
			creds.SecretAccessKey = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return nil, fmt.Errorf("profile %q not found or incomplete in %s", profile, path)
	}
	return &creds, nil
}
