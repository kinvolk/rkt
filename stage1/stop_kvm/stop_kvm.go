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
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const lkvmBinPath string = "stage1/rootfs/lkvm"

func stop() int {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting current directory: %v", err)
		return 1
	}

	podUUID := filepath.Base(pwd)
	lkvmName := "rkt-" + podUUID

	args := []string{lkvmBinPath, "stop", "-n", lkvmName}

	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "error execing lkvm: %v", err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(stop())
}
