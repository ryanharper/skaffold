package terraform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/kubernetes/manifest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	execCommand = exec.Command
	testTmpDir  string
	testCfg     latest.TerraformDeploy
)

func TestMain(m *testing.M) {
	// Setup
	var err error
	testTmpDir, err = os.MkdirTemp("", "skaffold-terraform-test")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	// Create a mock Terraform configuration file
	tfConfigContent := `
variable "key" {
  type    = string
  default = "default_value"
}

resource "null_resource" "example" {
  triggers = {
    key = var.key
  }

  provisioner "local-exec" {
    command = "echo ${var.key}"
  }
}

output "key_value" {
  value = var.key
}
`
	tfConfigPath := filepath.Join(testTmpDir, "main.tf")
	err = os.WriteFile(tfConfigPath, []byte(tfConfigContent), 0644)
	if err != nil {
		fmt.Printf("Failed to write Terraform config: %v\n", err)
		os.RemoveAll(testTmpDir)
		os.Exit(1)
	}

	testCfg = latest.TerraformDeploy{
		Deployments: []latest.TerrformDeployments{
			{
				Name:        "test-deployment",
				Dir:         testTmpDir,
				Vars:        map[string]string{"key": "test_value"},
				AutoApprove: true,
			},
		},
	}

	// Run tests
	code := m.Run()

	// Teardown
	os.RemoveAll(testTmpDir)

	os.Exit(code)
}

// Update the mock struct to implement manifest.ManifestList
type mockManifestList struct{}

func (m *mockManifestList) GetManifests() manifest.ManifestList {
	return manifest.ManifestList{}
}

func (m *mockManifestList) Append(manifest manifest.ManifestList) {
	// Do nothing for the mock
}

func (m *mockManifestList) String() string {
	return ""
}

// Helper function to create a mock ManifestListByConfig
func createMockManifestListByConfig() manifest.ManifestListByConfig {
	return manifest.ManifestListByConfig{
		//"test-config": &mockManifestList{},
	}
}

func TestNewDeployer(t *testing.T) {
	cfg := latest.TerraformDeploy{
		Deployments: []latest.TerrformDeployments{
			{Name: "test-deployment", Dir: "./test"},
		},
	}
	configName := "test-config"

	deployer, err := NewDeployer(cfg, configName)
	require.NoError(t, err)
	assert.Equal(t, configName, deployer.ConfigName())
	assert.Equal(t, &cfg, deployer.TerraformDeploy)
}

func TestDeploy(t *testing.T) {
	// Mock exec.Command
	execCommand = mockExecCommand
	defer func() { execCommand = exec.Command }()

	deployer, err := NewDeployer(testCfg, "test-config")
	require.NoError(t, err)

	ctx := context.Background()
	out := &bytes.Buffer{}

	mockLabellers := createMockManifestListByConfig()

	err = deployer.Deploy(ctx, out, nil, mockLabellers)

	assert.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "Creation complete")
}

func TestCleanup(t *testing.T) {
	// Mock exec.Command
	execCommand = mockExecCommand
	defer func() { execCommand = exec.Command }()

	deployer, err := NewDeployer(testCfg, "test-config")
	require.NoError(t, err)

	ctx := context.Background()
	out := &bytes.Buffer{}

	mockLabellers := createMockManifestListByConfig()

	err = deployer.Cleanup(ctx, out, false, mockLabellers)

	assert.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "Destroy complete")
}

// Add this function near the top of the file, after the imports
func setupDependencyTest() (string, error) {
	// Create a temporary directory for the dependency test
	tmpDir, err := os.MkdirTemp("", "skaffold-terraform-dependency-test")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}

	// Create subdirectories for each deployment
	dirs := []string{"dep1", "dep2", "dep3"}
	for _, dir := range dirs {
		err := os.Mkdir(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		// Create a simple main.tf file in each directory
		tfContent := fmt.Sprintf(`
resource "null_resource" "%s" {
  provisioner "local-exec" {
    command = "echo This is %s"
  }
}
`, dir, dir)
		err = os.WriteFile(filepath.Join(tmpDir, dir, "main.tf"), []byte(tfContent), 0644)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("failed to write main.tf for %s: %v", dir, err)
		}
	}

	return tmpDir, nil
}

