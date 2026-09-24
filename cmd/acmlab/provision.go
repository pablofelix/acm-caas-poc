package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pablofelix/acm-caas-poc/internal/batch"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
)

func provisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision and manage spoke clusters (Hive, HyperShift, CAPI)",
	}
	cmd.AddCommand(
		provisionPreflightCmd(),
		provisionOrphanCheckCmd(),
		provisionCreateCmd(),
		provisionDestroyCmd(),
		provisionStatusCmd(),
		provisionListCmd(),
		provisionImageSetsCmd(),
		provisionListCAPICmd(),
		provisionListHostedCmd(),
		provisionTemplateCreateCmd(),
		provisionTemplateGetCmd(),
		provisionTemplateListCmd(),
		provisionTemplateRemoveCmd(),
		provisionTemplateApplyCmd(),
	)
	return cmd
}

func provisionPreflightCmd() *cobra.Command {
	var platform, region, pullSecretFile string
	cmd := &cobra.Command{
		Use:   "preflight <cluster-name>",
		Short: "Run preflight checks before provisioning (credentials, quota, image sets)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)

			opts := provisioning.ClusterOpts{
				Name:     args[0],
				Platform: platform,
				Region:   region,
			}

			if pullSecretFile != "" {
				data, err := os.ReadFile(pullSecretFile)
				if err != nil {
					return fmt.Errorf("reading pull secret: %w", err)
				}
				opts.PullSecret = string(data)
			}

			if opts.Platform == "aws" || (opts.Platform == "" && cfg.Platform == "aws") {
				awsCreds, err := provisioning.LoadAWSCredentials("")
				if err != nil {
					return fmt.Errorf("loading AWS credentials: %w", err)
				}
				opts.AWSAccessKeyID = awsCreds.AccessKeyID
				opts.AWSSecretAccessKey = awsCreds.SecretAccessKey
			}

			results, err := mgr.Preflight(context.Background(), opts)
			if err != nil {
				return fmt.Errorf("preflight: %w", err)
			}
			fmt.Print(provisioning.FormatPreflightResults(results))
			if !provisioning.PreflightPassed(results) {
				return fmt.Errorf("preflight checks failed")
			}
			fmt.Println("All preflight checks passed — safe to provision.")
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "", "Cloud platform: ibmcloud, aws")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region")
	cmd.Flags().StringVar(&pullSecretFile, "pull-secret", "", "Path to pull secret file")
	return cmd
}

