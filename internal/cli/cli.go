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

// RuntimeFactory constructs the hosted reduced-contract RuntimeBackend for a requested backend name.
// An empty name must resolve to the in-memory reference backend. Tests inject doubles
// by supplying a factory that returns a stand-in implementation.
type RuntimeFactory func(backendName string) (backend runtime.RuntimeBackend, resolvedName string, err error)

// RunHandler submits one ClaimRequest file through the application path.
// Command behavior does not parse YAML or grant authority; the host does.
// The hosted backend is supplied to the application composition boundary.
type RunHandler func(path string, backend runtime.RuntimeBackend) (RunReport, error)

// RunReport is the backend-neutral submission result shown by `agenova run -f`.
type RunReport struct {
	RequestRef string
	Decision   string
	Principal  string
	Allocated  bool
	ClaimID    string
	Phase      string
}

const helpText = `Agenova hosts claim-scoped application services for one agent worker run.

Usage:
  agenova [flags] [command]

Commands:
  help       Show this help
  version    Print version and the hosted runtime backend
  run        Submit one ClaimRequest file through application resolution

Flags:
  --backend string   Runtime backend to host (default "memory")
  --help             Show this help
  --version          Print version and the hosted runtime backend
  -f, --file string  ClaimRequest YAML for agenova run

This composition root hosts the in-memory reference backend. Command behavior
does not import Kubernetes or other provider types, and it does not accept
authority flags such as --repo, --tools, or --model.

agenova run -f <file> submits the canonical ClaimRequest schema. Identity
comes from the local principal boundary, not from the file or CLI flags.
`

const runHelpText = `Usage:
  agenova run -f <claim-request.yaml>

Submit exactly one ClaimRequest YAML document. Requested access is intent.
The CLI does not accept --repo, --tools, or --model authority shortcuts.
`

type parsedArgs struct {
	help       bool
	version    bool
	command    string
	backend    string
	backendSet bool
	file       string
	fileSet    bool
}

// Main is the CLI entrypoint. args[0] is the program name, matching os.Args.
func Main(args []string, stdout, stderr io.Writer, newRuntime RuntimeFactory, run RunHandler) int {
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

	if parsed.help && parsed.command == "run" {
		fmt.Fprint(stdout, runHelpText)
		return 0
	}
	if parsed.help || parsed.command == "help" || (parsed.command == "" && !parsed.version) {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	if parsed.version || parsed.command == "version" {
		return printVersion(stdout, stderr, parsed.backend, newRuntime)
	}

	if parsed.command == "run" {
		return printRun(stdout, stderr, parsed, newRuntime, run)
	}

	fmt.Fprintf(stderr, "unknown command %q\n", parsed.command)
	fmt.Fprintln(stderr, "Run 'agenova --help' for usage.")
	return ExitUsage
}

func printRun(stdout, stderr io.Writer, parsed parsedArgs, newRuntime RuntimeFactory, run RunHandler) int {
	if !parsed.fileSet || strings.TrimSpace(parsed.file) == "" {
		fmt.Fprintln(stderr, "run requires -f <claim-request.yaml>")
		fmt.Fprintln(stderr, "Run 'agenova run --help' for usage.")
		return ExitUsage
	}
	if newRuntime == nil {
		fmt.Fprintln(stderr, "runtime factory is not configured")
		return 1
	}
	backend, _, err := newRuntime(parsed.backend)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		fmt.Fprintln(stderr, "Run 'agenova --help' for usage.")
		return ExitUsage
	}
	if backend == nil {
		fmt.Fprintln(stderr, "runtime factory returned no backend")
		return 1
	}
	if run == nil {
		fmt.Fprintln(stderr, "run handler is not configured")
		return 1
	}
	report, err := run(parsed.file, backend)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		fmt.Fprintln(stderr, "Run 'agenova run --help' for usage.")
		return ExitUsage
	}
	fmt.Fprintf(stdout, "request: %s\n", report.RequestRef)
	fmt.Fprintf(stdout, "decision: %s\n", report.Decision)
	fmt.Fprintf(stdout, "principal: %s\n", report.Principal)
	fmt.Fprintf(stdout, "allocated: %t\n", report.Allocated)
	if report.ClaimID != "" {
		fmt.Fprintf(stdout, "claim: %s\n", report.ClaimID)
	}
	if report.Phase != "" {
		fmt.Fprintf(stdout, "phase: %s\n", report.Phase)
	}
	if strings.EqualFold(report.Decision, "Deny") {
		return 1
	}
	return 0
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
			if i+1 >= len(argv) || looksLikeFlag(argv[i+1]) {
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
		case arg == "-f" || arg == "--file":
			if i+1 >= len(argv) || looksLikeFlag(argv[i+1]) {
				return parsedArgs{}, fmt.Errorf("flag -f requires a value")
			}
			i++
			if err := setFile(&parsed, argv[i]); err != nil {
				return parsedArgs{}, err
			}
		case strings.HasPrefix(arg, "--file="):
			if err := setFile(&parsed, strings.TrimPrefix(arg, "--file=")); err != nil {
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
	if parsed.fileSet && parsed.command != "run" && !parsed.help {
		return parsedArgs{}, fmt.Errorf("-f is only valid with agenova run")
	}
	return parsed, nil
}

func setFile(parsed *parsedArgs, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("flag -f requires a value")
	}
	parsed.file = value
	parsed.fileSet = true
	return nil
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

func looksLikeFlag(arg string) bool {
	return strings.HasPrefix(arg, "-") && arg != "-"
}