// Update the TestDeployWithDependencies function
func TestDeployWithDependencies(t *testing.T) {
	// Set up the test environment
	tmpDir, err := setupDependencyTest()
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Mock exec.Command
	execCommand = mockExecCommand
	defer func() { execCommand = exec.Command }()

	cfg := latest.TerraformDeploy{
		Deployments: []latest.TerrformDeployments{
			{Name: "dep1", Dir: filepath.Join(tmpDir, "dep1"), AutoApprove: true},
			{Name: "dep2", Dir: filepath.Join(tmpDir, "dep2"), DependsOn: []string{"dep1"}, AutoApprove: true},
			{Name: "dep3", Dir: filepath.Join(tmpDir, "dep3"), DependsOn: []string{"dep2"}, AutoApprove: true},
		},
	}
	deployer, err := NewDeployer(cfg, "test-config")
	require.NoError(t, err)

	ctx := context.Background()
	out := &bytes.Buffer{}
	mockLabellers := createMockManifestListByConfig()
	err = deployer.Deploy(ctx, out, nil, mockLabellers)

	assert.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "Creation complete")
	assert.Contains(t, output, "-auto-approve")
	// Check if the order is correct
	dep1Index := bytes.Index(out.Bytes(), []byte("dep1"))
	dep2Index := bytes.Index(out.Bytes(), []byte("dep2"))
	dep3Index := bytes.Index(out.Bytes(), []byte("dep3"))
	assert.True(t, dep1Index < dep2Index && dep2Index < dep3Index)
}

func TestDeployCircularDependency(t *testing.T) {
	cfg := latest.TerraformDeploy{
		Deployments: []latest.TerrformDeployments{
			{Name: "dep1", Dir: "./dep1", DependsOn: []string{"dep2"}},
			{Name: "dep2", Dir: "./dep2", DependsOn: []string{"dep1"}},
		},
	}
	deployer, _ := NewDeployer(cfg, "test-config")

	ctx := context.Background()
	out := &bytes.Buffer{}
	mockLabellers := createMockManifestListByConfig()
	err := deployer.Deploy(ctx, out, nil, mockLabellers)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")
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
// This is called by the mock exec.Command to handle the command execution
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
	case "terraform":
		switch args[0] {
		case "init":
			fmt.Fprintf(os.Stdout, "Mocked terraform init in %s\n", args[len(args)-1])
		case "apply":
			autoApprove := false
			for _, arg := range args {
				if arg == "-auto-approve" {
					autoApprove = true
					break
				}
			}
			if !autoApprove {
				fmt.Fprintf(os.Stderr, "Expected -auto-approve flag for apply\n")
				os.Exit(2)
			}
			fmt.Fprintf(os.Stdout, "Mocked terraform apply -auto-approve in %s\n", args[len(args)-1])
			fmt.Fprintf(os.Stdout, "Creation complete for %s\n", filepath.Base(args[len(args)-1]))
		case "destroy":
			autoApprove := false
			for _, arg := range args {
				if arg == "-auto-approve" {
					autoApprove = true
					break
				}
			}
			if !autoApprove {
				fmt.Fprintf(os.Stderr, "Expected -auto-approve flag for destroy\n")
				os.Exit(2)
			}
			fmt.Fprintf(os.Stdout, "Mocked terraform destroy -auto-approve in %s\n", args[len(args)-1])
			fmt.Fprintf(os.Stdout, "Destroy complete for %s\n", filepath.Base(args[len(args)-1]))
		default:
			fmt.Fprintf(os.Stderr, "Unknown terraform command: %s\n", args[0])
			os.Exit(2)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(2)
	}
}
