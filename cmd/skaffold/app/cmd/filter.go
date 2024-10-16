/*
Copyright 2020 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	apim "k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/ryanharper/skaffold/v2/cmd/skaffold/app/flags"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/config"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/debug"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/graph"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/kubernetes/debugging"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/kubernetes/manifest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/render/applysetters"
	rUtil "github.com/ryanharper/skaffold/v2/pkg/skaffold/render/renderer/util"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/runner"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/latest"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/util"
	pkgutil "github.com/ryanharper/skaffold/v2/pkg/skaffold/util"
)

// for tests
var doFilter = runFilter

// NewCmdFilter describes the CLI command to filter and transform a set of Kubernetes manifests.
func NewCmdFilter() *cobra.Command {
	var debuggingFilters bool
	var renderFromBuildOutputFile flags.BuildOutputFileFlag
	var postRenderer string
	return NewCmd("filter").
		Hidden(). // internal command
		WithDescription("Filter and transform a set of Kubernetes manifests from stdin").
		WithLongDescription("Unlike `render`, this command does not build artifacts.").
		WithCommonFlags().
		WithFlags([]*Flag{
			{Value: &renderFromBuildOutputFile, Name: "build-artifacts", Shorthand: "a", Usage: "File containing build result from a previous 'skaffold build --file-output'"},
			{Value: &debuggingFilters, Name: "debugging", DefValue: false, Usage: `Apply debug transforms similar to "skaffold debug"`, IsEnum: true},
			{Value: &debug.Protocols, Name: "protocols", DefValue: []string{}, Usage: "Priority sorted order of debugger protocols to support."},
			{Value: &postRenderer, Name: "post-renderer", DefValue: "", FlagAddMethod: "StringVar", Usage: "Any executable that accepts rendered Kubernetes manifests on STDIN and returns valid Kubernetes manifests on STDOUT"},
		}).
		NoArgs(func(ctx context.Context, out io.Writer) error {
			return doFilter(ctx, out, debuggingFilters, postRenderer, renderFromBuildOutputFile.BuildArtifacts())
		})
}

// runFilter loads the Kubernetes manifests from stdin and applies the debug transformations.
// Unlike `skaffold debug`, this filtering affects all images and not just the built artifacts.
func runFilter(ctx context.Context, out io.Writer, debuggingFilters bool, postRenderer string, buildArtifacts []graph.Artifact) error {
	return withRunner(ctx, out, func(r runner.Runner, configs []util.VersionedConfig) error {
		var manifestList manifest.ManifestList
		var err error
		if postRenderer != "" {
			cmd := exec.CommandContext(ctx, postRenderer)
			cmd.Stdin = os.Stdin
			stdoutPipe, err := cmd.StdoutPipe()
			if err != nil {
				return fmt.Errorf("running post-renderer: %w", err)
			}
			err = cmd.Start()
			if err != nil {
				return fmt.Errorf("running post-renderer: %w", err)
			}

			manifestList, err = manifest.Load(stdoutPipe)
			if err != nil {
				return fmt.Errorf("loading post-renderer result: %w", err)
			}
			stdoutPipe.Close()
		} else {
			manifestList, err = manifest.Load(os.Stdin)
		}
		if err != nil {
			return fmt.Errorf("loading manifests: %w", err)
		}
		var ass applysetters.ApplySetters
		manifestOverrides := pkgutil.EnvSliceToMap(opts.ManifestsOverrides, "=")
		for k, v := range manifestOverrides {
			ass.Setters = append(ass.Setters, applysetters.Setter{Name: k, Value: v})
		}
		manifestList, err = ass.Apply(ctx, manifestList)
		if err != nil {
			return err
		}
		allow, deny := getTransformList(configs)

		manifestList, err = manifestList.SetLabels(pkgutil.EnvSliceToMap(opts.CustomLabels, "="),
			manifest.NewResourceSelectorLabels(allow, deny))
		if err != nil {
			return err
		}
		manifestList, err = manifestList.ReplaceImages(ctx, buildArtifacts, manifest.NewResourceSelectorImages(allow, deny))
		if err != nil {
			return err
		}

		if debuggingFilters {
			// TODO(bdealwis): refactor this code
			debugHelpersRegistry, err := config.GetDebugHelpersRegistry(opts.GlobalConfig)
			if err != nil {
				return fmt.Errorf("resolving debug helpers: %w", err)
			}
			insecureRegistries, err := getInsecureRegistries(opts, configs)
			if err != nil {
				return fmt.Errorf("retrieving insecure registries: %w", err)
			}

			manifestList, err = debugging.ApplyDebuggingTransforms(manifestList, buildArtifacts, manifest.Registries{
				DebugHelpersRegistry: debugHelpersRegistry,
				InsecureRegistries:   insecureRegistries,
			})
			if err != nil {
				return fmt.Errorf("transforming manifests: %w", err)
			}
		}
		out.Write([]byte(manifestList.String()))
		return nil
	})
}

func getTransformList(configs []util.VersionedConfig) (map[apim.GroupKind]latest.ResourceFilter, map[apim.GroupKind]latest.ResourceFilter) {
	// TODO: remove code duplication by adding a new Filter method to the runner.
	// and reuse renderer/util.ConsolidateTransformConfiguration

	allow := manifest.TransformAllowlist
	deny := manifest.TransformDenylist

	// add default values
	for _, rf := range manifest.TransformAllowlist {
		groupKind := apim.ParseGroupKind(rf.GroupKind)
		allow[groupKind] = rUtil.ConvertJSONPathIndex(rf)
	}
	for _, rf := range manifest.TransformDenylist {
		groupKind := apim.ParseGroupKind(rf.GroupKind)
		deny[groupKind] = rUtil.ConvertJSONPathIndex(rf)
	}

	for _, cfg := range configs {
		for _, rf := range cfg.(*latest.SkaffoldConfig).ResourceSelector.Allow {
			groupKind := apim.ParseGroupKind(rf.GroupKind)
			allow[groupKind] = rUtil.ConvertJSONPathIndex(rf)
		}
		for _, rf := range cfg.(*latest.SkaffoldConfig).ResourceSelector.Deny {
			groupKind := apim.ParseGroupKind(rf.GroupKind)
			deny[groupKind] = rUtil.ConvertJSONPathIndex(rf)
		}
	}
	return allow, deny
}

func getInsecureRegistries(opts config.SkaffoldOptions, configs []util.VersionedConfig) (map[string]bool, error) {
	cfgRegistries, err := config.GetInsecureRegistries(opts.GlobalConfig)
	if err != nil {
		return nil, err
	}
	var regList []string

	regList = append(regList, opts.InsecureRegistries...)
	for _, cfg := range configs {
		regList = append(regList, cfg.(*latest.SkaffoldConfig).Build.InsecureRegistries...)
	}
	regList = append(regList, cfgRegistries...)
	insecureRegistries := make(map[string]bool, len(regList))
	for _, r := range regList {
		insecureRegistries[r] = true
	}
	return insecureRegistries, nil
}
