package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/virtualization"
)

func vmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vm",
		Short: "Manage virtual machines across managed clusters (OpenShift Virtualization)",
	}
	cmd.AddCommand(
		vmDeployCmd(),
		vmRemoveCmd(),
		vmStartCmd(),
		vmStopCmd(),
		vmMigrateCmd(),
		vmStatusCmd(),
		vmListCmd(),
		vmEnsureCNVCmd(),
		vmCNVStatusCmd(),
		vmRemoveCNVCmd(),
	)
	return cmd
}

func vmDeployCmd() *cobra.Command {
	var cluster, name, cpu, memory, image, diskSize string

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a virtual machine to a managed cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cluster == "" || name == "" {
				return fmt.Errorf("--cluster and --name are required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			opts := virtualization.VMOpts{
				Name:     name,
				Cluster:  cluster,
				CPU:      cpu,
				Memory:   memory,
				Image:    image,
				DiskSize: diskSize,
			}
			fmt.Printf("Deploying VM %s to %s...\n", name, cluster)
			if err := mgr.Deploy(context.Background(), opts); err != nil {
				return err
			}
			fmt.Println("VM deployed via ManifestWork.")
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster name (required)")
	cmd.Flags().StringVar(&name, "name", "", "VM name (required)")
	cmd.Flags().StringVar(&cpu, "cpu", "", "number of CPU cores (default: 2)")
	cmd.Flags().StringVar(&memory, "memory", "", "memory size (default: 4Gi)")
	cmd.Flags().StringVar(&image, "image", "", "container disk image")
	cmd.Flags().StringVar(&diskSize, "disk-size", "", "disk size (default: 20Gi)")
	return cmd
}

func vmRemoveCmd() *cobra.Command {
	var cluster string

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a virtual machine from a managed cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cluster == "" {
				return fmt.Errorf("--cluster is required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			fmt.Printf("Removing VM %s from %s...\n", args[0], cluster)
			if err := mgr.Remove(context.Background(), args[0], cluster); err != nil {
				return err
			}
			fmt.Println("VM removed.")
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster name (required)")
	return cmd
}

func vmStartCmd() *cobra.Command {
	var cluster string

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cluster == "" {
				return fmt.Errorf("--cluster is required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			if err := mgr.Start(context.Background(), args[0], cluster); err != nil {
				return err
			}
			fmt.Printf("VM %s started on %s\n", args[0], cluster)
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster name (required)")
	return cmd
}

func vmStopCmd() *cobra.Command {
	var cluster string

	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a virtual machine",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cluster == "" {
				return fmt.Errorf("--cluster is required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			if err := mgr.Stop(context.Background(), args[0], cluster); err != nil {
				return err
			}
			fmt.Printf("VM %s stopped on %s\n", args[0], cluster)
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster name (required)")
	return cmd
}

func vmMigrateCmd() *cobra.Command {
	var cluster string

	cmd := &cobra.Command{
		Use:   "migrate <name>",
		Short: "Live-migrate a virtual machine to another node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cluster == "" {
				return fmt.Errorf("--cluster is required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			fmt.Printf("Migrating VM %s on %s...\n", args[0], cluster)
			if err := mgr.Migrate(context.Background(), args[0], cluster); err != nil {
				return err
			}
			fmt.Println("Migration initiated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster name (required)")
	return cmd
}

func vmStatusCmd() *cobra.Command {
	var cluster string
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show virtual machine status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cluster == "" {
				return fmt.Errorf("--cluster is required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			detail, err := mgr.Status(context.Background(), args[0], cluster)
			if err != nil {
				return err
			}
			if outputJSON {
				data, _ := json.MarshalIndent(detail, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Printf("Name:     %s\n", detail.Name)
			fmt.Printf("Cluster:  %s\n", detail.Cluster)
			fmt.Printf("Status:   %s\n", detail.Status)
			fmt.Printf("CPU:      %s\n", detail.CPU)
			fmt.Printf("Memory:   %s\n", detail.Memory)
			fmt.Printf("Image:    %s\n", detail.Image)
			fmt.Printf("Disk:     %s\n", detail.DiskSize)
			if detail.Node != "" {
				fmt.Printf("Node:     %s\n", detail.Node)
			}
			if detail.IP != "" {
				fmt.Printf("IP:       %s\n", detail.IP)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "target cluster name (required)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func vmEnsureCNVCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ensure-cnv <cluster>",
		Short: "Install OpenShift Virtualization on a managed cluster via governance policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			fmt.Printf("Ensuring OpenShift Virtualization on %s...\n", args[0])
			if err := mgr.EnsureCNVOperator(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Println("CNV operator policy applied. Use 'acmlab vm cnv-status' to check progress.")
			return nil
		},
	}
	return cmd
}

func vmCNVStatusCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "cnv-status <cluster>",
		Short: "Check OpenShift Virtualization installation status on a managed cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			status, err := mgr.CNVStatus(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				data, _ := json.MarshalIndent(status, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Printf("Cluster:   %s\n", status.Cluster)
			fmt.Printf("Status:    %s\n", status.Compliant)
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func vmRemoveCNVCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-cnv <cluster>",
		Short: "Remove OpenShift Virtualization governance policy from a managed cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			fmt.Printf("Removing CNV policy for %s...\n", args[0])
			if err := mgr.RemoveCNVOperator(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Println("CNV operator policy removed.")
			return nil
		},
	}
	return cmd
}

func vmListCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all virtual machines across the fleet",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := virtualization.New(c, cfg, logger)
			vms, err := mgr.List(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				data, _ := json.MarshalIndent(vms, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			if len(vms) == 0 {
				fmt.Println("No virtual machines found")
				return nil
			}
			fmt.Printf("%-20s %-20s %-15s %s\n", "NAME", "CLUSTER", "STATUS", "IP")
			for _, vm := range vms {
				fmt.Printf("%-20s %-20s %-15s %s\n", vm.Name, vm.Cluster, vm.Status, vm.IP)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}
