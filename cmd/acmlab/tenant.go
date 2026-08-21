package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/tenant"
)

func tenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Manage tenant isolation on spoke clusters via ManifestWork",
	}
	cmd.AddCommand(
		tenantDeployCmd(),
		tenantRemoveCmd(),
		tenantListCmd(),
		tenantStatusCmd(),
	)
	return cmd
}

func tenantDeployCmd() *cobra.Command {
	var cluster, team, cpu, memory string
	var pods int64
	cmd := &cobra.Command{
		Use:   "deploy <tenant-name>",
		Short: "Deploy tenant namespace, RBAC, network policy, and quota to a spoke (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := tenant.New(c, cfg)
			opts := tenant.TenantOpts{
				Name:        args[0],
				Cluster:     cluster,
				Team:        team,
				CPULimit:    cpu,
				MemoryLimit: memory,
				PodLimit:    pods,
			}
			fmt.Printf("Deploying tenant %s to cluster %s...\n", args[0], cluster)
			if err := mgr.Deploy(context.Background(), opts); err != nil {
				return err
			}
			fmt.Println("Tenant deployed. Use 'acmlab tenant status' to check sync status.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&cluster, "cluster", "c", "", "target spoke cluster (required)")
	cmd.Flags().StringVarP(&team, "team", "t", "", "team/group name for RBAC (default: tenant name)")
	cmd.Flags().StringVar(&cpu, "cpu", "4", "CPU request limit")
	cmd.Flags().StringVar(&memory, "memory", "8Gi", "memory request limit")
	cmd.Flags().Int64Var(&pods, "pods", 20, "pod count limit")
	_ = cmd.MarkFlagRequired("cluster")
	return cmd
}

func tenantRemoveCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "remove <tenant-name>",
		Short: "Remove tenant isolation from a spoke — clean, no leftovers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := tenant.New(c, cfg)
			fmt.Printf("Removing tenant %s from cluster %s...\n", args[0], cluster)
			if err := mgr.Remove(context.Background(), args[0], cluster); err != nil {
				return err
			}
			fmt.Println("Tenant removed. ManifestWork deleted, spoke resources will be cleaned up.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&cluster, "cluster", "c", "", "target spoke cluster (required)")
	_ = cmd.MarkFlagRequired("cluster")
	return cmd
}

func tenantListCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tenants deployed to a spoke cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := tenant.New(c, cfg)
			tenants, err := mgr.List(context.Background(), cluster)
			if err != nil {
				return err
			}
			if len(tenants) == 0 {
				fmt.Println("No tenants found")
				return nil
			}
			fmt.Printf("%-25s %-25s %s\n", "TENANT", "CLUSTER", "STATUS")
			for _, t := range tenants {
				fmt.Printf("%-25s %-25s %s\n", t.Name, t.Cluster, t.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&cluster, "cluster", "c", "", "spoke cluster to list tenants for (required)")
	_ = cmd.MarkFlagRequired("cluster")
	return cmd
}

func tenantStatusCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "status <tenant-name>",
		Short: "Show tenant ManifestWork sync status and per-resource results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := tenant.New(c, cfg)
			ms, err := mgr.Status(context.Background(), args[0], cluster)
			if err != nil {
				return err
			}
			fmt.Printf("ManifestWork: %s\n", ms.Name)
			fmt.Printf("Cluster:      %s\n", ms.Cluster)
			fmt.Printf("Applied:      %v\n", ms.Applied)
			if len(ms.Conditions) > 0 {
				fmt.Printf("Conditions:   %v\n", ms.Conditions)
			}
			fmt.Println()
			if len(ms.Resources) > 0 {
				fmt.Printf("%-20s %-30s %-20s %s\n", "KIND", "NAME", "NAMESPACE", "STATUS")
				for _, r := range ms.Resources {
					ns := r.Namespace
					if ns == "" {
						ns = "-"
					}
					fmt.Printf("%-20s %-30s %-20s %s\n", r.Kind, r.Name, ns, r.Status)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&cluster, "cluster", "c", "", "spoke cluster (required)")
	_ = cmd.MarkFlagRequired("cluster")
	return cmd
}
