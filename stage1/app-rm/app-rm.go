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

//+build linux

package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coreos/rkt/common"
	rktlog "github.com/coreos/rkt/pkg/log"
	stage1types "github.com/coreos/rkt/stage1/common/types"
	stage1initcommon "github.com/coreos/rkt/stage1/init/common"

	"github.com/appc/spec/schema/types"
)

var (
	debug bool
	log   *rktlog.Logger
	diag  *rktlog.Logger
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
}

// TODO use named flags instead of positional
func main() {
	flag.Parse()

	stage1initcommon.InitDebug(debug)

	log, diag, _ = rktlog.NewLogSet("stage1", debug)
	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	uuid, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		log.PrintE("UUID is missing or malformed", err)
		os.Exit(1)
	}

	appName, err := types.NewACName(flag.Arg(1))
	if err != nil {
		log.PrintE("invalid app name", err)
		os.Exit(1)
	}

	root := "."
	_, err = stage1types.LoadPod(root, uuid)
	if err != nil {
		log.PrintE("failed to load pod", err)
		os.Exit(1)
	}

	// TODO exec "systemctl is-active appName.String()"
	// if active, bail out

	s1rootfs := common.Stage1RootfsPath(".")
	serviceDir := filepath.Join(s1rootfs, "usr", "lib", "systemd", "system")
	appServicePaths := []string{
		filepath.Join(serviceDir, appName.String()+".service"),
		filepath.Join(serviceDir, "reaper-"+appName.String()+".service"),
	}

	for _, p := range appServicePaths {
		if err := os.Remove(p); err != nil {
			log.PrintE("error removing app service file", err)
			os.Exit(1)
		}
	}

	// TODO exec "systemctl daemon-reload"

	// TODO unmount all the volumes

	os.Exit(0)
}
