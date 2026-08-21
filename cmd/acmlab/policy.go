package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/policy"
)

func policyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage governance policies on managed clusters",
	}
	cmd.AddCommand(
		policyListCmd(),
		policyStatusCmd(),
		policyApplyCmd(),
		policyRemoveCmd(),
		policySetRemediationCmd(),
		policyEnableCmd(),
		policyDisableCmd(),
	)
	return cmd
}

func policyListCmd() *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List policies and compliance status",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			policies, err := mgr.List(context.Background(), namespace)
			if err != nil {
				return err
			}
			if len(policies) == 0 {
				fmt.Println("No policies found")
				return nil
			}
			fmt.Printf("%-35s %-12s %-10s %-15s\n", "NAME", "REMEDIATION", "DISABLED", "COMPLIANT")
			for _, p := range policies {
				compliant := p.Compliant
				if compliant == "" {
					compliant = "-"
				}
				fmt.Printf("%-35s %-12s %-10v %-15s\n", p.Name, p.RemediationAction, p.Disabled, compliant)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	return cmd
}

func policyStatusCmd() *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show detailed policy compliance status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			info, err := mgr.Get(context.Background(), args[0], namespace)
			if err != nil {
				return err
			}
			fmt.Printf("Policy:      %s\n", info.Name)
			fmt.Printf("Namespace:   %s\n", info.Namespace)
			fmt.Printf("Remediation: %s\n", info.RemediationAction)
			fmt.Printf("Disabled:    %v\n", info.Disabled)
			compliant := info.Compliant
			if compliant == "" {
				compliant = "Unknown"
			}
			fmt.Printf("Compliant:   %s\n", compliant)
			fmt.Println()
			if len(info.ClusterCompliance) > 0 {
				fmt.Printf("%-30s %s\n", "CLUSTER", "STATE")
				for _, cc := range info.ClusterCompliance {
					fmt.Printf("%-30s %s\n", cc.ClusterName, cc.ComplianceState)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	return cmd
}

func policyApplyCmd() *cobra.Command {
	var namespace, remediation, labels, registries string
	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Create a policy with placement (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			opts := policy.PolicyOpts{
				Name:              args[0],
				Namespace:         namespace,
				RemediationAction: remediation,
			}
			if labels != "" {
				opts.ClusterLabels = parseLabels(labels)
			}
			if registries != "" {
				opts.AllowedRegistries = strings.Split(registries, ",")
			}
			fmt.Printf("Applying policy %s...\n", args[0])
			if err := mgr.Apply(context.Background(), opts); err != nil {
				return err
			}
			fmt.Println("Policy applied. Use 'acmlab policy status' to check compliance.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	cmd.Flags().StringVarP(&remediation, "remediation", "r", "inform", "remediation action: inform or enforce")
	cmd.Flags().StringVarP(&labels, "labels", "l", "", "cluster label selector (key=value,key2=value2)")
	cmd.Flags().StringVar(&registries, "registries", "", "allowed registries (comma-separated)")
	return cmd
}

func policyRemoveCmd() *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a policy and its placement — clean, no leftovers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			fmt.Printf("Removing policy %s...\n", args[0])
			if err := mgr.Remove(context.Background(), args[0], namespace); err != nil {
				return err
			}
			fmt.Println("Policy removed. All resources cleaned up.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	return cmd
}

func policySetRemediationCmd() *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "set-remediation <name> <inform|enforce>",
		Short: "Change policy remediation action",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			if err := mgr.SetRemediation(context.Background(), args[0], namespace, args[1]); err != nil {
				return err
			}
			fmt.Printf("Policy %s remediation set to %s\n", args[0], args[1])
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	return cmd
}

func policyEnableCmd() *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a disabled policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			if err := mgr.SetDisabled(context.Background(), args[0], namespace, false); err != nil {
				return err
			}
			fmt.Printf("Policy %s enabled\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	return cmd
}

func policyDisableCmd() *cobra.Command {
	var namespace string
	cmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a policy without removing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := policy.New(c, cfg)
			if err := mgr.SetDisabled(context.Background(), args[0], namespace, true); err != nil {
				return err
			}
			fmt.Printf("Policy %s disabled\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "policy namespace (default: global-set)")
	return cmd
}

func parseLabels(s string) map[string]string {
	labels := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			labels[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return labels
}
