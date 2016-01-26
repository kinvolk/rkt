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
	"github.com/coreos/rkt/networking/types"
)

type Paths struct {
	NetPluginsDirs []string
}

type Config {
	Paths      PluginsPaths
	CniConfigs map[string]*types.NetConf
}

func newConfig() *Config {
	return &Config{
		CniConfigs: make(map[string][]byte)
	}
}

type parserBase struct {
	configs []*Config
}

func (p *parserBase) getConfig(idx int) *Config {
	for len(p.configs) <= idx {
		p.configs = append(p.configs, newConfig())
	}
	return p.configs[idx]
}

type configPropagator interface {
	propagateConfig(config *Config)
}

// the parsers below implement both baseconfig.Parser and
// configPropagator interfaces

type pathsV1JsonParser struct {
	parserBase
}

type networksV1JsonParser struct {
	parserBase
}
