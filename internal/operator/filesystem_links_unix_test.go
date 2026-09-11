// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package operator

import (
	"fmt"
	"os"
	"syscall"
)

func regularFileLinkCount(_ string, info os.FileInfo) (uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("inspect fixture output links: unsupported file metadata")
	}
	return uint64(stat.Nlink), nil
}
