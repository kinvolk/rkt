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

package testutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

// DirDesc structure manages one directory and provides an option for
// rkt invocations
type DirDesc struct {
	t      *testing.T
	dir    string // directory path
	desc   string // directory description, mostly for failure cases
	prefix string // temporary directory prefix
	option string // rkt option for given directory
}

// RktOption returns option for rkt invocation
func (d *DirDesc) RktOption() string {
	d.ensureValid()
	return fmt.Sprintf("--%s=%s", d.option, d.dir)
}

// Path returns a path, duh.
func (d *DirDesc) Path() string {
	return d.dir
}

// RktRunCtx manages various rkt-specific directories - recreates them
// and ensures that they are properly cleaned up.
type RktRunCtx struct {
	t           *testing.T
	directories []*DirDesc
	mds         *exec.Cmd
	children    []*gexpect.ExpectSubprocess
}

// NewRktRunCtx creates a new context for running rkt
func NewRktRunCtx(t *testing.T) *RktRunCtx {
	return &RktRunCtx{
		t:           t,
		directories: getCtxDirectories(t),
	}
}

// T returns testing.T instance
func (ctx *RktRunCtx) T() *testing.T {
	return ctx.t
}

// DataDir returns a data directory description
func (ctx *RktRunCtx) DataDir() *DirDesc {
	return ctx.dir(0)
}

// LocalDir returns a local configuration directory description
func (ctx *RktRunCtx) LocalDir() *DirDesc {
	return ctx.dir(1)
}

// SystemDir returns a system configuration directory description
func (ctx *RktRunCtx) SystemDir() *DirDesc {
	return ctx.dir(2)
}

// LaunchMDS starts metadata service, it will be shutdown when ctx is
// cleaned up.
func (ctx *RktRunCtx) LaunchMDS() error {
	ctx.mds = exec.Command(ctx.RktBin(), "metadata-service")
	return ctx.mds.Start()
}

// Reset kills all the registered processes, GC's the pods, cleans up
// all the directories it manages and recreates them anew.
func (ctx *RktRunCtx) Reset() {
	ctx.commonCleanup()
	for _, d := range ctx.directories {
		d.reset()
	}
}

// Cleanup shuts down the metadata service if it was running (see
// LaunchMDS()), kills all the registered processes, GC's the pods and
// cleans up all the directories it manages. After this function, the
// ctx cannot be used, unless Reset() is called on it again. Note that
// Reset() will not rerun metadata-service, you will have to do it on
// your own.
func (ctx *RktRunCtx) Cleanup() {
	if ctx.mds != nil {
		ctx.mds.Process.Kill()
		ctx.mds.Wait()
		os.Remove("/run/rkt/metadata-svc.sock")
	}
	ctx.commonCleanup()
	for _, d := range ctx.directories {
		d.cleanup()
	}
}

// Cmd returns a rkt invocation command with all flags for setting
// directory paths.
func (ctx *RktRunCtx) Cmd() string {
	return fmt.Sprintf("%s %s",
		ctx.RktBin(),
		strings.Join(ctx.rktOptions(), " "),
	)
}

// RktBin returns a path to rkt binary
func (ctx *RktRunCtx) RktBin() string {
	// TODO(krnowak): port to getValueFromEnvOrPanic, when it is
	// moved over to testutils
	rkt := os.Getenv("RKT")
	if rkt == "" {
		ctx.t.Fatal("Empty RKT environment variable")
	}
	if abs, err := filepath.Abs(rkt); err == nil {
		return abs
	}
	return rkt
}

// RegisterChild registers a process to be cleaned up on Reset or
// Cleanup before the directories are removed.
func (ctx *RktRunCtx) RegisterChild(child *gexpect.ExpectSubprocess) {
	ctx.children = append(ctx.children, child)
}

