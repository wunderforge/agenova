// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Command engineer is the E9-T1 example Agent Artifact: it reads one
// canonical ClaimRequest (YAML or JSON) via --task and executes the
// requested tool sequence through a mock ToolClient. It demonstrates the
// worker side of the governance contract at fixture depth — tool access only
// through an explicit interface, no credentials — and is not live Agenova
// enforcement.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/examples/engineer/toolclient"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, &toolclient.Mock{}))
}

// run keeps the whole worker testable in-process: tests inject a mock client
// and buffers, then assert on the exit code, the printed output, and the
// invocations the mock recorded. The injected ToolClient is the only tool
// path; run never reaches a tool any other way.
func run(args []string, stdout, stderr io.Writer, client toolclient.ToolClient) int {
	flags := flag.NewFlagSet("engineer", flag.ContinueOnError)
	flags.SetOutput(stderr)
	taskPath := flags.String("task", "", "path to a ClaimRequest YAML or JSON file")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *taskPath == "" {
		fmt.Fprintln(stderr, "engineer: --task is required: path to a ClaimRequest YAML or JSON file")
		return exitUsage
	}

	request, err := loadClaimRequest(*taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "engineer: %v\n", err)
		return exitFailure
	}

	fmt.Fprintln(stdout, toolclient.FixtureModeLabel)
	fmt.Fprintf(stdout, "engineer: claim request %q template=%s task=%s\n",
		request.Metadata.Name, request.Spec.TemplateRef, request.Spec.Task.Type)

	scope := strings.Join(request.Spec.RequestedAccess.ResourceScopes, ",")
	for _, tool := range request.Spec.RequestedAccess.Tools {
		if err := client.Invoke(tool, scope); err != nil {
			fmt.Fprintf(stderr, "engineer: tool %q failed: %v\n", tool, err)
			return exitFailure
		}
		fmt.Fprintf(stdout, "engineer: invoked tool=%s scope=%s result=ok\n", tool, scope)
	}
	fmt.Fprintf(stdout, "engineer: completed %d mocked tool invocations\n",
		len(request.Spec.RequestedAccess.Tools))
	return exitOK
}

func loadClaimRequest(path string) (*v1alpha1.ClaimRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file %s: %w", path, err)
	}
	var request *v1alpha1.ClaimRequest
	var invalid *v1alpha1.ValidationError
	if strings.EqualFold(filepath.Ext(path), ".json") {
		request, invalid = v1alpha1.ParseClaimRequestJSON(data)
	} else {
		request, invalid = v1alpha1.ParseClaimRequestYAML(data)
	}
	if invalid != nil {
		return nil, fmt.Errorf("invalid ClaimRequest in %s: %v", path, invalid)
	}
	return request, nil
}
