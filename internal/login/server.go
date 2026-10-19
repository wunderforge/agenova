// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package login

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The default endpoint contains no credentials. Tokens remain in the OS store.
func serverPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agenova", "current-server"), nil
}
func DefaultServer() (string, error) {
	path, err := serverPath()
	if err != nil {
		return "", err
	}
	return readServer(path)
}
func readServer(path string) (string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("cannot read saved server; use --server explicitly")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return "", errors.New("cannot read saved server")
	}
	server := strings.TrimSpace(string(data))
	if len(data) > 4096 || !Origin(server) {
		return "", errors.New("invalid saved server; use agenova login --server HTTPS_ORIGIN to replace it")
	}
	return server, nil
}
func RememberServer(server string) error {
	path, err := serverPath()
	if err != nil {
		return err
	}
	return writeServer(path, server)
}
func writeServer(path, server string) error {
	if !Origin(server) {
		return errors.New("invalid default server")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".current-server-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.WriteString(server + "\n"); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
func ForgetServer(server string) error {
	path, err := serverPath()
	if err != nil {
		return err
	}
	current, err := readServer(path)
	if err != nil {
		return err
	}
	if current != server {
		return nil
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