// RunGC tries to run a GC.
func (ctx *RktRunCtx) RunGC() {
	rktArgs := append(ctx.rktOptions(),
		"gc",
		"--grace-period=0s",
	)
	if err := exec.Command(ctx.RktBin(), rktArgs...).Run(); err != nil {
		ctx.t.Fatalf("Failed to run gc: %v", err)
	}
}

// DirDesc private API

// newDirDesc creates DirDesc instance managing a temporary directory.
func newDirDesc(t *testing.T, prefix, desc, option string) *DirDesc {
	dir := &DirDesc{
		t:      t,
		dir:    "",
		desc:   desc,
		prefix: prefix,
		option: option,
	}
	dir.reset()
	return dir
}

// reset removes the managed directory and recreates it
func (d *DirDesc) reset() {
	d.cleanup()
	dir, err := ioutil.TempDir("", d.prefix)
	if err != nil {
		d.t.Fatalf("Failed to create temporary %s directory: %v", d.desc, err)
	}
	d.dir = dir
}

// cleanup removes the managed directory. After cleanup this instance
// cannot be used for anything, until it is reset.
func (d *DirDesc) cleanup() {
	if d.dir == "" {
		return
	}
	if err := os.RemoveAll(d.dir); err != nil && !os.IsNotExist(err) {
		d.t.Fatalf("Failed to remove temporary %s directory %q: %s", d.desc, d.dir, err)
	}
	d.dir = ""
}

// ensureValid makes sure that the directory description has a backing
// directory on filesystem set up.
func (d *DirDesc) ensureValid() {
	if d.dir == "" {
		d.t.Fatalf("A temporary %s directory is not set up", d.desc)
	}
}

// RktRunCtx private API

// commonCleanup tries to stop all the registered children and run the
// GC.
func (ctx *RktRunCtx) commonCleanup() {
	ctx.cleanupChildren()
	ctx.RunGC()
}

// getCtxDirectories gets data, local config and system config
// directories for rkt run context. It makes sure that if any
// directory description fails to set tup then other previously
// created will be cleaned up.
func getCtxDirectories(t *testing.T) []*DirDesc {
	dirs := make([]*DirDesc, 3)
	defer func() {
		for _, d := range dirs {
			if d != nil {
				d.cleanup()
			}
		}
	}()
	dirs[0] = newDirDesc(t, "datadir-", "data", "dir")
	dirs[1] = newDirDesc(t, "localdir-", "local configuration", "local-config")
	dirs[2] = newDirDesc(t, "systemdir-", "system configuration", "system-config")
	retDirs := dirs

	dirs = nil
	return retDirs
}

// dir returns a directory description at a given index.
func (ctx *RktRunCtx) dir(idx int) *DirDesc {
	ctx.ensureValid()
	return ctx.directories[idx]
}

// cleanupChildren tries to stop all the registered children.
func (ctx *RktRunCtx) cleanupChildren() {
	for _, child := range ctx.children {
		if child.Cmd.ProcessState.Exited() {
			ctx.t.Logf("Child %q already exited", child.Cmd.Path)
			continue
		}
		ctx.t.Logf("Shutting down child %q", child.Cmd.Path)
		if err := child.Cmd.Process.Kill(); err != nil {
			ctx.t.Errorf("Failed to kill the child process: %v", err)
			continue
		}
		if _, err := child.Cmd.Process.Wait(); err != nil {
			ctx.t.Errorf("Failed to wait for the child process: %v", err)
		}
	}
}

// rktOptions returns all rkt flags for setting the directories.
func (ctx *RktRunCtx) rktOptions() []string {
	ctx.ensureValid()
	opts := make([]string, 0, len(ctx.directories))
	for _, d := range ctx.directories {
		opts = append(opts, d.RktOption())
	}
	return opts
}

// ensureValid makes sure that all the managed directories are valid.
func (ctx *RktRunCtx) ensureValid() {
	for _, d := range ctx.directories {
		d.ensureValid()
	}
}
