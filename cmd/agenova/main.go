// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/wunderforge/agenova/internal/app"
	"github.com/wunderforge/agenova/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args, os.Stdout, os.Stderr, app.NewRuntime))
}
