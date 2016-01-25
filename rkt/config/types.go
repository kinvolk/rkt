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
	"net/http"
)

// Headerer is an interface for getting additional HTTP headers to use
// when downloading data (images, signatures).
type Headerer interface {
	Header() http.Header
}

type BasicCredentials struct {
	User     string
	Password string
}

type ConfigurablePaths struct {
	DataDir string
}

// Config is a single place where configuration for rkt frontend needs
// resides.
type Config struct {
	AuthPerHost                  map[string]Headerer
	DockerCredentialsPerRegistry map[string]BasicCredentials
	Paths                        ConfigurablePaths
}

func newConfig() *Config {
	return &Config{
		AuthPerHost:                  make(map[string]Headerer),
		DockerCredentialsPerRegistry: make(map[string]BasicCredentials),
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

type authV1JsonParser struct {
	parserBase
}

type dockerAuthV1JsonParser struct {
	parserBase
}

type pathsV1JsonParser struct {
	parserBase
}
