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

package inspect

import (
	"context"
	"io"

	"github.com/ryanharper/skaffold/v2/pkg/skaffold/config"
	"github.com/ryanharper/skaffold/v2/pkg/skaffold/inspect"
)

type profileList struct {
	Profiles []profileEntry `json:"profiles"`
}

type profileEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Module string `json:"module,omitempty"`
}

func PrintProfilesList(ctx context.Context, out io.Writer, opts inspect.Options) error {
	formatter := inspect.OutputFormatter(out, opts.OutFormat)
	cfgs, err := inspect.GetConfigSet(ctx, config.SkaffoldOptions{ConfigurationFile: opts.Filename, ConfigurationFilter: opts.Modules, RemoteCacheDir: opts.RemoteCacheDir})
	if err != nil {
		formatter.WriteErr(err)
		return err
	}

	l := &profileList{Profiles: []profileEntry{}}
	for _, c := range cfgs {
		for _, p := range c.Profiles {
			if opts.BuildEnv != inspect.BuildEnvs.Unspecified && inspect.GetBuildEnv(&p.Build.BuildType) != opts.BuildEnv {
				continue
			}
			l.Profiles = append(l.Profiles, profileEntry{Name: p.Name, Path: c.SourceFile, Module: c.Metadata.Name})
		}
	}
	return formatter.Write(l)
}
