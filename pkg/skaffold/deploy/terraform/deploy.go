package terraform

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/access"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/debug"
	deployerr "github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/deploy/error"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/deploy/label"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/deploy/types"
	dockerutil "github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/docker/debugger"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/docker/logger"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/docker/tracker"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/graph"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/kubernetes/manifest"
	olog "github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/output/log"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/status"
	pkgsync "github.com/GoogleContainerTools/skaffold/v2/pkg/skaffold/sync"
)

type Deployer struct {
	configName string
	cfg        *latest.TerraformDeploy

	debugger           debug.Debugger
	logger             *logger.Logger
	syncer             pkgsync.Syncer
	monitor            status.Monitor
	tracker            *tracker.ContainerTracker
	client             dockerutil.LocalDaemon
	network            string
	networkDeployed    bool
	globalConfig       string
	insecureRegistries map[string]bool
	resources          []*latest.PortForwardResource
	once               sync.Once
	labeller           *label.DefaultLabeller
}

func NewDeployer(ctx context.Context, cfg dockerutil.Config, labeller *label.DefaultLabeller, d *latest.TerraformDeploy, resources []*latest.PortForwardResource, artifacts []*latest.Artifact, configName string) (*Deployer, error) {

	client, err := dockerutil.NewAPIClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tracker := tracker.NewContainerTracker()
	l, err := logger.NewLogger(ctx, tracker, cfg, true)
	if err != nil {
		return nil, err
	}

	var dbg *debugger.DebugManager
	if cfg.ContainerDebugging() {
		debugHelpersRegistry, err := config.GetDebugHelpersRegistry(cfg.GlobalConfig())
		if err != nil {
			return nil, deployerr.DebugHelperRetrieveErr(fmt.Errorf("retrieving debug helpers registry: %w", err))
		}
		dbg = debugger.NewDebugManager(cfg.GetInsecureRegistries(), debugHelpersRegistry)
	}

	return &Deployer{
		cfg:                d,
		globalConfig:       cfg.GlobalConfig(),
		insecureRegistries: cfg.GetInsecureRegistries(),
		tracker:            tracker,

		client:   client,
		debugger: dbg,
		logger:   l,
		syncer:   pkgsync.NewContainerSyncer(),
		monitor:  &status.NoopMonitor{},
		labeller: labeller,
	}, nil
}

func (d *Deployer) Deploy(ctx context.Context, out io.Writer, builds []types.Artifact) error {
	//d.logger.Println("Starting Terraform deployment")
	olog.Entry(ctx).Warnf("unable to retrieve mount from debug init container: debugging may not work correctly!")
	// Your Terraform deployment logic here
	fmt.Println("Deploying Terraform")
	//d.logger.Println("Finished Terraform deployment")
	return nil
}

func (d *Deployer) Cleanup(ctx context.Context, out io.Writer, dryRun bool, byConfig manifest.ManifestListByConfig) error {
	//d.logger.Println("Starting Terraform cleanup")
	olog.Entry(ctx).Warnf("unable to retrieve mount from debug init container: debugging may not work correctly!")
	// Your Terraform cleanup logic here

	//d.logger.Println("Finished Terraform cleanup")
	return nil
}

func (d *Deployer) GetAccessor() access.Accessor {
	fmt.Println("Deploying Terraform")
	//olog.Entry(ctx).Warnf("unable to retrieve mount from debug init container: debugging may not work correctly!")
	return nil
}

func (d *Deployer) GetDebugger() debug.Debugger {
	fmt.Println("Deploying Terraform")
	//olog.Entry(ctx).Warnf("unable to retrieve mount from debug init container: debugging may not work correctly!")
	return d.debugger
}

func (d *Deployer) GetLogger() *logger.Logger {
	fmt.Println("Deploying Terraform")
	return d.logger
}

func (d *Deployer) GetSyncer() pkgsync.Syncer {
	fmt.Println("Deploying Terraform")
	return d.syncer
}

func (d *Deployer) GetStatusMonitor() status.Monitor {
	fmt.Println("Deploying Terraform")
	return d.monitor
}

func (d *Deployer) RegisterLocalImages([]graph.Artifact) {
	fmt.Println("Deploying Terraform")
	// all images are local, so this is a noop
}