func provisionOrphanCheckCmd() *cobra.Command {
	var infraID, platform, region string
	cmd := &cobra.Command{
		Use:   "orphan-check [cluster-name]",
		Short: "Check for orphaned cloud resources from a cluster's infrastructure",
		Long: `Queries the cloud provider for resources matching the cluster's infraID.
Use after destroy to verify all resources were cleaned up, or proactively to find leaked resources.

When the ClusterDeployment still exists, pass its name as the argument.
After the cluster has been destroyed, use --infra-id, --platform, and --region instead:

  acmlab provision orphan-check --infra-id <infra-id> --platform aws --region us-east-1`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			ctx := context.Background()

			if infraID != "" || platform != "" || region != "" {
				if infraID == "" || platform == "" || region == "" {
					return fmt.Errorf("--infra-id, --platform, and --region must all be provided together")
				}
			} else {
				if len(args) == 0 {
					return fmt.Errorf("provide a cluster name or use --infra-id, --platform, and --region")
				}
				var captureErr error
				infraID, platform, region, captureErr = mgr.CaptureInfraID(ctx, args[0])
				if captureErr != nil {
					return fmt.Errorf("reading cluster metadata: %w", captureErr)
				}
			}

			result, err := mgr.CheckOrphans(ctx, infraID, platform, region)
			if err != nil {
				return fmt.Errorf("orphan check: %w", err)
			}

			fmt.Print(provisioning.FormatOrphanCheckResult(result))
			if !result.Clean {
				return fmt.Errorf("%d orphaned resources found", len(result.Orphans))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&infraID, "infra-id", "", "Infrastructure ID (use after cluster is destroyed)")
	cmd.Flags().StringVar(&platform, "platform", "", "Cloud platform: ibmcloud, aws (required with --infra-id)")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region (required with --infra-id)")
	return cmd
}

func provisionCreateCmd() *cobra.Command {
	var platform, region, baseDomain, imageSet, workerType, masterType, sshKeyFile, sshPrivateKeyFile, pullSecretFile, manifestsDir string
	var clusterType, kubernetesVersion, infraProvider, releaseImage string
	var workers, masters int64
	cmd := &cobra.Command{
		Use:   "create <cluster-name>",
		Short: "Create a spoke cluster (Hive, HyperShift, CAPI)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)

			if clusterType == "hypershift" {
				opts := provisioning.HyperShiftOpts{
					Name:             args[0],
					Platform:         platform,
					Region:           region,
					BaseDomain:       baseDomain,
					ReleaseImage:     releaseImage,
					NodePoolReplicas: workers,
				}
				if pullSecretFile != "" {
					data, err := os.ReadFile(pullSecretFile)
					if err != nil {
						return fmt.Errorf("reading pull secret: %w", err)
					}
					opts.PullSecret = string(data)
				}
				fmt.Printf("Creating HyperShift hosted cluster %s...\n", args[0])
				if err := mgr.CreateHyperShift(context.Background(), opts); err != nil {
					return err
				}
				fmt.Println("HostedCluster and NodePool created. Control plane runs as pods on the hub.")
				fmt.Println("Use 'acmlab provision status' to monitor progress.")
				return nil
			}

			if clusterType == "capi" {
				opts := provisioning.CAPIClusterOpts{
					Name:              args[0],
					InfraProvider:     infraProvider,
					KubernetesVersion: kubernetesVersion,
					WorkerReplicas:    workers,
				}
				if pullSecretFile != "" {
					data, err := os.ReadFile(pullSecretFile)
					if err != nil {
						return fmt.Errorf("reading pull secret: %w", err)
					}
					opts.PullSecret = string(data)
				}
				fmt.Printf("Creating CAPI cluster %s (provider: %s)...\n", args[0], opts.InfraProvider)
				if err := mgr.CreateCAPI(context.Background(), opts); err != nil {
					return err
				}
				fmt.Println("CAPI Cluster and MachineDeployment created.")
				fmt.Println("Use 'acmlab provision status' to monitor progress.")
				return nil
			}

			opts := provisioning.ClusterOpts{
				Name:           args[0],
				Platform:       platform,
				BaseDomain:     baseDomain,
				Region:         region,
				ImageSet:       imageSet,
				WorkerType:     workerType,
				MasterType:     masterType,
				WorkerReplicas: workers,
				MasterReplicas: masters,
				ManifestsDir:   manifestsDir,
			}

			if pullSecretFile != "" {
				data, err := os.ReadFile(pullSecretFile)
				if err != nil {
					return fmt.Errorf("reading pull secret: %w", err)
				}
				opts.PullSecret = string(data)
			}

			if sshKeyFile != "" {
				data, err := os.ReadFile(sshKeyFile)
				if err != nil {
					return fmt.Errorf("reading SSH public key: %w", err)
				}
				opts.SSHKey = string(data)
			}

			if sshPrivateKeyFile != "" {
				data, err := os.ReadFile(sshPrivateKeyFile)
				if err != nil {
					return fmt.Errorf("reading SSH private key: %w", err)
				}
				opts.SSHPrivateKey = string(data)
			}

			if opts.Platform == "aws" || (opts.Platform == "" && cfg.Platform == "aws") {
				awsCreds, err := provisioning.LoadAWSCredentials("")
				if err != nil {
					return fmt.Errorf("loading AWS credentials: %w", err)
				}
				opts.AWSAccessKeyID = awsCreds.AccessKeyID
				opts.AWSSecretAccessKey = awsCreds.SecretAccessKey
			}

			fmt.Println("Running preflight checks...")
			results, err := mgr.Preflight(context.Background(), opts)
			if err != nil {
				return fmt.Errorf("preflight: %w", err)
			}
			fmt.Print(provisioning.FormatPreflightResults(results))
			if !provisioning.PreflightPassed(results) {
				return fmt.Errorf("preflight checks failed — fix the issues above before provisioning")
			}

			fmt.Printf("Creating cluster %s in %s...\n", args[0], opts.Region)
			if err := mgr.Create(context.Background(), opts); err != nil {
				return err
			}
			fmt.Println("ClusterDeployment created. Hive will now provision the cluster.")
			fmt.Println("Use 'acmlab provision status' to monitor progress.")
			return nil
		},
	}
	cmd.Flags().StringVar(&clusterType, "type", "", "provisioning type: hive (default), hypershift, capi")
	cmd.Flags().StringVar(&releaseImage, "release-image", "", "OCP release image for HyperShift clusters")
	cmd.Flags().StringVar(&kubernetesVersion, "kubernetes-version", "", "Kubernetes version for CAPI clusters (default: v1.30.0)")
	cmd.Flags().StringVar(&infraProvider, "infra-provider", "", "CAPI infrastructure provider: docker, aws, azure, gcp (default: docker)")
	cmd.Flags().StringVar(&platform, "platform", "", "cloud platform: ibmcloud, aws, gcp, azure (default: from env)")
	cmd.Flags().StringVar(&baseDomain, "base-domain", "", "base DNS domain for the cluster (default: from ACM_BASE_DOMAIN env)")
	cmd.Flags().StringVar(&region, "region", "", "cloud region (default: from env)")
	cmd.Flags().StringVar(&imageSet, "image-set", "", "ClusterImageSet name (default: from env)")
	cmd.Flags().StringVar(&workerType, "worker-type", "", "worker instance type (default: bx2-4x16)")
	cmd.Flags().StringVar(&masterType, "master-type", "", "master instance type (default: bx2-8x32)")
	cmd.Flags().Int64Var(&workers, "workers", 0, "number of worker nodes (default: 2)")
	cmd.Flags().Int64Var(&masters, "masters", 0, "number of master nodes (default: 3)")
	cmd.Flags().StringVar(&pullSecretFile, "pull-secret", "", "path to pull secret file (required)")
	cmd.Flags().StringVar(&sshKeyFile, "ssh-key", "", "path to SSH public key file")
	cmd.Flags().StringVar(&sshPrivateKeyFile, "ssh-private-key", "", "path to SSH private key file")
	cmd.Flags().StringVar(&manifestsDir, "manifests-dir", "", "path to ccoctl-generated manifests directory (optional)")
	return cmd
}

