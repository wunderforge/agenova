// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestConsoleSignalHelper(t *testing.T) {
	if os.Getenv("AGENOVA_SIGNAL_TEST") != "1" {
		return
	}
	ctx, stop := consoleSignalContext(context.Background())
	defer stop()
	fmt.Println("ready")
	<-ctx.Done()
	fmt.Println("graceful")
}

func TestConsoleSIGTERMFollowsGracefulShutdownContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX SIGTERM delivery is exercised by Linux CI")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConsoleSignalHelper$")
	cmd.Env = append(os.Environ(), "AGENOVA_SIGNAL_TEST=1")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(pipe)
	if !scanner.Scan() || scanner.Text() != "ready" {
		t.Fatal("signal helper failed to start")
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() || scanner.Text() != "graceful" {
		t.Fatal("SIGTERM skipped graceful context")
	}
	if err = cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}
