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

package networking

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/networking/netinfo"
	nettypes "github.com/coreos/rkt/networking/types"
)

const (
	// Suffix to LocalConfigDir path, where users place their net configs
	UserNetPathSuffix = "net.d"

	// Default net path relative to stage1 root
	DefaultNetPath           = "etc/rkt/net.d/99-default.conf"
	DefaultRestrictedNetPath = "etc/rkt/net.d/99-default-restricted.conf"
)

// "base" struct that's populated from the beginning
// describing the environment in which the pod
// is running in
type podEnv struct {
	podRoot      string
	podID        types.UUID
	netsLoadList common.NetList
	localConfig  string
}

// Loads nets specified by user and default one from stage1
func (e *podEnv) loadNets() ([]*nettypes.ActiveNet, error) {
	nets, err := loadUserNets(e.localConfig, e.netsLoadList)
	if err != nil {
		return nil, err
	}

	if e.netsLoadList.None() {
		return nets, nil
	}

	if !netExists(nets, "default") && !netExists(nets, "default-restricted") {
		var defaultNet string
		if e.netsLoadList.Specific("default") || e.netsLoadList.All() {
			defaultNet = DefaultNetPath
		} else {
			defaultNet = DefaultRestrictedNetPath
		}
		defPath := path.Join(common.Stage1RootfsPath(e.podRoot), defaultNet)
		n, err := loadNet(defPath)
		if err != nil {
			return nil, err
		}
		nets = append(nets, n)
	}

	missing := missingNets(e.netsLoadList, nets)
	if len(missing) > 0 {
		return nil, fmt.Errorf("networks not found: %v", strings.Join(missing, ", "))
	}

	return nets, nil
}

func (e *podEnv) podNSPath() string {
	return filepath.Join(e.podRoot, "netns")
}

func (e *podEnv) netDir() string {
	return filepath.Join(e.podRoot, "net")
}

func (e *podEnv) setupNets(nets []*nettypes.ActiveNet) error {
	err := os.MkdirAll(e.netDir(), 0755)
	if err != nil {
		return err
	}

	i := 0
	defer func() {
		if err != nil {
			e.teardownNets(nets[:i])
		}
	}()

	nspath := e.podNSPath()

	var n *nettypes.ActiveNet
	for i, n = range nets {
		stderr.Printf("loading network %v with type %v", n.Conf.Name, n.Conf.Type)

		n.Runtime.IfName = fmt.Sprintf(IfNamePattern, i)
		if n.Runtime.ConfPath, err = copyFileToDir(n.Runtime.ConfPath, e.netDir()); err != nil {
			return errwrap.Wrap(fmt.Errorf("error copying %q to %q", n.Runtime.ConfPath, e.netDir()), err)
		}

		n.Runtime.IP, n.Runtime.HostIP, err = e.netPluginAdd(n, nspath)
		if err != nil {
			return errwrap.Wrap(fmt.Errorf("error adding network %q", n.Conf.Name), err)
		}
	}
	return nil
}

func (e *podEnv) teardownNets(nets []*nettypes.ActiveNet) {
	nspath := e.podNSPath()

	for i := len(nets) - 1; i >= 0; i-- {
		stderr.Printf("teardown - executing net-plugin %v", nets[i].Conf.Type)

		err := e.netPluginDel(nets[i], nspath)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("error deleting %q", nets[i].Conf.Name), err)
		}

		// Delete the conf file to signal that the network was
		// torn down (or at least attempted to)
		if err = os.Remove(nets[i].Runtime.ConfPath); err != nil {
			stderr.PrintE(fmt.Sprintf("error deleting %q", nets[i].Runtime.ConfPath), err)
		}
	}
}

func listFiles(dir string) ([]string, error) {
	dirents, err := ioutil.ReadDir(dir)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return nil, nil
	default:
		return nil, err
	}

	var files []string
	for _, dent := range dirents {
		if dent.IsDir() {
			continue
		}

		files = append(files, dent.Name())
	}

	return files, nil
}

func netExists(nets []*nettypes.ActiveNet, name string) bool {
	for _, n := range nets {
		if n.Conf.Name == name {
			return true
		}
	}
	return false
}

func loadNet(filepath string) (*nettypes.ActiveNet, error) {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	n := &nettypes.NetConf{}
	if err = json.Unmarshal(bytes, n); err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("error loading %v", filepath), err)
	}

	return &nettypes.ActiveNet{
		ConfBytes: bytes,
		Conf:      n,
		Runtime: &netinfo.NetInfo{
			NetName:  n.Name,
			ConfPath: filepath,
		},
	}, nil
}

func copyFileToDir(src, dstdir string) (string, error) {
	dst := filepath.Join(dstdir, filepath.Base(src))

	s, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return dst, err
}

func loadUserNets(localConfig string, netsLoadList common.NetList) ([]*nettypes.ActiveNet, error) {
	if netsLoadList.None() {
		stderr.Printf("networking namespace with loopback only")
		return nil, nil
	}

	userNetPath := filepath.Join(localConfig, UserNetPathSuffix)
	stderr.Printf("loading networks from %v", userNetPath)

	files, err := listFiles(userNetPath)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	nets := make([]*nettypes.ActiveNet, 0, len(files))

	for _, filename := range files {
		filepath := filepath.Join(userNetPath, filename)

		if !strings.HasSuffix(filepath, ".conf") {
			continue
		}

		n, err := loadNet(filepath)
		if err != nil {
			return nil, err
		}

		if !(netsLoadList.All() || netsLoadList.Specific(n.Conf.Name)) {
			continue
		}

		if n.Conf.Name == "default" ||
			n.Conf.Name == "default-restricted" {
			stderr.Printf(`overriding %q network with %v`, n.Conf.Name, filename)
		}

		if netExists(nets, n.Conf.Name) {
			stderr.Printf("%q network already defined, ignoring %v", n.Conf.Name, filename)
			continue
		}

		n.runtime.Args = netsLoadList.SpecificArgs(n.Conf.Name)

		nets = append(nets, n)
	}

	return nets, nil
}

func missingNets(defined common.NetList, loaded []*nettypes.ActiveNet) []string {
	diff := make(map[string]struct{})
	for _, n := range defined.StringsOnlyNames() {
		if n != "all" {
			diff[n] = struct{}{}
		}
	}

	for _, an := range loaded {
		delete(diff, an.Conf.Name)
	}

	var missing []string
	for n, _ := range diff {
		missing = append(missing, n)
	}
	return missing
}
