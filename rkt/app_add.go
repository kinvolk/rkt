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

package main

import (
	"github.com/coreos/rkt/common/apps"

	"github.com/coreos/rkt/common"
	pkgPod "github.com/coreos/rkt/pkg/pod"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/coreos/rkt/store/treestore"

	"github.com/spf13/cobra"
)

var (
	cmdAppAdd = &cobra.Command{
		Use:   "add UUID IMAGEID [--memory=LIMIT]...",
		Short: "Add an app to a pod",
		Long:  `This allows addin an app that's present on the store to a running pod`,
		Run:   runWrapper(runAppAdd),
	}
)

func init() {
	cmdApp.AddCommand(cmdAppAdd)
	cmdAppAdd.Flags().Var((*appMemoryLimit)(&rktApps), "memory", "memory limit for the preceding image (example: '--memory=16Mi', '--memory=50M', '--memory=1G')")

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. All the subsequent parsing will be done by parseApps.
	// This is needed to correctly handle image args
	cmdAppAdd.Flags().SetInterspersed(false)
}

func runAppAdd(cmd *cobra.Command, args []string) (exit int) {
	if len(args) < 2 {
		stderr.Print("must provide the pod UUID and an IMAGEID")
		return 1
	}

	err := parseApps(&rktApps, args[1:], cmd.Flags(), true)
	if err != nil {
		stderr.PrintE("error parsing app image arguments", err)
		return 1
	}
	if rktApps.Count() > 1 {
		stderr.Print("must give only one app")
		return 1
	}

	p, err := pkgPod.PodFromUUIDString(getDataDir(), args[0])
	if err != nil {
		stderr.PrintE("problem retrieving pod", err)
		return 1
	}
	defer p.Close()

	if p.State() != pkgPod.Running {
		stderr.Printf("pod %q isn't currently running", p.UUID)
		return 1
	}

	s, err := imagestore.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}

	ts, err := treestore.NewStore(treeStoreDir(), s)
	if err != nil {
		stderr.PrintE("cannot open treestore", err)
		return 1
	}

	fn := &image.Finder{
		S:  s,
		Ts: ts,
		Ks: getKeystore(),

		StoreOnly: true,
		NoStore:   false,
	}

	img, err := fn.FindImage(args[1], "", apps.AppImageGuess)
	if err != nil {
		stderr.PrintE("error finding images", err)
		return 1
	}

	cfg := stage0.CommonConfig{
		Store:     s,
		TreeStore: ts,
		UUID:      p.UUID,
		Debug:     globalFlags.Debug,
	}

	rktgid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		stderr.Printf("group %q not found, will use default gid when rendering images", common.RktGroup)
		rktgid = -1
	}

	pcfg := stage0.RunConfig{
		CommonConfig: &cfg,
		UseOverlay:   p.UsesOverlay(),
		RktGid:       rktgid,
	}

	err = stage0.AddApp(pcfg, p.Path(), img)
	if err != nil {
		stderr.PrintE("error adding app to pod", err)
		return 1
	}

	return 0
}
