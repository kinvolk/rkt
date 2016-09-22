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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/coreos/rkt/common/cgroup"

	rktlog "github.com/coreos/rkt/pkg/log"
	stage1types "github.com/coreos/rkt/stage1/common/types"
	stage1initcommon "github.com/coreos/rkt/stage1/init/common"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/go-systemd/unit"
)

var (
	debug               bool
	disableCapabilities bool
	disablePaths        bool
	disableSeccomp      bool
	privateUsers        string
	log                 *rktlog.Logger
	diag                *rktlog.Logger
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.BoolVar(&disableCapabilities, "disable-capabilities-restriction", false, "Disable capability restrictions")
	flag.BoolVar(&disablePaths, "disable-paths", false, "Disable paths restrictions")
	flag.BoolVar(&disableSeccomp, "disable-seccomp", false, "Disable seccomp restrictions")
	flag.StringVar(&privateUsers, "private-users", "", "Run within user namespace. Can be set to [=UIDBASE[:NUIDS]]")
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

	enterCmd := []string{flag.Arg(2)}
	enterCmd = append(enterCmd, fmt.Sprintf("--pid=%s", flag.Arg(3)), "--")

	root := "."
	p, err := stage1types.LoadPod(root, uuid)
	if err != nil {
		log.PrintE("failed to load pod", err)
		os.Exit(1)
	}

	insecureOptions := stage1initcommon.Stage1InsecureOptions{
		DisablePaths:        disablePaths,
		DisableCapabilities: disableCapabilities,
		DisableSeccomp:      disableSeccomp,
	}

	ra := p.Manifest.Apps.Get(*appName)
	if ra == nil {
		log.Printf("failed to get app")
		os.Exit(1)
	}

	if ra.App.WorkingDirectory == "" {
		ra.App.WorkingDirectory = "/"
	}

	/* prepare cgroups */
	enabledCgroups, err := cgroup.GetEnabledV1Cgroups()
	if err != nil {
		log.FatalE("error getting cgroups", err)
		os.Exit(1)
	}
	b, err := ioutil.ReadFile(filepath.Join(p.Root, "subcgroup"))
	if err == nil {
		subcgroup := string(b)
		serviceName := stage1initcommon.ServiceUnitName(ra.Name)

		if err := cgroup.RemountCgroupKnobsRW(enabledCgroups, subcgroup, serviceName, enterCmd); err != nil {
			log.FatalE("error restricting container cgroups", err)
			os.Exit(1)
		}
	} else {
		log.PrintE("continuing with per-app isolators disabled", err)
	}

	/* prepare mounts */
	/* TODO(alban): use GenerateMounts, see stage1/init/common/pod.go:appToNspawnArgs */
	mountSlave, err := ioutil.TempDir("", "rkt.propagate.")
	if err != nil {
		log.FatalE("error creating temporary propagation directory", err)
		os.Exit(1)
	}
	defer os.Remove(mountSlave)

	err = syscall.Mount(mountSlave, mountSlave, "bind", syscall.MS_BIND, "")
	if err != nil {
		log.FatalE("error mounting temporary directory", err)
		os.Exit(1)
	}
	defer syscall.Unmount(mountSlave, 0)

	err = syscall.Mount("", mountSlave, "none", syscall.MS_SLAVE, "")
	if err != nil {
		log.FatalE("error mounting temporary directory", err)
		os.Exit(1)
	}

	mountTmp := filepath.Join(mountSlave, "mount")
	if err := os.MkdirAll(mountTmp, 0700); err != nil {
		log.FatalE("error creating temporary mount directory", err)
		os.Exit(1)
	}
	defer os.Remove(mountTmp)

	err = syscall.Mount("/home/alban/tmp", mountTmp, "bind", syscall.MS_BIND, "")
	if err != nil {
		log.FatalE("error mounting temporary directory", err)
		os.Exit(1)
	}
	defer syscall.Unmount(mountTmp, 0)

	readonly := true
	if readonly {
		err = syscall.Mount("", mountTmp, "bind", syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_BIND, "")
		if err != nil {
			log.FatalE("error remounting temporary mount directory read-only", err)
			os.Exit(1)
		}
	}

	mountOutside := filepath.Join("/run/systemd/nspawn/propagate/", "rkt-"+p.UUID.String(), "rkt.mount")
	mountInside := filepath.Join("/run/systemd/nspawn/incoming/", filepath.Base(mountOutside))
	mountDst := "/opt/stage2/alpine-sh/rootfs/newmnt"
	mountDstOutside := filepath.Join(p.Root, "stage1/rootfs", mountDst)
	if err := os.MkdirAll(mountOutside, 0700); err != nil {
		log.FatalE("error creating temporary mount directory", err)
		os.Exit(1)
	}
	defer os.Remove(mountOutside)

	if err := os.MkdirAll(mountDstOutside, 0700); err != nil {
		log.FatalE("error creating destination directory", err)
		os.Exit(1)
	}

	err = syscall.Mount(mountTmp, mountOutside, "", syscall.MS_MOVE, "")
	if err != nil {
		log.FatalE("error moving mount directory", err)
		os.Exit(1)
	}

	args := enterCmd
	args = append(args, "/bin/mount", "--move", mountInside, mountDst)

	cmd := exec.Cmd{
		Path: args[0],
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		log.PrintE("error executing mount move", err)
		os.Exit(1)
	}

	/* write service file */
	binPath, err := stage1initcommon.FindBinPath(p, ra)
	if err != nil {
		log.PrintE("failed to find bin path", err)
		os.Exit(1)
	}

	w := stage1initcommon.NewUnitWriter(p)

	w.AppUnit(ra, binPath, privateUsers, insecureOptions,
		unit.NewUnitOption("Unit", "Before", "halt.target"),
		unit.NewUnitOption("Unit", "Conflicts", "halt.target"),
		unit.NewUnitOption("Service", "StandardOutput", "journal+console"),
		unit.NewUnitOption("Service", "StandardError", "journal+console"),
	)

	w.AppReaperUnit(ra.Name, binPath)

	if err := w.Error(); err != nil {
		log.PrintE("error generating app units", err)
		os.Exit(1)
	}

	args = enterCmd
	args = append(args, "/usr/bin/systemctl")
	args = append(args, "daemon-reload")

	cmd = exec.Cmd{
		Path: args[0],
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		log.PrintE("error executing daemon-reload", err)
		os.Exit(1)
	}

	args = enterCmd
	args = append(args, "/usr/bin/systemctl")
	args = append(args, "start")
	args = append(args, appName.String())

	cmd = exec.Cmd{
		Path: args[0],
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		log.PrintE(fmt.Sprintf("error starting app %q", appName.String()), err)
		os.Exit(1)
	}

	// TODO unmount all the volumes

	os.Exit(0)
}
