// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package operator

import (
	"fmt"
	"os"
	"syscall"
)

func regularFileLinkCount(path string, _ os.FileInfo) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	var information syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &information); err != nil {
		return 0, fmt.Errorf("inspect fixture output links: %w", err)
	}
	return uint64(information.NumberOfLinks), nil
}
