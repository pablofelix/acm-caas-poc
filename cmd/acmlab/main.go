package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
)

var cfg config.Config

func main() {
	_ = godotenv.Load()

	var err error
	cfg, err = config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	root := &cobra.Command{
		Use:   "acmlab",
		Short: "ACM CaaS PoC Lab CLI",
	}

	root.PersistentFlags().StringVar(&cfg.Kubeconfig, "kubeconfig", cfg.Kubeconfig, "Path to kubeconfig")
	root.PersistentFlags().StringVar(&cfg.HubContext, "context", cfg.HubContext, "Kubernetes context for the hub")

	root.AddCommand(versionCmd())
	root.AddCommand(fleetCmd())
	root.AddCommand(monitorCmd())
	root.AddCommand(policyCmd())
	root.AddCommand(mcpCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print acmlab version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("acmlab v0.1.0 (acm-caas-poc)")
		},
	}
}

func buildClient() (*client.Client, error) {
	if cfg.Kubeconfig != "" || cfg.HubContext != "" {
		return client.NewFromContext(cfg.Kubeconfig, cfg.HubContext)
	}
	return client.NewFromDefault()
}
