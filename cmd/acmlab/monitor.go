package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/monitoring"
)

func monitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor cluster resources via ManagedClusterInfo",
	}
	cmd.AddCommand(monitorListCmd(), monitorStatusCmd())
	return cmd
}

func monitorListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cluster resource summaries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mon := monitoring.New(c, cfg)
			results, err := mon.ListClusterResources(context.Background())
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Println("No cluster info found")
				return nil
			}
			fmt.Printf("%-25s %-8s %-8s %-10s %-15s %s\n", "NAME", "NODES", "READY", "CPU", "OCP VERSION", "CHANNEL")
			for _, cr := range results {
				fmt.Printf("%-25s %-8d %-8d %-10d %-15s %s\n",
					cr.Name, cr.TotalNodes, cr.ReadyNodes, cr.TotalCPU, cr.OCPVersion, cr.Channel)
			}
			return nil
		},
	}
}

func monitorStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show detailed resource info for a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mon := monitoring.New(c, cfg)
			cr, err := mon.GetClusterResources(context.Background(), args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Cluster:     %s\n", cr.Name)
			fmt.Printf("OCP Version: %s\n", cr.OCPVersion)
			fmt.Printf("Channel:     %s\n", cr.Channel)
			fmt.Printf("K8s Version: %s\n", cr.Version)
			fmt.Printf("Console:     %s\n", cr.ConsoleURL)
			fmt.Printf("Nodes:       %d total, %d ready\n", cr.TotalNodes, cr.ReadyNodes)
			fmt.Printf("Total CPU:   %d cores\n", cr.TotalCPU)
			fmt.Println()

			if len(cr.Nodes) > 0 {
				fmt.Printf("%-35s %-6s %-6s %-12s %-15s %-12s %s\n",
					"NODE", "READY", "CPU", "MEMORY", "INSTANCE", "REGION", "ZONE")
				for _, n := range cr.Nodes {
					fmt.Printf("%-35s %-6v %-6s %-12s %-15s %-12s %s\n",
						n.Name, n.Ready, n.CPUCapacity, n.MemoryKi, n.InstanceType, n.Region, n.Zone)
				}
			}
			return nil
		},
	}
}
