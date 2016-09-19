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

// +build host coreos src kvm

package main

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sd_dbus "github.com/coreos/go-systemd/dbus"
	sd_util "github.com/coreos/go-systemd/util"
	"github.com/coreos/rkt/tests/testutils"
	"github.com/godbus/dbus"
)

func TestNotify(t *testing.T) {
	if !sd_util.IsRunningSystemd() {
		t.Skip("Systemd is not running on the host.")
	}

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	conn, err := sd_dbus.New()
	if err != nil {
		t.Fatal(err)
	}

	imageFile := getInspectImagePath()

	image, err := filepath.Abs(imageFile)
	if err != nil {
		t.Fatal(err)
	}
	// we need to add --silent-sigterm so inspect terminates correctly and the
	// transient service exits successfully so it disappears from the list of
	// units
	sleepTime := 3
	opts := fmt.Sprintf("-- --silent-sigterm --notify --sleep=%d", sleepTime)

	cmd := fmt.Sprintf("%s --insecure-options=image run --mds-register=false %s %s", ctx.Cmd(), image, opts)
	props := []sd_dbus.Property{
		sd_dbus.PropExecStart(strings.Split(cmd, " "), false),
		// TODO alepuccetti: when https://github.com/coreos/go-systemd/pull/200 is available in the release use it
		{
			Name:  "Type",
			Value: dbus.MakeVariant("notify"),
		},
	}
	target := fmt.Sprintf("rkt-testing-transient-notify-%d.service", r.Int())

	reschan := make(chan string)
	_, err = conn.StartTransientUnit(target, "replace", props, reschan)
	if err != nil {
		t.Fatal(err)
	}

	units, err := conn.ListUnits()

	var found bool
	for _, u := range units {
		if u.Name == target {
			found = true
			if u.ActiveState != "activating" {
				t.Fatalf("Test unit %s not activating: %s (target: %s)", u.Name, u.ActiveState, target)
			}
		}
	}

	if !found {
		t.Fatalf("Test unit not found in list")
	}

	// Wait some time to allow the notification
	time.Sleep(time.Duration(sleepTime) * time.Second)
	units, err = conn.ListUnits()

	found = false
	for _, u := range units {
		if u.Name == target {
			found = true
			if u.ActiveState != "active" {
				t.Fatalf("Test unit %s not active: %s (target: %s)", u.Name, u.ActiveState, target)
			}
		}
	}

	if !found {
		t.Fatalf("Test unit not found in list")
	}

	// Wait for unit to terminate
	time.Sleep(time.Duration(sleepTime) * time.Second)

	units, err = conn.ListUnits()

	found = false
	for _, u := range units {
		if u.Name == target {
			found = true
		}
	}

	if found {
		t.Fatalf("Test unit found in list, should be stopped")
	}
}
