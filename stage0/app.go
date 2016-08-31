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

package stage0

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/coreos/rkt/common"

	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
)

// FIXME RunConfig? uh. Maybe refactor
func AddApp(cfg RunConfig, dir string, img *types.Hash) error {
	im, err := cfg.Store.GetImageManifest(img.String())
	if err != nil {
		return err
	}
	appName, err := imageNameToAppName(im.Name)
	if err != nil {
		return errwrap.Wrap(errors.New("error converting image name to app name"), err)
	}

	appInfoDir := common.AppInfoPath(dir, *appName)
	if err := os.MkdirAll(appInfoDir, common.DefaultRegularDirPerm); err != nil {
		return errwrap.Wrap(errors.New("error creating apps info directory"), err)
	}

	if err := writeManifest(*cfg.CommonConfig, *img, appInfoDir); err != nil {
		return errwrap.Wrap(errors.New("error writing manifest"), err)
	}

	// TODO check overlay
	treeStoreID, _, err := cfg.TreeStore.Render(img.String(), false)
	if err != nil {
		return errwrap.Wrap(errors.New("error rendering tree image"), err)
	}

	hash, err := cfg.TreeStore.Check(treeStoreID)
	if err != nil {
		log.PrintE("warning: tree cache is in a bad state.  Rebuilding...", err)
		var err error
		treeStoreID, hash, err = cfg.TreeStore.Render(img.String(), true)
		if err != nil {
			return errwrap.Wrap(errors.New("error rendering tree image"), err)
		}
	}
	cfg.CommonConfig.RootHash = hash

	if err := ioutil.WriteFile(common.AppTreeStoreIDPath(dir, *appName), []byte(treeStoreID), common.DefaultRegularFilePerm); err != nil {
		return errwrap.Wrap(errors.New("error writing app treeStoreID"), err)
	}

	if err := setupAppImage(cfg, *appName, *img, dir, cfg.UseOverlay); err != nil {
		return fmt.Errorf("error setting up app image: %v", err)
	}

	return nil
}
