/*
Copyright 2019 The Skaffold Authors

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

package v1beta3

import (
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/util"
	next "github.com/ryanharper/skaffold/v2/pkg/skaffold/schema/v1beta4"
	pkgutil "github.com/ryanharper/skaffold/v2/pkg/skaffold/util"
)

// Upgrade upgrades a configuration to the next version.
// Config changes from v1beta3 to v1beta4
// 1. Additions:
// helm skipBuildDependencies
// profile patches
// profile activation
// 2. No removals
// 3. No updates
func (c *SkaffoldConfig) Upgrade() (util.VersionedConfig, error) {
	var newConfig next.SkaffoldConfig

	pkgutil.CloneThroughJSON(c, &newConfig)
	newConfig.APIVersion = next.Version

	return &newConfig, nil
}
