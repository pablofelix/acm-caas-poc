//go:build integration

package client

import (
	"context"
	"os"
	"testing"
)

func TestNewFromDefaultConnectsToCluster(t *testing.T) {
	c, err := NewFromDefault()
	if err != nil {
		t.Fatalf("NewFromDefault failed: %v", err)
	}

	list, err := c.List(context.Background(), GVRManagedCluster, "", "")
	if err != nil {
		t.Fatalf("List ManagedClusters failed: %v", err)
	}
	if len(list.Items) == 0 {
		t.Error("expected at least one ManagedCluster, got 0")
	}
}

func TestNewFromContextConnectsToCluster(t *testing.T) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}

	c, err := NewFromContext(kubeconfig, "")
	if err != nil {
		t.Fatalf("NewFromContext failed: %v", err)
	}

	list, err := c.List(context.Background(), GVRManagedCluster, "", "")
	if err != nil {
		t.Fatalf("List ManagedClusters failed: %v", err)
	}
	if len(list.Items) == 0 {
		t.Error("expected at least one ManagedCluster, got 0")
	}
}

func TestNewFromContextWithExplicitContext(t *testing.T) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = home + "/.kube/config"
	}
	ctx := os.Getenv("ACM_HUB_CONTEXT")
	if ctx == "" {
		t.Skip("ACM_HUB_CONTEXT not set")
	}

	c, err := NewFromContext(kubeconfig, ctx)
	if err != nil {
		t.Fatalf("NewFromContext with context %q failed: %v", ctx, err)
	}

	list, err := c.List(context.Background(), GVRManagedCluster, "", "")
	if err != nil {
		t.Fatalf("List ManagedClusters failed: %v", err)
	}
	if len(list.Items) == 0 {
		t.Error("expected at least one ManagedCluster, got 0")
	}
}

func TestNewFromContextInvalidPathReturnsError(t *testing.T) {
	_, err := NewFromContext("/nonexistent/kubeconfig", "")
	if err == nil {
		t.Error("expected error for invalid kubeconfig path, got nil")
	}
}
