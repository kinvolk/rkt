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

package lib

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/appc/spec/schema"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/hashicorp/errwrap"
)

// Exported state. See Documentation/devel/pod-lifecycle.md for some explanation
const (
	Embryo         = "embryo"
	Preparing      = "preparing"
	AbortedPrepare = "aborted prepare"
	Prepared       = "prepared"
	Running        = "running"
	Deleting       = "deleting" // This covers pod.isExitedDeleting and pod.isDeleting.
	Exited         = "exited"   // This covers pod.isExited and pod.isExitedGarbage.
	Garbage        = "garbage"
)

type Pod struct {
	*lock.FileLock

	uuid string

	dataDir string

	isEmbryo         bool // directory starts as embryo before entering preparing state, serves as stage for acquiring lock before rename to prepare/.
	isPreparing      bool // when locked at pods/prepare/$uuid the pod is actively being prepared
	isAbortedPrepare bool // when unlocked at pods/prepare/$uuid the pod never finished preparing
	isPrepared       bool // when at pods/prepared/$uuid the pod is prepared, serves as stage for acquiring lock before rename to run/.
	isExited         bool // when locked at pods/run/$uuid the pod is running, when unlocked it's exited.
	isExitedGarbage  bool // when unlocked at pods/exited-garbage/$uuid the pod is exited and is garbage
	isExitedDeleting bool // when locked at pods/exited-garbage/$uuid the pod is exited, garbage, and is being actively deleted
	isGarbage        bool // when unlocked at pods/garbage/$uuid the pod is garbage that never ran
	isDeleting       bool // when locked at pods/garbage/$uuid the pod is garbage that never ran, and is being actively deleted
	isGone           bool // when a pod no longer can be located at its uuid anywhere XXX: only set by refreshState()
}

// embryoPath returns the path to the pod where it would be in the embryoDir in its embryonic state.
func embryoPath(dataDir string, p *Pod) string {
	return filepath.Join(dataDir, "pods", "embryo", p.uuid)
}

// preparePath returns the path to the pod where it would be in the prepareDir in its preparing state.
func preparePath(dataDir string, p *Pod) string {
	return filepath.Join(dataDir, "pods", "prepare", p.uuid)
}

// preparedPath returns the path to the pod where it would be in the preparedDir.
func preparedPath(dataDir string, p *Pod) string {
	return filepath.Join(dataDir, "pods", "prepared", p.uuid)
}

// runPath returns the path to the pod where it would be in the runDir.
func runPath(dataDir string, p *Pod) string {
	return filepath.Join(dataDir, "pods", "run", p.uuid)
}

// exitedGarbagePath returns the path to the pod where it would be in the exitedGarbageDir.
func exitedGarbagePath(dataDir string, p *Pod) string {
	return filepath.Join(dataDir, "pods", "exited-garbage", p.uuid)
}

// garbagePath returns the path to the pod where it would be in the garbageDir.
func garbagePath(dataDir string, p *Pod) string {
	return filepath.Join(dataDir, "pods", "garbage", p.uuid)
}