func provisionDestroyCmd() *cobra.Command {
	var fromFile string
	var concurrency int
	var outputJSON bool
	var checkOrphans bool

	cmd := &cobra.Command{
		Use:   "destroy [name...]",
		Short: "Destroy one or more provisioned clusters",
		Long: `Destroy one or more provisioned clusters.
With --check-orphans (single cluster only), captures the infraID before destroying,
waits for the ClusterDeployment to be fully removed, then checks for orphaned cloud resources.`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			var fileItems []batch.ClusterItem
			if fromFile != "" {
				var err error
				fileItems, err = batch.LoadFile(fromFile)
				if err != nil {
					return err
				}
			}
			items, err := batch.NamesFromArgs(args, fileItems)
			if err != nil {
				return err
			}

			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			ctx := context.Background()

			if checkOrphans {
				if len(items) != 1 {
					return fmt.Errorf("--check-orphans requires exactly one cluster name")
				}
				result, err := mgr.DestroyWithOrphanCheck(ctx, items[0].Name)
				if err != nil {
					return err
				}
				fmt.Print(provisioning.FormatOrphanCheckResult(result))
				if !result.Clean {
					return fmt.Errorf("%d orphaned resources found", len(result.Orphans))
				}
				return nil
			}

			work := make([]batch.Work, len(items))
			for i, item := range items {
				item := item
				work[i] = batch.Work{
					Name: item.Name,
					Run: func(ctx context.Context) (string, error) {
						if err := mgr.Destroy(ctx, item.Name); err != nil {
							return "", err
						}
						return "destruction initiated", nil
					},
				}
			}

			results := batch.Execute(ctx, work, concurrency, os.Stdout)
			if outputJSON {
				data, _ := batch.ToJSON(results)
				fmt.Println(string(data))
			} else {
				batch.PrintSummary(results, os.Stdout)
			}
			for _, r := range results {
				if !r.OK {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "YAML file with cluster list")
	cmd.Flags().IntVar(&concurrency, "concurrency", 5, "Max parallel operations (max 20)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output results as JSON array")
	cmd.Flags().BoolVar(&checkOrphans, "check-orphans", false, "Capture infraID, destroy, wait, then check for orphaned resources (single cluster only)")
	return cmd
}

func provisionStatusCmd() *cobra.Command {
	var fromFile string
	var concurrency int
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "status [name...]",
		Short: "Show provisioning status of one or more clusters",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			var fileItems []batch.ClusterItem
			if fromFile != "" {
				var err error
				fileItems, err = batch.LoadFile(fromFile)
				if err != nil {
					return err
				}
			}
			items, err := batch.NamesFromArgs(args, fileItems)
			if err != nil {
				return err
			}

			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			ctx := context.Background()

			// single-item: preserve existing detailed output
			if len(items) == 1 && fromFile == "" {
				info, err := mgr.Status(ctx, items[0].Name)
				if err != nil {
					return err
				}
				if outputJSON {
					data, _ := json.MarshalIndent(info, "", "  ")
					fmt.Println(string(data))
					return nil
				}
				fmt.Printf("Cluster:      %s\n", info.Name)
				fmt.Printf("BaseDomain:   %s\n", info.BaseDomain)
				fmt.Printf("Region:       %s\n", info.Region)
				fmt.Printf("ImageSet:     %s\n", info.ImageSet)
				fmt.Printf("Installed:    %v\n", info.Installed)
				fmt.Printf("Provisioned:  %v\n", info.Provisioned)
				if info.FailureReason != "" {
					fmt.Printf("Failure:      %s\n", info.FailureReason)
				}
				if len(info.Conditions) > 0 {
					fmt.Printf("Conditions:   %v\n", info.Conditions)
				}
				return nil
			}

			work := make([]batch.Work, len(items))
			for i, item := range items {
				item := item
				work[i] = batch.Work{
					Name: item.Name,
					Run: func(ctx context.Context) (string, error) {
						s, err := mgr.Status(ctx, item.Name)
						if err != nil {
							return "", err
						}
						return fmt.Sprintf("Installed=%v Provisioned=%v", s.Installed, s.Provisioned), nil
					},
				}
			}
			results := batch.Execute(ctx, work, concurrency, os.Stdout)
			if outputJSON {
				data, _ := batch.ToJSON(results)
				fmt.Println(string(data))
			} else {
				batch.PrintSummary(results, os.Stdout)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "YAML file with cluster list")
	cmd.Flags().IntVar(&concurrency, "concurrency", 5, "Max parallel operations (max 20)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func provisionListCmd() *cobra.Command {
	var outputJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clusters provisioned via acmlab",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			clusters, err := mgr.List(context.Background())
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				fmt.Println("No clusters provisioned via acmlab")
				return nil
			}
			if outputJSON {
				data, _ := json.MarshalIndent(clusters, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Printf("%-20s %-25s %-12s %-10s %s\n", "NAME", "DOMAIN", "REGION", "INSTALLED", "IMAGE SET")
			for _, c := range clusters {
				fmt.Printf("%-20s %-25s %-12s %-10v %s\n", c.Name, c.BaseDomain, c.Region, c.Installed, c.ImageSet)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func provisionListCAPICmd() *cobra.Command {
	var outputJSON bool
	cmd := &cobra.Command{
		Use:   "list-capi",
		Short: "List CAPI-provisioned vanilla Kubernetes clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			clusters, err := mgr.ListCAPI(context.Background())
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				fmt.Println("No CAPI clusters provisioned via acmlab")
				return nil
			}
			if outputJSON {
				data, _ := json.MarshalIndent(clusters, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Printf("%-20s %-20s %-15s %-8s %s\n", "NAME", "NAMESPACE", "PHASE", "READY", "K8S VERSION")
			for _, c := range clusters {
				fmt.Printf("%-20s %-20s %-15s %-8v %s\n", c.Name, c.Namespace, c.Phase, c.Ready, c.KubernetesVersion)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func provisionListHostedCmd() *cobra.Command {
	var outputJSON bool
	cmd := &cobra.Command{
		Use:   "list-hosted",
		Short: "List HyperShift hosted clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			clusters, err := mgr.ListHyperShift(context.Background())
			if err != nil {
				return err
			}
			if len(clusters) == 0 {
				fmt.Println("No HyperShift hosted clusters provisioned via acmlab")
				return nil
			}
			if outputJSON {
				data, _ := json.MarshalIndent(clusters, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			fmt.Printf("%-20s %-15s %-10s %s\n", "NAME", "NAMESPACE", "AVAILABLE", "VERSION")
			for _, c := range clusters {
				fmt.Printf("%-20s %-15s %-10v %s\n", c.Name, c.Namespace, c.Available, c.Version)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func provisionImageSetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "image-sets",
		Short: "List available ClusterImageSets",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			sets, err := mgr.ListImageSets(context.Background())
			if err != nil {
				return err
			}
			if len(sets) == 0 {
				fmt.Println("No ClusterImageSets found")
				return nil
			}
			fmt.Printf("%-35s %s\n", "NAME", "RELEASE IMAGE")
			for _, s := range sets {
				img := s.ReleaseImage
				if len(img) > 60 {
					img = img[:57] + "..."
				}
				fmt.Printf("%-35s %s\n", s.Name, img)
			}
			return nil
		},
	}
}

func provisionTemplateCreateCmd() *cobra.Command {
	var patches []string
	cmd := &cobra.Command{
		Use:   "template-create <name>",
		Short: "Create a ClusterDeploymentCustomization template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed := make([]provisioning.TemplatePatch, 0, len(patches))
			for _, p := range patches {
				parts := strings.SplitN(p, ":", 3)
				if len(parts) != 3 {
					return fmt.Errorf("invalid patch format %q (expected op:path:value)", p)
				}
				parsed = append(parsed, provisioning.TemplatePatch{Op: parts[0], Path: parts[1], Value: parts[2]})
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			if err := mgr.CreateTemplate(context.Background(), args[0], parsed); err != nil {
				return err
			}
			fmt.Printf("Template %s created with %d patches\n", args[0], len(parsed))
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&patches, "patch", nil, "install config patches (format: op:path:value)")
	return cmd
}

func provisionTemplateGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "template-get <name>",
		Short: "Get details of a cluster template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			obj, err := mgr.GetTemplate(context.Background(), args[0])
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(obj, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func provisionTemplateListCmd() *cobra.Command {
	var outputJSON bool
	cmd := &cobra.Command{
		Use:   "template-list",
		Short: "List all cluster templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			templates, err := mgr.ListTemplates(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				data, _ := json.MarshalIndent(templates, "", "  ")
				fmt.Println(string(data))
				return nil
			}
			if len(templates) == 0 {
				fmt.Println("No templates found")
				return nil
			}
			fmt.Printf("%-30s %s\n", "NAME", "PATCHES")
			for _, t := range templates {
				fmt.Printf("%-30s %d\n", t.Name, t.Patches)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output as JSON")
	return cmd
}

func provisionTemplateRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "template-remove <name>",
		Short: "Remove a cluster template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			if err := mgr.RemoveTemplate(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Template %s removed\n", args[0])
			return nil
		},
	}
}

func provisionTemplateApplyCmd() *cobra.Command {
	var template string
	cmd := &cobra.Command{
		Use:   "template-apply <cluster>",
		Short: "Apply a cluster template to a ClusterDeployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if template == "" {
				return fmt.Errorf("--template is required")
			}
			c, err := buildClient()
			if err != nil {
				return err
			}
			mgr := provisioning.New(c, cfg, logger)
			if err := mgr.ApplyTemplate(context.Background(), args[0], template); err != nil {
				return err
			}
			fmt.Printf("Template %s applied to %s\n", template, args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "template name (required)")
	return cmd
}
