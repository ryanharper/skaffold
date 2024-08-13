package terraform

import (
	"context"
	//"fmt"
	"io"
	"os/exec"

	"github.com/ryanharper/skaffold/pkg/skaffold/schema/latest"
)

// TerraformDeployer deploys workflows using Terraform.
type TerraformDeployer struct {
	config *latest.TerraformDeploy
}

// NewTerraformDeployer creates a new TerraformDeployer.
func NewTerraformDeployer(cfg *latest.TerraformDeploy) *TerraformDeployer {
	return &TerraformDeployer{
		config: cfg,
	}
}

// Deploy deploys the application using Terraform.
func (d *TerraformDeployer) Deploy(ctx context.Context, out io.Writer) error {
	// cmd := exec.CommandContext(ctx, "terraform", "apply", "-auto-approve")
	cmd := exec.CommandContext(ctx, "echo", "hello", "hello")
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

// Cleanup removes the deployed resources using Terraform.
func (d *TerraformDeployer) Cleanup(ctx context.Context, out io.Writer) error {
	// cmd := exec.CommandContext(ctx, "terraform", "destroy", "-auto-approve")
	cmd := exec.CommandContext(ctx, "echo", "hello", "hello")
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
