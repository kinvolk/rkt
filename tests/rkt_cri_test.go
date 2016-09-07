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

// +build !fly

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/rkt/tests/testutils"
)

func TestCRI(t *testing.T) {
	aciSleep := patchTestACI("rkt-inspect-sleep.aci", "--name=coreos.com/rkt-inspect/sleep", "--exec=/inspect --sleep=100")
	defer os.Remove(aciSleep)

	aciHello := patchTestACI("rkt-inspect-sleep.aci", "--name=coreos.com/rkt-inspect/hello", "--exec=/inspect --print-msg=HelloCRI")
	defer os.Remove(aciHello)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	cmd := strings.Fields(fmt.Sprintf("%s fetch --insecure-options=image %s %s", ctx.Cmd(), aciSleep, aciHello))
	fetchOutput, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("Unexpected error: %v\n%s", err, fetchOutput)
	}

	tmpDir := createTempDirOrPanic("rkt-test-cri-")
	defer os.RemoveAll(tmpDir)

	rktCmd := fmt.Sprintf("%s app sandbox --debug --uuid-file-save=%s/uuid", ctx.Cmd(), tmpDir)
	child := spawnOrFail(t, rktCmd)

	expected := "Reached target rkt apps target."
	if err := expectTimeoutWithOutput(child, expected, time.Minute); err != nil {
		t.Fatalf("Expected %q but not found: %v", expected, err)
	}

	// K8s will really need a "rkt prepare" for "app sandbox" so we know when the
	// uuid file is written.
	time.Sleep(3 * time.Second)

	podUUID, err := ioutil.ReadFile(filepath.Join(tmpDir, "uuid"))
	if err != nil {
		t.Fatalf("Can't read pod UUID: %v", err)
	}

	cmd = strings.Fields(fmt.Sprintf("%s app add %s %s", ctx.Cmd(), podUUID, "coreos.com/rkt-inspect/hello"))
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("Unexpected error: %v\n%s", err, output)
	}

	cmd = strings.Fields(fmt.Sprintf("%s app start %s --app=%s", ctx.Cmd(), podUUID, "hello"))
	output, err = exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("Unexpected error: %v\n%s", err, output)
	}

	expected = "HelloCRI"
	if err := expectTimeoutWithOutput(child, expected, time.Minute); err != nil {
		t.Fatalf("Expected %q but not found: %v", expected, err)
	}

	cmd = strings.Fields(fmt.Sprintf("%s stop %s", ctx.Cmd(), podUUID))
	output, err = exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("Unexpected error: %v\n%s", err, output)
	}

	if err := child.Wait(); err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}
