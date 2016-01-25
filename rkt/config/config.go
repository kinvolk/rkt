// Copyright 2015 The rkt Authors
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
	"net/http"

	"github.com/coreos/rkt/common"
	baseconfig "github.com/coreos/rkt/pkg/config"
)

type configSetup struct {
	directory   *baseconfig.Directory
	propagators []configPropagator
}

const (
	ConfigurationDirectoryBaseName = "cfg"
)

// ResolveAuthPerHost takes a map of strings to Headerer and resolves the
// Headerers to http.Headers
func ResolveAuthPerHost(authPerHost map[string]Headerer) map[string]http.Header {
	hostHeaders := make(map[string]http.Header, len(authPerHost))
	for k, v := range authPerHost {
		hostHeaders[k] = v.Header()
	}
	return hostHeaders
}

// GetConfigFrom gets the Config instance with configuration taken
// from given paths. Subsequent paths override settings from the
// previous paths.
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
	d := common.NewConfigDirectory(ConfigurationDirectoryBaseName)
	authV1 := &authV1JsonParser{}
	dockerAuthV1 := &dockerAuthV1JsonParser{}
	pathsV1 := &pathsV1JsonParser{}
	parsers := []*baseconfig.ParserSetup{
		{
			Kind:    "auth",
			Version: "v1",
			Parser:  authV1,
		},
		{
			Kind:    "dockerAuth",
			Version: "v1",
			Parser:  dockerAuthV1,
		},
		{
			Kind:    "paths",
			Version: "v1",
			Parser:  pathsV1,
		},
	}
	subdirs := []*baseconfig.SubdirSetup{
		{
			Subdir: "auth.d",
			Kinds:  []string{"auth", "dockerAuth"},
		},
		{
			Subdir: "paths.d",
			Kinds:  []string{"paths"},
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
			authV1,
			dockerAuthV1,
			pathsV1,
		},
	}
	return setup, nil
}

func (s *configSetup) propagateChanges(cfg *Config) {
	for _, p := range s.propagators {
		p.propagateConfig(cfg)
	}
}
