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

package common

import (
	"encoding/json"
	"errors"

	"github.com/coreos/rkt/pkg/config"
)

func NewConfigDirectory(cdbn string) *config.Directory {
	return config.NewDirectory(cdbn, &rktJSON{})
}

type rktJSONConfigHeader struct {
	RktVersion string `json:"rktVersion"`
	RktKind    string `json:"rktKind"`
}

// implements github.com/coreos/rkt/pkg/config Type
type rktJSON struct{}

func (*rktJSON) Extension() string {
	return "json"
}

func (*rktJSON) GetKindAndVersion(raw []byte) (string, string, error) {
	var header rktJSONConfigHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return "", "", err
	}
	if len(header.RktKind) == 0 {
		return "", "", errors.New("no rktKind specified")
	}
	if len(header.RktVersion) == 0 {
		return "", "", errors.New("no rktVersion specified")
	}
	return header.RktKind, header.RktVersion, nil
}
