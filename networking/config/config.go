// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"github.com/coreos/rkt/common"
	baseconfig "github.com/coreos/rkt/pkg/config"
)

type configSetup struct {
	directory   *baseconfig.Directory
	propagators []configPropagator
}

func GetConfigFrom(dirs ...string) (*Config, error) {
	setup, err := getConfigSetup()
	if err != nil {
		return nil, err
	}
	if err := setup.directory.WalkDirectories(dirs...); err != nil {
		return nil, err
	}
	cfg := newConfig()
	setup.propagateChanges(cfg)
	return cfg, nil
}

func getConfigSetup() (*configSetup, error) {
	d := common.NewConfigDirectory(common.Stage1CDBN)
	pathsV1 := &pathsV1JsonParser{}
	networksV1 := &networksV1JsonParser{}
	parsers := []*baseconfig.ParserSetup{
		{
			Kind:    "paths",
			Version: "v1",
			Parser:  pathsV1,
		},
		{
			Kind:    "networks",
			Version: "v1",
			Parser:  networksV1,
		},
	}
	subdirs := []*baseconfig.SubdirSetup{
		{
			Subdir: "paths.d",
			Kinds:  []string{"paths"},
		},
		{
			Subdir: "net.d",
			Kinds:  []string{"networks"},
		},
	}
	if err := d.RegisterParsers(parsers); err != nil {
		return nil, err
	}
	if err := d.RegisterSubdirectories(subdirs); err != nil {
		return nil, err
	}
	setup := &configSetup{
		directory: d,
		propagators: []configPropagator{
			pathsV1,
			networksV1,
		},
	}
	return setup, nil
}

func (s *configSetup) propagateChanges(cfg *Config) {
	for _, p := range s.propagators {
		p.propagateConfig(cfg)
	}
}
