package grype

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/docker"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/util"
)

type Runner struct {
	cfg        docker.Config
	grypeTests []*latest.GrypeTest
	imageName  string
	workspace  string
}

func New(cfg docker.Config, imageName string, ws string, grypeTests []*latest.GrypeTest) (*Runner, error) {
	return &Runner{
		cfg:        cfg,
		imageName:  imageName,
		grypeTests: grypeTests,
		workspace:  ws,
	}, nil
}

func (r *Runner) Test(ctx context.Context, out io.Writer, imageTag string) error {
	for _, test := range r.grypeTests {
		fmt.Printf("grype test failed: %v", imageTag)
		args := []string{imageTag, "--fail-on", test.Severity}
		cmd := exec.CommandContext(ctx, "grype", args...)
		cmd.Stdout = out
		cmd.Stderr = out

		if err := util.RunCmd(ctx, cmd); err != nil {
			return fmt.Errorf("grype test failed: %w", err)
		}
	}
	return nil
}

func (r *Runner) TestDependencies(ctx context.Context) ([]string, error) {
	return nil, nil
}
