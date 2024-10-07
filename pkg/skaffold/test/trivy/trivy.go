package trivy

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
	trivyTests []*latest.TrivyTest
}

func New(cfg docker.Config, trivyTests []*latest.TrivyTest) *Runner {
	return &Runner{
		cfg:        cfg,
		trivyTests: trivyTests,
	}
}

func (r *Runner) Test(ctx context.Context, out io.Writer, imageTag string) error {
	for _, test := range r.trivyTests {
		args := []string{"image", "--severity", test.Severity, imageTag}
		cmd := exec.CommandContext(ctx, "trivy", args...)
		cmd.Stdout = out
		cmd.Stderr = out

		if err := util.RunCmd(ctx, cmd); err != nil {
			return fmt.Errorf("trivy test failed: %w", err)
		}
	}
	return nil
}

func (r *Runner) TestDependencies(ctx context.Context) ([]string, error) {
	return nil, nil
}
