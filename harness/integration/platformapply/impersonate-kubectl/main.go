// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// This test-only kubectl wrapper exercises the real cluster with Kubernetes
// impersonation, without copying kubeconfig credentials into a second file.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	real := os.Getenv("AGENOVA_TEST_REAL_KUBECTL")
	user := os.Getenv("AGENOVA_TEST_IMPERSONATE_USER")
	if real == "" || user == "" {
		fmt.Fprintln(os.Stderr, "test kubectl wrapper requires real executable and impersonated user")
		os.Exit(2)
	}
	args := append([]string{"--as=" + user}, os.Args[1:]...)
	command := exec.CommandContext(context.Background(), real, args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "test kubectl wrapper could not invoke kubectl")
		os.Exit(1)
	}
}
