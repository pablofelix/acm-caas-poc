//go:build integration

package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/joho/godotenv"

	"github.com/pablofelix/acm-caas-poc/internal/client"
	"github.com/pablofelix/acm-caas-poc/internal/config"
	"github.com/pablofelix/acm-caas-poc/internal/fleet"
	"github.com/pablofelix/acm-caas-poc/internal/importing"
	"github.com/pablofelix/acm-caas-poc/internal/lifecycle"
	"github.com/pablofelix/acm-caas-poc/internal/monitoring"
	"github.com/pablofelix/acm-caas-poc/internal/policy"
	"github.com/pablofelix/acm-caas-poc/internal/provisioning"
	"github.com/pablofelix/acm-caas-poc/internal/registry"
	"github.com/pablofelix/acm-caas-poc/internal/scaling"
	"github.com/pablofelix/acm-caas-poc/internal/tenant"
)

type suiteContext struct {
	client      *client.Client
	cfg         config.Config
	fleet       *fleet.Inspector
	policy      *policy.Manager
	tenant      *tenant.Manager
	monitoring  *monitoring.Monitor
	lifecycle   *lifecycle.Manager
	importing   *importing.Manager
	scaling     *scaling.Manager
	registry    *registry.Manager
	provisioner *provisioning.Manager

	err            error
	clusters       []fleet.ClusterInfo
	cluster        *fleet.ClusterInfo
	policyInfo     *policy.PolicyInfo
	policies       []policy.PolicyInfo
	tenants        []tenant.TenantInfo
	manifestStatus *tenant.ManifestStatus
	resources      *monitoring.ClusterResources
	resourceList   []monitoring.ClusterResources
	importStatus   *importing.ImportStatus
	importList     []importing.ImportStatus
	machinePool    *scaling.MachinePoolInfo
	machinePools   []scaling.MachinePoolInfo
	images         []registry.RequiredImage
	mirrorStatus   *registry.MirrorStatus
	mirrorScript   string
	powerState         lifecycle.PowerState
	provisionList      []provisioning.ClusterInfo
	lifecycleCluster       string
	lifecycleNamespace     string
	lastManifestWorkName   string
	lastManifestWorkNS     string
	importKubeconfig       []byte
	lastCredential         *provisioning.CentralCredential
}

func TestFeatures(t *testing.T) {
	_ = godotenv.Load("../.env")

	tags := os.Getenv("GODOG_TAGS")
	if tags == "" {
		tags = "@core"
	}

	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../features"},
			Tags:     tags,
			Strict:   true,
			Output:   colors.Colored(os.Stdout),
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero exit from godog suite")
	}
}

func InitializeScenario(sc *godog.ScenarioContext) {
	s := &suiteContext{}

	sc.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		s.err = nil
		s.policyInfo = nil
		s.policies = nil
		s.tenants = nil
		s.manifestStatus = nil
		s.importStatus = nil
		s.importList = nil
		s.importKubeconfig = nil
		s.clusters = nil
		s.cluster = nil
		s.powerState = ""
		s.lifecycleCluster = ""
		s.lifecycleNamespace = ""
		s.lastManifestWorkName = ""
		s.lastManifestWorkNS = ""
		return ctx, nil
	})

	// Shared steps
	sc.Step(`^the ACM hub is (?:reachable|healthy)$`, s.theACMHubIsReachable)
	sc.Step(`^I have a dynamic client for (?:the hub|hive\.openshift\.io/v1)$`, s.iHaveADynamicClient)

	// Fleet steps
	registerFleetSteps(sc, s)

	// Policy steps
	registerPolicySteps(sc, s)

	// Tenant steps
	registerTenantSteps(sc, s)

	// Monitoring steps
	registerMonitoringSteps(sc, s)

	// Lifecycle steps
	registerLifecycleSteps(sc, s)

	// Importing steps
	registerImportingSteps(sc, s)

	// Scaling steps
	registerScalingSteps(sc, s)

	// Registry steps
	registerRegistrySteps(sc, s)

	// Provisioning steps
	registerProvisioningSteps(sc, s)
}

func (s *suiteContext) theACMHubIsReachable(ctx context.Context) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}
	s.cfg = cfg

	c, err := client.NewFromContext(cfg.Kubeconfig, cfg.HubContext)
	if err != nil {
		return err
	}
	s.client = c

	logger := slog.Default()
	s.fleet = fleet.New(c, cfg, logger)
	s.policy = policy.New(c, cfg, logger)
	s.tenant = tenant.New(c, cfg, logger)
	s.monitoring = monitoring.New(c, cfg, logger)
	s.lifecycle = lifecycle.New(c, cfg, logger)
	s.importing = importing.New(c, cfg, logger)
	s.scaling = scaling.New(c, cfg, logger)
	s.registry = registry.New(c, cfg, logger)
	s.provisioner = provisioning.New(c, cfg, logger)

	return nil
}

func (s *suiteContext) resolveCluster(name string) string {
	return s.cfg.ResolveCluster(name)
}

func (s *suiteContext) iHaveADynamicClient(ctx context.Context) error {
	if s.client == nil {
		return s.theACMHubIsReachable(ctx)
	}
	return nil
}
