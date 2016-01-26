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
	"errors"
	"path/filepath"

	baseconfig "github.com/coreos/rkt/pkg/config"
)

func (p* pathsV1JsonParser) Parse(idx *baseconfig.PathIndex, raw []byte) error {
	type pathsV1 struct {
		NetPlugins []string `json:"netPlugins"`
	}
	var paths pathsV1
	if err := json.Unmarshal(raw, &paths); err != nil {
		return err
	}
	config := p.getConfig(idx.Index)
	if len(paths.NetPlugins) > 0 {
		if len(config.Paths.NetPlugins) > 0 {
			return errors.New("network plugins paths are already specified")
		}
		for _, p := paths.NetPlugins {
			if !filepath.IsAbs() {
				return errors.New("network plugin path must be an absolute path")
			}
		}
		config.Paths.NetPlugins = paths.NetPlugins
	}
	return nil
}

func (p *pathsV1JsonParser) propagateConfig(config *Config) {
	for _, subconfig := range p.configs {
		if len(subconfig.Paths.NetPlugins) > 0 {
			config.Paths.NetPlugins = subconfig.Paths.NetPlugins
		}
	}
}
