package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/access"
)

func accessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Manage credential-free hub-to-spoke access via ManagedServiceAccount",
	}
	cmd.AddCommand(accessEnableCmd(), accessDisableCmd(), accessStatusCmd(), accessListCmd(), accessEnableProxyCmd(), accessDisableProxyCmd(), accessProxyStatusCmd())
	return cmd
}

func accessEnableCmd() *cobra.Command {
	var ttl string
	var roles []string

	cmd := &cobra.Command{
		Use:   "enable <cluster>",
		Short: "Enable managed access on a spoke cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			opts := access.AccessOpts{
				TTL:   ttl,
				Roles: roles,
			}
			if err := mgr.Enable(context.Background(), args[0], opts); err != nil {
				return err
			}
			fmt.Printf("Managed access enabled on %s (TTL=%s)\n", args[0], ttl)
			return nil
		},
	}
	cmd.Flags().StringVar(&ttl, "ttl", "720h", "Token rotation interval")
	cmd.Flags().StringSliceVar(&roles, "roles", []string{"cluster-admin"}, "RBAC roles for the service account")
	return cmd
}

func accessDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <cluster>",
		Short: "Disable managed access on a spoke cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			removed, err := mgr.Disable(context.Background(), args[0])
			if err != nil {
				return err
			}
			if !removed {
				fmt.Printf("Managed access not found on %s (nothing to remove)\n", args[0])
			} else {
				fmt.Printf("Managed access disabled on %s\n", args[0])
			}
			return nil
		},
	}
	return cmd
}

func accessStatusCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "status <cluster>",
		Short: "Show managed access status for a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			status, err := mgr.GetStatus(context.Background(), args[0])
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(status, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Cluster:         %s\n", status.Cluster)
			fmt.Printf("Enabled:         %v\n", status.Enabled)
			fmt.Printf("Token Available: %v\n", status.TokenAvailable)
			fmt.Printf("Token Rotation:  %s\n", status.TokenRotation)
			fmt.Printf("Addon Healthy:   %v\n", status.AddonHealthy)
			if status.LastRotation != "" {
				fmt.Printf("Last Rotation:   %s\n", status.LastRotation)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func accessListCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clusters with managed access enabled",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			infos, err := mgr.List(context.Background())
			if err != nil {
				return err
			}

			if outputJSON {
				data, _ := json.MarshalIndent(infos, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(infos) == 0 {
				fmt.Println("No clusters with managed access")
				return nil
			}

			fmt.Printf("%-20s  %-7s  %-5s  %-15s\n", "CLUSTER", "ENABLED", "TOKEN", "ADDON STATUS")
			for _, info := range infos {
				fmt.Printf("%-20s  %-7v  %-5v  %-15s\n", info.Cluster, info.Enabled, info.TokenAvailable, info.AddonStatus)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func accessEnableProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable-proxy <cluster>",
		Short: "Enable cluster proxy on a spoke cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			if err := mgr.EnableProxy(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Cluster proxy enabled on %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func accessDisableProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable-proxy <cluster>",
		Short: "Disable cluster proxy on a spoke cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			removed, err := mgr.DisableProxy(context.Background(), args[0])
			if err != nil {
				return err
			}
			if removed {
				fmt.Printf("Cluster proxy disabled on %s\n", args[0])
			} else {
				fmt.Printf("Cluster proxy not found on %s (nothing to remove)\n", args[0])
			}
			return nil
		},
	}
	return cmd
}

func accessProxyStatusCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "proxy-status <cluster>",
		Short: "Show cluster proxy status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := access.New(c, cfg, logger)
			status, err := mgr.GetProxyStatus(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				data, _ := json.MarshalIndent(status, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Printf("Cluster:   %s\n", status.Cluster)
			fmt.Printf("Enabled:   %v\n", status.Enabled)
			fmt.Printf("Healthy:   %v\n", status.Healthy)
			if status.Endpoint != "" {
				fmt.Printf("Endpoint:  %s\n", status.Endpoint)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}
