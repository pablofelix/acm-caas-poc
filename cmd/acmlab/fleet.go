package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/fleet"
)

func fleetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Query managed cluster fleet",
	}
	cmd.AddCommand(fleetListCmd(), fleetStatusCmd())
	return cmd
}

func fleetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all managed clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			insp := fleet.New(c, cfg)
			clusters, err := insp.ListClusters(context.Background())
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				fmt.Println("No managed clusters found")
				return nil
			}
			fmt.Printf("%-30s %-10s %-10s %-10s %s\n", "NAME", "AVAILABLE", "JOINED", "ACCEPTED", "VERSION")
			for _, cl := range clusters {
				fmt.Printf("%-30s %-10v %-10v %-10v %s\n",
					cl.Name, cl.Available, cl.Joined, cl.Accepted, cl.Version)
			}
			return nil
		},
	}
}

func fleetStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <name>",
		Short: "Show detailed status for a managed cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			insp := fleet.New(c, cfg)
			info, err := insp.GetCluster(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Cluster: %s\n", info.Name)
			fmt.Printf("Version: %s\n", info.Version)
			fmt.Printf("Available: %v\n", info.Available)
			fmt.Printf("Joined: %v\n", info.Joined)
			fmt.Printf("Accepted: %v\n", info.Accepted)
			fmt.Printf("Labels:\n")
			for k, v := range info.Labels {
				fmt.Printf("  %s=%s\n", k, v)
			}
			fmt.Printf("Conditions:\n")
			for _, c := range info.Conditions {
				fmt.Printf("  %-45s %-6s %s\n", c.Type+":", c.Status, c.Message)
			}
			return nil
		},
	}
}
