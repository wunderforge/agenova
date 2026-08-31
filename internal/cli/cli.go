// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/wunderforge/agenova/internal/runtime"
)

// Version is the reported CLI version. Releases may override it with ldflags.
var Version = "dev"

// ExitUsage is returned for unknown commands, unknown flags, and invalid configuration.
const ExitUsage = 2

// RuntimeFactory constructs the hosted RuntimeBackend for a requested backend name.
// An empty name must resolve to the in-memory reference backend. Tests inject doubles
// by supplying a factory that returns a stand-in implementation.
type RuntimeFactory func(backendName string) (backend runtime.RuntimeBackend, resolvedName string, err error)

const helpText = `Agenova hosts claim-scoped application services for one agent worker run.

Usage:
  agenova [flags] [command]

Commands:
  help       Show this help
  version    Print version and the hosted runtime backend

Flags:
  --backend string   Runtime backend to host (default "memory")
  --help             Show this help
  --version          Print version and the hosted runtime backend

This composition root hosts the in-memory reference backend. Command behavior
does not import Kubernetes or other provider types, and it does not accept
authority flags such as --repo, --tools, or --model.

agenova run -f is not part of this binary yet.
`

type parsedArgs struct {
	help       bool
	version    bool
	command    string
	backend    string
	backendSet bool
}

// Main is the CLI entrypoint. args[0] is the program name, matching os.Args.
func Main(args []string, stdout, stderr io.Writer, newRuntime RuntimeFactory) int {
	argv := []string{}
	if len(args) > 0 {
		argv = args[1:]
	}

	parsed, err := parseArgs(argv)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		fmt.Fprintln(stderr, "Run 'agenova --help' for usage.")
		return ExitUsage
	}

	if parsed.help || parsed.command == "help" || (parsed.command == "" && !parsed.version) {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	if parsed.version || parsed.command == "version" {
		return printVersion(stdout, stderr, parsed.backend, newRuntime)
	}

	fmt.Fprintf(stderr, "unknown command %q\n", parsed.command)
	fmt.Fprintln(stderr, "Run 'agenova --help' for usage.")
	return ExitUsage
}

func printVersion(stdout, stderr io.Writer, backendName string, newRuntime RuntimeFactory) int {
	if newRuntime == nil {
		fmt.Fprintln(stderr, "runtime factory is not configured")
		return 1
	}
	backend, resolved, err := newRuntime(backendName)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		fmt.Fprintln(stderr, "Run 'agenova --help' for usage.")
		return ExitUsage
	}
	if backend == nil {
		fmt.Fprintln(stderr, "runtime factory returned no backend")
		return 1
	}
	fmt.Fprintf(stdout, "agenova %s\n", Version)
	fmt.Fprintf(stdout, "runtime-backend: %s\n", resolved)
	return 0
}

func parseArgs(argv []string) (parsedArgs, error) {
	var parsed parsedArgs
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case arg == "--help" || arg == "-h":
			parsed.help = true
		case arg == "--version" || arg == "-v":
			parsed.version = true
		case arg == "--backend":
			if i+1 >= len(argv) {
				return parsedArgs{}, fmt.Errorf("flag --backend requires a value")
			}
			i++
			if err := setBackend(&parsed, argv[i]); err != nil {
				return parsedArgs{}, err
			}
		case strings.HasPrefix(arg, "--backend="):
			if err := setBackend(&parsed, strings.TrimPrefix(arg, "--backend=")); err != nil {
				return parsedArgs{}, err
			}
		case arg == "--repo" || strings.HasPrefix(arg, "--repo=") ||
			arg == "--tools" || strings.HasPrefix(arg, "--tools=") ||
			arg == "--model" || strings.HasPrefix(arg, "--model="):
			return parsedArgs{}, fmt.Errorf("unknown flag %q\nAgenova does not grant authority through CLI flags", flagName(arg))
		case strings.HasPrefix(arg, "-") && arg != "-":
			return parsedArgs{}, fmt.Errorf("unknown flag %q", flagName(arg))
		default:
			if parsed.command != "" {
				return parsedArgs{}, fmt.Errorf("unexpected argument %q", arg)
			}
			parsed.command = arg
		}
	}
	return parsed, nil
}

func setBackend(parsed *parsedArgs, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("flag --backend requires a value")
	}
	parsed.backend = value
	parsed.backendSet = true
	return nil
}

func flagName(arg string) string {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i]
	}
	return arg
}
