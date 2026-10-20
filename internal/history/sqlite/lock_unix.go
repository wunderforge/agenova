//go:build !windows

// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package sqlite

import (
	"os"
	"syscall"
)

func lockFile(f *os.File) error   { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
func unlockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