func GetPodByFullUUID(dataDir, uuid string) (*Pod, error) {
	p := &Pod{uuid: uuid, dataDir: dataDir}

	// we try open the pod in all possible directories, in the same order the states occur
	l, err := lock.NewLock(embryoPath(dataDir, p), lock.Dir)
	if err == nil {
		p.isEmbryo = true
	} else if err == lock.ErrNotExist {
		l, err = lock.NewLock(preparePath(dataDir, p), lock.Dir)
		if err == nil {
			// treat as aborted prepare until lock is tested
			p.isAbortedPrepare = true
		} else if err == lock.ErrNotExist {
			l, err = lock.NewLock(preparedPath(dataDir, p), lock.Dir)
			if err == nil {
				p.isPrepared = true
			} else if err == lock.ErrNotExist {
				l, err = lock.NewLock(runPath(dataDir, p), lock.Dir)
				if err == nil {
					// treat as exited until lock is tested
					p.isExited = true
				} else if err == lock.ErrNotExist {
					l, err = lock.NewLock(exitedGarbagePath(dataDir, p), lock.Dir)
					if err == lock.ErrNotExist {
						l, err = lock.NewLock(garbagePath(dataDir, p), lock.Dir)
						if err == nil {
							p.isGarbage = true
						} else {
							return nil, fmt.Errorf("pod %q not found", uuid)
						}
					} else if err == nil {
						p.isExitedGarbage = true
						p.isExited = true // ExitedGarbage is _always_ implicitly exited
					}
				}
			}
		}
	}

	if err != nil && err != lock.ErrNotExist {
		return nil, errwrap.Wrap(fmt.Errorf("error opening pod %q", uuid), err)
	}

	if !p.isPrepared && !p.isEmbryo {
		// preparing, run, exitedGarbage, and garbage dirs use exclusive locks to indicate preparing/aborted, running/exited, and deleting/marked
		if err = l.TrySharedLock(); err != nil {
			if err != lock.ErrLocked {
				l.Close()
				return nil, errwrap.Wrap(errors.New("unexpected lock error"), err)
			}
			if p.isExitedGarbage {
				// locked exitedGarbage is also being deleted
				p.isExitedDeleting = true
			} else if p.isExited {
				// locked exited and !exitedGarbage is not exited (default in the run dir)
				p.isExited = false
			} else if p.isAbortedPrepare {
				// locked in preparing is preparing, not aborted (default in the preparing dir)
				p.isAbortedPrepare = false
				p.isPreparing = true
			} else if p.isGarbage {
				// locked in non-exited garbage is deleting
				p.isDeleting = true
			}
			err = nil
		} else {
			l.Unlock()
		}
	}

	p.FileLock = l

	return p, nil
}

func (p *Pod) path() string {
	if p.isEmbryo {
		return embryoPath(p.dataDir, p)
	} else if p.isPreparing || p.isAbortedPrepare {
		return preparePath(p.dataDir, p)
	} else if p.isPrepared {
		return preparedPath(p.dataDir, p)
	} else if p.isExitedGarbage {
		return exitedGarbagePath(p.dataDir, p)
	} else if p.isGarbage {
		return garbagePath(p.dataDir, p)
	} else if p.isGone {
		return "" // TODO(vc): anything better?
	}

	return runPath(p.dataDir, p)
}

// readFile reads an entire file from a pod's directory.
func (p *Pod) readFile(path string) ([]byte, error) {
	f, err := p.openFile(path, syscall.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}

// openFile opens a file from a pod's directory returning a file descriptor.
func (p *Pod) openFile(path string, flags int) (*os.File, error) {
	cdirfd, err := p.Fd()
	if err != nil {
		return nil, err
	}

	fd, err := syscall.Openat(cdirfd, path, flags, 0)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), path), nil
}

func (p *Pod) GetPodManifest() (*schema.PodManifest, error) {
	pmb, err := p.readFile("pod")
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error reading pod manifest"), err)
	}
	pm := &schema.PodManifest{}
	if err = pm.UnmarshalJSON(pmb); err != nil {
		return nil, errwrap.Wrap(errors.New("invalid pod manifest"), err)
	}
	return pm, nil
}

// GetState returns the current state of the pod
func (p *Pod) GetState() string {
	state := "running"

	if p.isEmbryo {
		state = Embryo
	} else if p.isPreparing {
		state = Preparing
	} else if p.isAbortedPrepare {
		state = AbortedPrepare
	} else if p.isPrepared {
		state = Prepared
	} else if p.isExitedDeleting || p.isDeleting {
		state = Deleting
	} else if p.isExited { // this covers p.isExitedGarbage
		state = Exited
	} else if p.isGarbage {
		state = Garbage
	}

	return state
}

// AfterRun tests if a pod is in a post-running state
func (p *Pod) AfterRun() bool {
	return p.isExitedDeleting || p.isDeleting || p.isExited || p.isGarbage
}
