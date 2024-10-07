package packer

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/docker"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/platform"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/util"
)

type Builder struct {
	cfg         docker.Config
	localPacker docker.LocalDaemon
}

func NewBuilder(cfg docker.Config, localPacker docker.LocalDaemon) *Builder {
	return &Builder{
		cfg:         cfg,
		localPacker: localPacker,
	}
}

func (b *Builder) Build(ctx context.Context, out io.Writer, artifact *latest.Artifact, tag string, platforms platform.Matcher) (string, error) {
	if artifact.PackerArtifact == nil {
		return "", fmt.Errorf("packer artifact is nil")
	}

	// Run packer init before building
	if err := b.PackerInit(ctx, out, artifact); err != nil {
		return "", fmt.Errorf("packer init failed: %w", err)
	}

	args := []string{"build"}
	args = append(args, artifact.PackerArtifact.BuildArgs...)
	args = append(args, "-var", fmt.Sprintf("image_name=%s", artifact.ImageName))
	args = append(args, "-var", fmt.Sprintf("image_tag=%s", tag))
	// if len(artifact.PackerArtifact.PostProcessors) > 0 {
	// 	args = append(args, "-only", fmt.Sprintf("'%s'", artifact.PackerArtifact.PostProcessors))
	// }

	args = append(args, artifact.PackerArtifact.TemplatePath)

	cmd := exec.CommandContext(ctx, "packer", args...)
	cmd.Env = append(util.OSEnviron(), artifact.PackerArtifact.Env...)
	cmd.Dir = b.getContextDir(artifact)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := util.RunCmd(ctx, cmd); err != nil {
		return "", fmt.Errorf("packer build failed: %w", err)
	}

	// TODO: Implement platform-specific logic if needed
	// For now, we're ignoring the platforms parameter

	return b.localPacker.ImageID(ctx, tag)
}

func (b *Builder) PackerInit(ctx context.Context, out io.Writer, a *latest.Artifact) error {
	if a.PackerArtifact == nil {
		return fmt.Errorf("packer artifact is nil")
	}

	args := []string{"init", a.PackerArtifact.TemplatePath}

	cmd := exec.CommandContext(ctx, "packer", args...)
	cmd.Env = append(util.OSEnviron(), a.PackerArtifact.Env...)
	cmd.Dir = b.getContextDir(a)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := util.RunCmd(ctx, cmd); err != nil {
		return fmt.Errorf("packer init failed: %w", err)
	}

	return nil
}

func (b *Builder) SupportedPlatforms() platform.Matcher {
	// TODO: Implement proper platform support for Packer
	// For now, we're returning platform.All, which might not be accurate for all cases
	return platform.All
}

func (b *Builder) getContextDir(artifact *latest.Artifact) string {
	if artifact.Workspace != "" {
		return artifact.Workspace
	}
	return filepath.Dir(artifact.PackerArtifact.TemplatePath)
}
