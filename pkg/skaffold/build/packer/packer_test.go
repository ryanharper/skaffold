package packer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/docker"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/platform"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/stretchr/testify/assert"
)

var (
	execCommand = exec.Command
	testTmpDir  string
	testCfg     latest.PackerArtifact
)

func TestMain(m *testing.M) {
	// Setup
	var err error
	testTmpDir, err = os.MkdirTemp("", "skaffold-packer-test")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	// Create a dummy Packer template file
	dummyTemplate := `
source "null" "example" {
  communicator = "none"
}

build {
  sources = ["source.null.example"]

  provisioner "shell-local" {
    inline = ["echo 'Hello, Packer!'"]
  }
}

variable "image_name" {
  type = string
  default = "test"
}
`
	templatePath := filepath.Join(testTmpDir, "template.pkr.hcl")
	err = os.WriteFile(templatePath, []byte(dummyTemplate), 0644)
	if err != nil {
		fmt.Printf("Failed to write Packer template: %v\n", err)
		os.RemoveAll(testTmpDir)
		os.Exit(1)
	}

	testCfg = latest.PackerArtifact{
		TemplatePath: templatePath,
	}

	// Run tests
	code := m.Run()

	// Teardown
	os.RemoveAll(testTmpDir)

	os.Exit(code)
}

func TestPackerBuild(t *testing.T) {
	// Mock exec.Command
	execCommand = mockExecCommand
	defer func() { execCommand = exec.Command }()

	artifact := &latest.Artifact{
		ImageName: "test-image",
		Workspace: testTmpDir,
		ArtifactType: latest.ArtifactType{
			PackerArtifact: &testCfg,
		},
	}

	cfg := &mockConfig{}
	localDaemon := &mockLocalDaemon{}
	b := NewBuilder(cfg, localDaemon)

	ctx := context.Background()
	out := &bytes.Buffer{}

	b.Build(ctx, out, artifact, "test-image:latest", platform.Matcher{})

	// assert.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "required_plugins block")

}

func TestPackerInit(t *testing.T) {
	// Mock exec.Command
	execCommand = mockExecCommand
	defer func() { execCommand = exec.Command }()

	artifact := &latest.Artifact{
		ArtifactType: latest.ArtifactType{
			PackerArtifact: &testCfg,
		},
	}

	cfg := &mockConfig{}
	localDaemon := &mockLocalDaemon{}
	b := NewBuilder(cfg, localDaemon)

	ctx := context.Background()
	out := &bytes.Buffer{}

	err := b.PackerInit(ctx, out, artifact)

	assert.NoError(t, err)
	assert.Contains(t, out.String(), "No plugins requirement found")
}

// Mock exec.Command
func mockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess isn't a real test. It's used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for i, val := range args {
		if val == "--" {
			args = args[i+1:]
			break
		}
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "packer":
		switch args[0] {
		case "init":
			fmt.Fprintf(os.Stdout, "Mocked packer init in %s\n", args[len(args)-1])
		case "build":
			fmt.Fprintf(os.Stdout, "Mocked packer build in %s\n", args[len(args)-1])
			fmt.Fprintf(os.Stdout, "Hello, Packer!\n")
		default:
			fmt.Fprintf(os.Stderr, "Unknown packer command: %s\n", args[0])
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(2)
	}
}

type mockConfig struct {
	docker.Config
}

type mockLocalDaemon struct {
	docker.LocalDaemon
	built []struct {
		tag string
	}
}

func (m *mockLocalDaemon) ImageID(ctx context.Context, ref string) (string, error) {
	return "image-id", nil
}

func (m *mockLocalDaemon) Build(ctx context.Context, out io.Writer, workspace string, artifact string, dockerArtifact *latest.DockerArtifact, opts docker.BuildOptions) (string, error) {
	m.built = append(m.built, struct {
		tag string
	}{
		tag: opts.Tag,
	})
	return "image-id", nil
}
