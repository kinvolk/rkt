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

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/tests/testutils"
)

const (
	rmImageReferenced = `rkt: image ID %q is referenced by some containers, cannot remove.`
	rmImageOk         = "rkt: successfully removed aci for image ID:"

	unreferencedACI = "rkt-unreferencedACI.aci"
	unreferencedApp = "coreos.com/rkt-unreferenced"
	referencedApp   = "coreos.com/rkt-inspect"

	stage1App = "coreos.com/rkt/stage1"
)

func TestImageRunRm(t *testing.T) {
	imageFile := patchTestACI(unreferencedACI, fmt.Sprintf("--name=%s", unreferencedApp))
	defer os.Remove(imageFile)
	ctx := testutils.NewRktRunCtx(t)
	defer ctx.Cleanup()

	cmd := fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.Cmd(), imageFile)
	t.Logf("Fetching %s: %v", imageFile, cmd)
	spawnAndWaitOrFail(t, cmd, WaitSuccess)

	// at this point we know that RKT_INSPECT_IMAGE env var is not empty
	referencedACI := os.Getenv("RKT_INSPECT_IMAGE")
	cmd = fmt.Sprintf("%s --insecure-skip-verify run --mds-register=false %s", ctx.Cmd(), referencedACI)
	t.Logf("Running %s: %v", referencedACI, cmd)
	spawnAndWaitOrFail(t, cmd, WaitSuccess)

	t.Logf("Retrieving stage1 image ID")
	stage1ImageID := getImageId(ctx, stage1App)

	t.Logf("Retrieving %s image ID", referencedApp)
	referencedImageID := getImageId(ctx, referencedApp)

	t.Logf("Retrieving %s image ID", unreferencedApp)
	unreferencedImageID := getImageId(ctx, unreferencedApp)

	t.Logf("Removing stage1 image (should work)")
	removeImageId(ctx, stage1ImageID, true)

	t.Logf("Removing image for app %s (should work)", referencedApp)
	removeImageId(ctx, referencedImageID, true)

	t.Logf("Removing image for app %s (should work)", unreferencedApp)
	removeImageId(ctx, unreferencedImageID, true)
}

func TestImagePrepareRmRun(t *testing.T) {
	imageFile := patchTestACI(unreferencedACI, fmt.Sprintf("--name=%s", unreferencedApp))
	defer os.Remove(imageFile)
	ctx := testutils.NewRktRunCtx(t)
	defer ctx.Cleanup()

	cmd := fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.Cmd(), imageFile)
	t.Logf("Fetching %s: %v", imageFile, cmd)
	spawnAndWaitOrFail(t, cmd, WaitSuccess)

	// at this point we know that RKT_INSPECT_IMAGE env var is not empty
	referencedACI := os.Getenv("RKT_INSPECT_IMAGE")
	cmds := strings.Fields(ctx.Cmd())
	prepareCmd := exec.Command(cmds[0], cmds[1:]...)
	prepareCmd.Args = append(prepareCmd.Args, "--insecure-skip-verify", "prepare", referencedACI)
	output, err := prepareCmd.Output()
	if err != nil {
		t.Fatalf("Cannot read the output: %v", err)
	}

	podIDStr := strings.TrimSpace(string(output))
	podID, err := types.NewUUID(podIDStr)
	if err != nil {
		t.Fatalf("%q is not a valid UUID: %v", podIDStr, err)
	}

	t.Logf("Retrieving stage1 imageID")
	stage1ImageID := getImageId(ctx, stage1App)

	t.Logf("Retrieving %s image ID", referencedApp)
	referencedImageID := getImageId(ctx, referencedApp)

	t.Logf("Retrieving %s image ID", unreferencedApp)
	unreferencedImageID := getImageId(ctx, unreferencedApp)

	t.Logf("Removing stage1 image (should work)")
	removeImageId(ctx, stage1ImageID, true)

	t.Logf("Removing image for app %s (should work)", referencedApp)
	removeImageId(ctx, referencedImageID, true)

	t.Logf("Removing image for app %s (should work)", unreferencedApp)
	removeImageId(ctx, unreferencedImageID, true)

	cmd = fmt.Sprintf("%s run-prepared --mds-register=false %s", ctx.Cmd(), podID.String())
	t.Logf("Running %s: %v", referencedACI, cmd)
	spawnAndWaitOrFail(t, cmd, WaitSuccess)
}

func getImageId(ctx *testutils.RktRunCtx, name string) string {
	t := ctx.T()
	cmd := fmt.Sprintf(`/bin/sh -c "%s image list --fields=id,name --no-legend | grep %s | awk '{print $1}'"`, ctx.Cmd(), name)
	child := spawnOrFail(t, cmd)

	imageID, err := child.ReadLine()
	if err != nil {
		t.Fatalf("Could not get an output from %q: %v", cmd, err)
	}
	imageID = strings.TrimSpace(imageID)
	imageID = string(bytes.Trim([]byte(imageID), "\x00"))

	waitOrFail(t, child, WaitSuccess)
	return imageID
}

func removeImageId(ctx *testutils.RktRunCtx, imageID string, shouldWork bool) {
	t := ctx.T()
	expect := fmt.Sprintf(rmImageReferenced, imageID)
	if shouldWork {
		expect = rmImageOk
	}

	cmd := fmt.Sprintf("%s image rm %s", ctx.Cmd(), imageID)
	child := spawnOrFail(t, cmd)
	if err := expectWithOutput(child, expect); err != nil {
		t.Fatalf("Expected %q but not found: %v", expect, err)
	}
	waitOrFail(t, child, WaitSuccess)
}
