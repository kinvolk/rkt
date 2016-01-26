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
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/coreos/rkt/networking/types"
	baseconfig "github.com/coreos/rkt/pkg/config"
)

func (p* networksV1JsonParser) Parse(idx *baseconfig.PathIndex, raw []byte) error {
	type networksV1 struct {
		CniConfig *types.NetConf `json:"cniConfig"`
	}
	var networks networksV1
	if err := json.Unmarshal(raw, &networks); err != nil {
		return err
	}
	config := p.getConfig(idx.Index)
	if networks.CniConfig != nil {
		config.CniConfigs[filepath.Join(idx.Path, idx.Subdirectory, idx.Filename)] = networks.CniConfig
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
