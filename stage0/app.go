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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/aci"
	"github.com/coreos/rkt/pkg/user"
	// FIXME this should not be in stage1 anymore
	stage1types "github.com/coreos/rkt/stage1/common/types"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
)

var (
	// TODO refactor this, it's also in stage1/init/common/pod.go
	defaultEnv = map[string]string{
		"PATH":    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"SHELL":   "/bin/sh",
		"USER":    "root",
		"LOGNAME": "root",
		"HOME":    "/root",
	}
)

// TODO(iaguis): add override options for Exec, Environment (à la patch-manifest)
func AddApp(cfg RunConfig, dir string, img *types.Hash) error {
	im, err := cfg.Store.GetImageManifest(img.String())
	if err != nil {
		return err
	}
	appName, err := imageNameToAppName(im.Name)
	if err != nil {
		return err
	}

	p, err := stage1types.LoadPod(dir, cfg.UUID)
	if err != nil {
		return errwrap.Wrap(errors.New("error loading pod manifest"), err)
	}

	pm := p.Manifest

	if pm.Apps.Get(*appName) != nil {
		return fmt.Errorf("error: multiple apps with name %s", *appName)
	}
	if im.App == nil {
		return fmt.Errorf("error: image %s has no app section)", img)
	}

	appInfoDir := common.AppInfoPath(dir, *appName)
	if err := os.MkdirAll(appInfoDir, common.DefaultRegularDirPerm); err != nil {
		return errwrap.Wrap(errors.New("error creating apps info directory"), err)
	}

	uidRange := user.NewBlankUidRange()
	// TODO(iaguis): DRY: refactor this
	if cfg.UseOverlay {
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

		imgDir := filepath.Join(dir, "overlay", treeStoreID)
		if err := os.Chown(imgDir, -1, cfg.RktGid); err != nil {
			return err
		}
	} else {
		ad := common.AppPath(dir, *appName)

		err := os.MkdirAll(ad, common.DefaultRegularDirPerm)
		if err != nil {
			return errwrap.Wrap(errors.New("error creating image directory"), err)
		}

		privateUsers, err := preparedWithPrivateUsers(dir)
		if err != nil {
			log.FatalE("error reading user namespace information", err)
		}

		if err := uidRange.Deserialize([]byte(privateUsers)); err != nil {
			return err
		}

		shiftedUid, shiftedGid, err := uidRange.ShiftRange(uint32(os.Getuid()), uint32(os.Getgid()))
		if err != nil {
			return errwrap.Wrap(errors.New("error getting uid, gid"), err)
		}

		if err := os.Chown(ad, int(shiftedUid), int(shiftedGid)); err != nil {
			return errwrap.Wrap(fmt.Errorf("error shifting app %q's stage2 dir", *appName), err)
		}

		if err := aci.RenderACIWithImageID(*img, ad, cfg.Store, uidRange); err != nil {
			return errwrap.Wrap(errors.New("error rendering ACI"), err)
		}
	}

	if err := writeManifest(*cfg.CommonConfig, *img, appInfoDir); err != nil {
		return errwrap.Wrap(errors.New("error writing manifest"), err)
	}

	if err := setupAppImage(cfg, *appName, *img, dir, cfg.UseOverlay); err != nil {
		return fmt.Errorf("error setting up app image: %v", err)
	}

	ra := schema.RuntimeApp{
		Name: *appName,
		App:  im.App,
		Image: schema.RuntimeImage{
			Name:   &im.Name,
			ID:     *img,
			Labels: im.Labels,
		},
		// TODO(iaguis): default isolators
	}

	env := ra.App.Environment

	env.Set("AC_APP_NAME", appName.String())
	envFilePath := filepath.Join(common.Stage1RootfsPath(dir), "rkt", "env", appName.String())

	if err := writeEnvFile(env, uidRange, envFilePath); err != nil {
		return err
	}

	apps := append(p.Manifest.Apps, ra)
	p.Manifest.Apps = apps

	if err := updatePodManifest(dir, p.Manifest); err != nil {
		return err
	}

	return nil
}

// TODO refactor this, it's also in stage1/init/common/pod.go
// writeEnvFile creates an environment file for given app name, the minimum
// required environment variables by the appc spec will be set to sensible
// defaults here if they're not provided by env.
func writeEnvFile(env types.Environment, uidRange *user.UidRange, envFilePath string) error {
	ef := bytes.Buffer{}

	for dk, dv := range defaultEnv {
		if _, exists := env.Get(dk); !exists {
			fmt.Fprintf(&ef, "%s=%s\n", dk, dv)
		}
	}

	for _, e := range env {
		fmt.Fprintf(&ef, "%s=%s\n", e.Name, e.Value)
	}

	if err := ioutil.WriteFile(envFilePath, ef.Bytes(), 0644); err != nil {
		return err
	}

	if err := shiftFiles([]string{envFilePath}, uidRange); err != nil {
		return err
	}

	return nil
}

// TODO refactor this, it's also in stage1/init/common/pod.go
// shiftFiles shifts filesToshift by the amounts specified in uidRange
func shiftFiles(filesToShift []string, uidRange *user.UidRange) error {
	if uidRange.Shift != 0 && uidRange.Count != 0 {
		for _, f := range filesToShift {
			if err := os.Chown(f, int(uidRange.Shift), int(uidRange.Shift)); err != nil {
				return err
			}
		}
	}
	return nil
}

func updatePodManifest(dir string, newPodManifest *schema.PodManifest) error {
	pmb, err := json.Marshal(newPodManifest)
	if err != nil {
		return err
	}

	debug("Writing pod manifest")
	return updateFile(common.PodManifestPath(dir), pmb)
}

func updateFile(path string, contents []byte) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile(filepath.Dir(path), "")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Chmod(fi.Mode().Perm()); err != nil {
		return err
	}

	if _, err := f.Write(contents); err != nil {
		return errwrap.Wrap(errors.New("error writing to temp file"), err)
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}

	return nil
}
