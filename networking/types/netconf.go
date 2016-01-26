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

package types

import (
	cnitypes "github.com/appc/cni/pkg/types"
)

// NetConf local struct extends cnitypes.NetConf with information about masquerading
// similar to CNI plugins
type NetConf struct {
	cnitypes.NetConf
	IPMasq bool `json:"ipMasq"`
	MTU    int  `json:"mtu"`
}
