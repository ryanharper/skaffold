/*
Copyright 2021 The Skaffold Authors

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

package runner

import (
	"context"
	"io"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/kubernetes/manifest"
)

func (r *SkaffoldRunner) Cleanup(ctx context.Context, out io.Writer, dryRun bool, manifestListByConfig manifest.ManifestListByConfig, command string) error {
	var err error
	if command == "verify" {
		err = r.verifier.Cleanup(ctx, out, dryRun)
		if err != nil {
			return err
		}
		return nil
	}
	err = r.deployer.Cleanup(ctx, out, dryRun, manifestListByConfig)
	if err != nil {
		return err
	}
	return nil
}
