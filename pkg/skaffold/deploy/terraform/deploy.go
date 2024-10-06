package terraform

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/access"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/debug"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/graph"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/kubernetes/manifest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/log"
	olog "github.com/ryanharper/skaffold/v2/pkg/skaffold/output/log"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/status"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/sync"
)

type Deployer struct {
	configName string
	*latest.TerraformDeploy
}

func NewDeployer(cfg latest.TerraformDeploy, configName string) (*Deployer, error) {
	return &Deployer{
		configName:      configName,
		TerraformDeploy: &cfg,
	}, nil
}

func (t *Deployer) Deploy(ctx context.Context, out io.Writer, builds []graph.Artifact, labellers manifest.ManifestListByConfig) error {
	olog.Entry(ctx).Infof("Terraform Deployer: Starting deployment for config %s", t.configName)

	// Create a map of deployments by name for easy lookup
	deploymentMap := make(map[string]*latest.TerrformDeployments)
	for i := range t.Deployments {
		deploymentMap[t.Deployments[i].Name] = &t.Deployments[i]
	}

	// Create a list to store the order of deployments
	deploymentOrder := make([]*latest.TerrformDeployments, 0, len(t.Deployments))

	// Create a set to keep track of deployments in the current path
	visited := make(map[string]bool)

	// Helper function to add a deployment and its dependencies to the order
	var addToOrder func(*latest.TerrformDeployments) error
	addToOrder = func(deployment *latest.TerrformDeployments) error {
		// Check for self-dependency
		if contains(deployment.DependsOn, deployment.Name) {
			return fmt.Errorf("deployment %s depends on itself", deployment.Name)
		}

		// Check if the deployment is already in the order
		for _, d := range deploymentOrder {
			if d.Name == deployment.Name {
				return nil
			}
		}

		// Check for circular dependencies
		if visited[deployment.Name] {
			return fmt.Errorf("circular dependency detected involving %s", deployment.Name)
		}
		visited[deployment.Name] = true

		// Add dependencies first
		for _, depName := range deployment.DependsOn {
			if dep, ok := deploymentMap[depName]; ok {
				if err := addToOrder(dep); err != nil {
					return err
				}
			} else {
				olog.Entry(ctx).Warnf("Dependency %s not found for deployment %s", depName, deployment.Name)
			}
		}

		// Add the deployment itself
		deploymentOrder = append(deploymentOrder, deployment)
		delete(visited, deployment.Name) // Remove from visited set after processing
		return nil
	}

	// Build the deployment order
	for _, deployment := range t.Deployments {
		if err := addToOrder(&deployment); err != nil {
			return err
		}
	}

	// Execute deployments in order
	for _, deployment := range deploymentOrder {
		if err := t.deployTerraform(ctx, out, deployment); err != nil {
			return fmt.Errorf("failed to deploy %s: %w", deployment.Name, err)
		}
	}

	olog.Entry(ctx).Infof("Terraform Deployer: All deployments completed for config %s", t.configName)
	return nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func (t *Deployer) deployTerraform(ctx context.Context, out io.Writer, deployment *latest.TerrformDeployments) error {
	workingDir := deployment.Dir

	// Run terraform init
	initArgs := []string{"init"}
	for key, value := range deployment.BackendConfig {
		initArgs = append(initArgs, fmt.Sprintf("-backend-config=%s=%s", key, value))
	}
	if err := t.runTerraformCommand(ctx, out, workingDir, initArgs...); err != nil {
		return fmt.Errorf("failed to run terraform init: %w", err)
	}

	// Set or select workspace if specified
	if deployment.Workspace != "" {
		workspaceArgs := []string{"workspace", "select", "-or-create", deployment.Workspace}
		if err := t.runTerraformCommand(ctx, out, workingDir, workspaceArgs...); err != nil {
			return fmt.Errorf("failed to set terraform workspace: %w", err)
		}
	}

	// Prepare apply command with vars, var-files, and extra args
	applyArgs := []string{"apply"}
	for key, value := range deployment.Vars {
		applyArgs = append(applyArgs, "-var", fmt.Sprintf("%s=%s", key, value))
	}
	for _, varFile := range deployment.VarFiles {
		applyArgs = append(applyArgs, "-var-file", varFile)
	}
	applyArgs = append(applyArgs, deployment.ExtraArgs...)
	if deployment.AutoApprove {
		applyArgs = append(applyArgs, "-auto-approve")
	}

	// Run terraform apply
	if err := t.runTerraformCommand(ctx, out, workingDir, applyArgs...); err != nil {
		return fmt.Errorf("failed to run terraform apply: %w", err)
	}

	olog.Entry(ctx).Infof("Terraform Deployer: Deployment completed for %s", deployment.Name)
	return nil
}

func (t *Deployer) Dependencies() ([]string, error) {
	olog.Entry(context.Background()).Infof("Terraform Deployer: Checking dependencies")
	return nil, nil
}

func (t *Deployer) Cleanup(ctx context.Context, out io.Writer, dryRun bool, _ manifest.ManifestListByConfig) error {
	olog.Entry(ctx).Infof("Terraform Deployer: Starting cleanup for config %s", t.configName)

	for _, deployment := range t.Deployments {
		workingDir := deployment.Dir

		if dryRun {
			fmt.Fprintf(out, "Terraform Deployer: Would run 'terraform destroy' for %s (dry run)\n", deployment.Dir)
		} else {
			// Prepare destroy command with vars, var-files, and extra args
			destroyArgs := []string{"destroy"}
			for key, value := range deployment.Vars {
				destroyArgs = append(destroyArgs, "-var", fmt.Sprintf("%s=%s", key, value))
			}
			for _, varFile := range deployment.VarFiles {
				destroyArgs = append(destroyArgs, "-var-file", varFile)
			}
			destroyArgs = append(destroyArgs, deployment.ExtraArgs...)
			destroyArgs = append(destroyArgs, "-auto-approve")

			if err := t.runTerraformCommand(ctx, out, workingDir, destroyArgs...); err != nil {
				return fmt.Errorf("failed to run terraform destroy: %w", err)
			}
		}

		olog.Entry(ctx).Infof("Terraform Deployer: Cleanup completed for %s", deployment.Dir)
	}

	olog.Entry(ctx).Infof("Terraform Deployer: All cleanups completed for config %s", t.configName)
	return nil
}

func (t *Deployer) runTerraformCommand(ctx context.Context, out io.Writer, workingDir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "terraform", args...)
	cmd.Dir = workingDir
	cmd.Stdout = out
	cmd.Stderr = out

	olog.Entry(ctx).Infof("Running terraform command: %s", cmd.String())
	return cmd.Run()
}

func (t *Deployer) ConfigName() string {
	return t.configName
}

// Implement other necessary methods (returning nil or no-op for now)

// func (t *Deployer) GetAccessor() access.Accessor { return nil }
// func (t *Deployer) GetDebugger() debug.Debugger  { return nil }
func (t *Deployer) GetLogger() log.Logger {
	return &log.NoopLogger{} // or implement a proper logger if needed
}

// func (t *Deployer) GetStatusMonitor() status.Monitor                      { return nil }
// func (t *Deployer) GetSyncer() sync.Syncer                                { return nil }
func (t *Deployer) TrackBuildArtifacts(builds, deployed []graph.Artifact) {}
func (t *Deployer) RegisterLocalImages(images []graph.Artifact)           {}

func (t *Deployer) GetAccessor() access.Accessor     { return &access.NoopAccessor{} }
func (t *Deployer) GetDebugger() debug.Debugger      { return &debug.NoopDebugger{} }
func (t *Deployer) GetStatusMonitor() status.Monitor { return &status.NoopMonitor{} }
func (t *Deployer) GetSyncer() sync.Syncer           { return &sync.NoopSyncer{} }
