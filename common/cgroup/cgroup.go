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

//+build linux

package cgroup

import (
	"errors"
	"path/filepath"
	"syscall"

	"github.com/hashicorp/errwrap"
	"github.com/rkt/rkt/common/cgroup/v1"
	"github.com/rkt/rkt/common/cgroup/v2"
)

const (
	// The following consts come from
	// #define CGROUP2_SUPER_MAGIC  0x63677270
	// #define TMPFS_MAGIC  0x01021994
	// https://github.com/torvalds/linux/blob/v4.6/include/uapi/linux/magic.h#L58
	Cgroup2fsMagicNumber = 0x63677270
	TmpfsMagicNumber     = 0x01021994
)

// IsIsolatorSupported returns whether an isolator is supported in the kernel
func IsIsolatorSupported(isolator string) (bool, error) {
	isUnified, err := IsCgroupUnified("/")
	if err != nil {
		return false, errwrap.Wrap(errors.New("error determining cgroup version"), err)
	}

	if isUnified {
		controllers, err := v2.GetEnabledControllers()
		if err != nil {
			return false, errwrap.Wrap(errors.New("error determining enabled controllers"), err)
		}
		for _, c := range controllers {
			if c == isolator {
				return true, nil
			}
		}
		return false, nil
	}
	return v1.IsControllerMounted(isolator)
}

func isDirFs(dir string, magicNumber int64) (bool, error) {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(dir, &statfs); err != nil {
		return false, err
	}

	return statfs.Type == magicNumber, nil
}

func isDirCgroupV2(dir string) (bool, error) {
	return isDirFs(dir, Cgroup2fsMagicNumber)
}

func isDirCgroupV1(dir string) (bool, error) {
	return isDirFs(dir, TmpfsMagicNumber)
}

// IsCgroupUnified checks if cgroup mounted at /sys/fs/cgroup is
// the new unified version (cgroup v2)
func IsCgroupUnified(root string) (bool, error) {
	cgroupFsPath := filepath.Join(root, "/sys/fs/cgroup")
	return isDirCgroupV2(cgroupFsPath)
}

// IsCgroupHybrid checks if the cgroup mounted at /sys/fs/cgroup is a v1-v2
// hybrid, that is, it is a cgroup v1 and has a cgroup v2 hierarchy in the
// "unified" subdirectory
func IsCgroupHybrid(root string) (bool, error) {
	cgroupFsPath := filepath.Join(root, "/sys/fs/cgroup")
	unifiedPath := filepath.Join(cgroupFsPath, "unified")
	isV1, err := isDirCgroupV1(cgroupFsPath)
	if err != nil {
		return false, err
	}
	hasUnified, err := isDirCgroupV2(unifiedPath)
	if err != nil {
		return false, err
	}

	return isV1 && hasUnified, nil
}
