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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/aci"
	"github.com/coreos/rkt/pkg/user"
	// FIXME this should not be in stage1 anymore
	stage1types "github.com/coreos/rkt/stage1/common/types"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
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
	var treeStoreID string
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
		cfg.RootHash = hash

		if err := ioutil.WriteFile(common.AppTreeStoreIDPath(dir, *appName), []byte(treeStoreID), common.DefaultRegularFilePerm); err != nil {
			return errwrap.Wrap(errors.New("error writing app treeStoreID"), err)
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

	if cfg.UseOverlay {
		imgDir := filepath.Join(dir, "overlay", treeStoreID)
		if err := os.Chown(imgDir, -1, cfg.RktGid); err != nil {
			return err
		}
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

	if err := common.WriteEnvFile(env, uidRange, envFilePath); err != nil {
		return err
	}

	apps := append(p.Manifest.Apps, ra)
	p.Manifest.Apps = apps

	if err := updatePodManifest(dir, p.Manifest); err != nil {
		return err
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

func RmApp(dir string, uuid *types.UUID, usesOverlay bool, appName *types.ACName, podPID int) error {
	p, err := stage1types.LoadPod(dir, uuid)
	if err != nil {
		return errwrap.Wrap(errors.New("error loading pod manifest"), err)
	}

	pm := p.Manifest
	app := pm.Apps.Get(*appName)
	if app == nil {
		return fmt.Errorf("error: nonexistent app %q", *appName)
	}

	treeStoreID, err := ioutil.ReadFile(common.AppTreeStoreIDPath(dir, *appName))
	if err != nil {
		return err
	}

	// TODO call stop entry point

	s1rootfs := common.Stage1RootfsPath(dir)

	previousDir, err := os.Getwd()
	if err != nil {
		return err
	}

	debug("Pivoting to filesystem %s", dir)
	if err := os.Chdir(dir); err != nil {
		log.FatalE("failed changing to dir", err)
	}

	rmEp, err := getStage1Entrypoint(dir, appRmEntrypoint)
	if err != nil {
		return fmt.Errorf("rkt app rm not implemented for pod's stage1: %v", err)
	}
	args := []string{filepath.Join(s1rootfs, rmEp)}
	debug("Execing %s", rmEp)

	enterEp, err := getStage1Entrypoint(dir, enterEntrypoint)
	if err != nil {
		return errwrap.Wrap(errors.New("error determining 'enter' entrypoint"), err)
	}

	args = append(args, uuid.String())
	args = append(args, appName.String())
	args = append(args, filepath.Join(s1rootfs, enterEp))
	args = append(args, strconv.Itoa(podPID))

	c := exec.Cmd{
		Path:   args[0],
		Args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := c.Run(); err != nil {
		return fmt.Errorf("error executing stage1's app rm: %v", err)
	}

	if err := os.Chdir(previousDir); err != nil {
		log.FatalE("failed changing to dir", err)
	}

	appInfoDir := common.AppInfoPath(dir, *appName)
	if err := os.RemoveAll(appInfoDir); err != nil {
		return errwrap.Wrap(errors.New("error removing app info directory"), err)
	}

	if usesOverlay {
		appRootfs := common.AppRootfsPath(dir, *appName)
		if err := syscall.Unmount(appRootfs, 0); err != nil {
			return err
		}

		ts := filepath.Join(dir, "overlay", string(treeStoreID))
		if err := os.RemoveAll(ts); err != nil {
			return errwrap.Wrap(errors.New("error removing app info directory"), err)
		}
	}

	if err := os.RemoveAll(common.AppPath(dir, *appName)); err != nil {
		return err
	}

	appStatusPath := filepath.Join(common.Stage1RootfsPath(dir), "rkt", "status", appName.String())
	if err := os.Remove(appStatusPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	envPath := filepath.Join(common.Stage1RootfsPath(dir), "rkt", "env", appName.String())
	if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	removeAppFromPodManifest(pm, appName)

	if err := updatePodManifest(dir, pm); err != nil {
		return err
	}

	return nil
}

func removeAppFromPodManifest(pm *schema.PodManifest, appName *types.ACName) {
	for i, app := range pm.Apps {
		if app.Name == *appName {
			pm.Apps = append(pm.Apps[:i], pm.Apps[i+1:]...)
		}
	}
}
