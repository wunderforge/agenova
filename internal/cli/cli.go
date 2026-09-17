// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/wunderforge/agenova/internal/adapterregistry"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/platform"
	"github.com/wunderforge/agenova/internal/platformapply"
	"github.com/wunderforge/agenova/internal/registration"
	"github.com/wunderforge/agenova/internal/runtime"
	"gopkg.in/yaml.v3"
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
type ConnectedRunHandler func(path, stateDirectory string) (RunReport, error)
type ConnectedShowHandler func(ref, stateDirectory string) (evidence.View, error)
type ConnectedListHandler func(stateDirectory string) ([]evidence.View, error)
type APIConnectHandler func(stateDirectory string, port int, stdout, stderr io.Writer) error

type AdapterLifecycleFactory func(stateDirectory string) (*adapterregistry.Lifecycle, error)

type PlatformServiceFactory func(stateDirectory string) (platformapply.Service, error)
type RegistrationServiceFactory func(stateDirectory string) (registration.Service, error)

type Services struct {
	NewRuntime      RuntimeFactory
	Run             RunHandler
	RunConnected    ConnectedRunHandler
	ShowConnected   ConnectedShowHandler
	ListConnected   ConnectedListHandler
	ConnectAPI      APIConnectHandler
	NewAdapters     AdapterLifecycleFactory
	NewPlatform     PlatformServiceFactory
	NewRegistration RegistrationServiceFactory
	Input           io.Reader
}

// RunReport is the backend-neutral submission result shown by `agenova run -f`.
type RunReport struct {
	RequestRef string
	Decision   string
	Principal  string
	Allocated  bool
	ClaimID    string
	Phase      string
	Evidence   *evidence.View
}

const helpText = `Agenova hosts claim-scoped application services for one agent worker run.

Usage:
  agenova [flags] [command]

Commands:
  help       Show this help
  version    Print version and the hosted runtime backend
  run        Submit one ClaimRequest file through application resolution
  adapters   Inspect and activate bundled adapter implementations
  platform   Validate, plan, apply, and inspect one declarative Platform
  policy     Register and activate an immutable PolicyBundle
  agent-template Register an immutable AgentTemplate
  work       Query current-session Work evidence from the installed service
  api        Connect the local Portal to the private installed API

Flags:
  --backend string    Explicit reference runtime backend ("memory"); default run uses the installed service
  --state-dir string  Local Agenova installation state for adapter commands
  --help              Show this help
  --version           Print version and the hosted runtime backend
  -f, --file string   ClaimRequest YAML for agenova run
  --json              Print JSON for run or adapter commands
  --yes               Confirm platform apply non-interactively
  --port int          Local loopback port for api connect (default 8088)

The default run command submits to the installed reference service. Use
--backend memory to opt into the isolated in-memory reference path. Command
behavior stays provider-neutral and does not accept authority flags such as
--repo, --tools, or --model.

agenova run -f <file> submits the canonical ClaimRequest schema. Identity
comes from the local principal boundary, not from the file or CLI flags.
`

const runHelpText = `Usage:
  agenova run -f <claim-request.yaml>
  agenova run -f <claim-request.yaml> --json

Submit exactly one ClaimRequest YAML document. Requested access is intent.
The CLI does not accept --repo, --tools, or --model authority shortcuts.
`

const workHelpText = `Usage:
  agenova work list [--json]
  agenova work show <request-ref> [--json]

Query current-session Work from the installed service's private API.
`

const apiHelpText = `Usage:
  agenova api connect [--port 8088]

Keep this command running while the local Portal uses the private API.
The tunnel binds 127.0.0.1 only; it is transport, not user authentication.
`

const adaptersHelpText = `Usage:
  agenova adapters catalog [--json]
  agenova adapters list [--json]
  agenova adapters inspect <id[@version]> [--json]
  agenova adapters install <id[@version]> [--json]
  agenova adapters init <id[@version]> [--name <local-name>] [--json]

Catalog is the set of implementations shipped with this Agenova distribution.
List is the exact set activated for the selected --state-dir installation.
Init emits a reviewable Platform YAML fragment; it does not grant authority.
`

const platformHelpText = `Usage:
  agenova platform validate -f <platform.yaml> [--json]
  agenova platform plan -f <platform.yaml> [--json]
  agenova platform apply -f <platform.yaml> [--yes] [--json]
  agenova platform status [--json]

Validate and plan never mutate the selected target. Apply uses the context and
namespace from the deployment adapter config and the caller's current identity.
`

type parsedArgs struct {
	help       bool
	version    bool
	command    string
	backend    string
	backendSet bool
	file       string
	fileSet    bool
	json       bool
	stateDir   string
	stateSet   bool
	name       string
	nameSet    bool
	yes        bool
	port       int
	portSet    bool
	operands   []string
}

// Main preserves the reduced composition entrypoint used by existing tests.
func Main(args []string, stdout, stderr io.Writer, newRuntime RuntimeFactory, run RunHandler) int {
	return MainWithServices(args, stdout, stderr, Services{NewRuntime: newRuntime, Run: run})
}

// MainWithServices is the full composition-aware entrypoint. The executable
// injects registry/lifecycle construction; command behavior never switches on
// provider identities.
func MainWithServices(args []string, stdout, stderr io.Writer, services Services) int {
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
	if parsed.help && parsed.command == "adapters" {
		fmt.Fprint(stdout, adaptersHelpText)
		return 0
	}
	if parsed.help && parsed.command == "platform" {
		fmt.Fprint(stdout, platformHelpText)
		return 0
	}
	if parsed.help && parsed.command == "work" {
		fmt.Fprint(stdout, workHelpText)
		return 0
	}
	if parsed.help && parsed.command == "api" {
		fmt.Fprint(stdout, apiHelpText)
		return 0
	}
	if parsed.help && (parsed.command == "policy" || parsed.command == "agent-template") {
		fmt.Fprintf(stdout, "Usage: agenova %s apply -f <document.yaml> [--json]\n", parsed.command)
		return 0
	}
	if parsed.help || parsed.command == "help" || (parsed.command == "" && !parsed.version) {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	if parsed.version || parsed.command == "version" {
		return printVersion(stdout, stderr, parsed.backend, services.NewRuntime)
	}
	if parsed.command == "run" {
		if services.RunConnected != nil && !parsed.backendSet {
			return printConnectedRun(stdout, stderr, parsed, services.RunConnected)
		}
		return printRun(stdout, stderr, parsed, services.NewRuntime, services.Run)
	}
	if parsed.command == "work" {
		return printWork(stdout, stderr, parsed, services)
	}
	if parsed.command == "api" {
		if len(parsed.operands) != 1 || parsed.operands[0] != "connect" || services.ConnectAPI == nil {
			fmt.Fprint(stderr, apiHelpText)
			return ExitUsage
		}
		if err := services.ConnectAPI(parsed.stateDir, parsed.port, stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if parsed.command == "adapters" {
		return printAdapters(stdout, stderr, parsed, services.NewAdapters)
	}
	if parsed.command == "platform" {
		return printPlatform(stdout, stderr, parsed, services)
	}
	if parsed.command == "policy" || parsed.command == "agent-template" {
		return printRegistration(stdout, stderr, parsed, services.NewRegistration)
	}

	fmt.Fprintf(stderr, "unknown command %q\n", parsed.command)
	fmt.Fprintln(stderr, "Run 'agenova --help' for usage.")
	return ExitUsage
}

func printConnectedRun(stdout, stderr io.Writer, parsed parsedArgs, run ConnectedRunHandler) int {
	if !parsed.fileSet || strings.TrimSpace(parsed.file) == "" {
		fmt.Fprintln(stderr, "run requires -f <claim-request.yaml>")
		return ExitUsage
	}
	report, err := run(parsed.file, parsed.stateDir)
	if report.Evidence != nil {
		if outputErr := printRunReport(stdout, report, parsed.json); outputErr != nil {
			fmt.Fprintln(stderr, outputErr)
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if strings.EqualFold(report.Decision, "Deny") || strings.EqualFold(report.Phase, "Failed") || strings.EqualFold(report.Phase, "Expired") || strings.EqualFold(report.Phase, "Cancelled") {
		return 1
	}
	return 0
}

func printWork(stdout, stderr io.Writer, parsed parsedArgs, services Services) int {
	if len(parsed.operands) == 1 && parsed.operands[0] == "list" && services.ListConnected != nil {
		views, err := services.ListConnected(parsed.stateDir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if parsed.json {
			return printJSON(stdout, stderr, views)
		}
		if len(views) == 0 {
			fmt.Fprintln(stdout, "No Work recorded in the current service session.")
			return 0
		}
		for _, view := range views {
			fmt.Fprintf(stdout, "%s\t%s\n", view.RequestRef, workPhase(view))
		}
		return 0
	}
	if len(parsed.operands) == 2 && parsed.operands[0] == "show" && services.ShowConnected != nil {
		view, err := services.ShowConnected(parsed.operands[1], parsed.stateDir)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if parsed.json {
			return printJSON(stdout, stderr, view)
		}
		fmt.Fprintf(stdout, "request: %s\nphase: %s\n", view.RequestRef, workPhase(view))
		if view.State != nil {
			fmt.Fprintf(stdout, "decision: %s\n", view.State.Decision.Result)
			if view.State.Claim != nil {
				fmt.Fprintf(stdout, "claim: %s\n", view.State.Claim.ID)
			}
		}
		if view.Outcome != nil {
			if view.Outcome.Failure != "" {
				fmt.Fprintf(stdout, "failure: %s\n", view.Outcome.Failure)
			}
			if view.Outcome.Text != "" {
				fmt.Fprintf(stdout, "result: %s\n", view.Outcome.Text)
			}
		}
		return 0
	}
	fmt.Fprint(stderr, workHelpText)
	return ExitUsage
}

func workPhase(view evidence.View) string {
	if view.Outcome != nil {
		return view.Outcome.Status
	}
	if view.State != nil {
		if view.State.Decision.Result == "Deny" {
			return "Denied"
		}
		if view.State.Claim != nil {
			return string(view.State.Claim.Phase)
		}
	}
	return "Pending"
}

func printRegistration(stdout, stderr io.Writer, parsed parsedArgs, factory RegistrationServiceFactory) int {
	if len(parsed.operands) != 1 || parsed.operands[0] != "apply" || !parsed.fileSet || strings.TrimSpace(parsed.file) == "" {
		fmt.Fprintf(stderr, "Usage: agenova %s apply -f <document.yaml> [--json]\n", parsed.command)
		return ExitUsage
	}
	if factory == nil {
		fmt.Fprintln(stderr, "registration service is not configured")
		return 1
	}
	service, err := factory(parsed.stateDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var result registration.Result
	if parsed.command == "policy" {
		result, err = service.ApplyPolicyFile(parsed.file)
	} else {
		result, err = service.ApplyTemplateFile(parsed.file)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if parsed.json {
		return printJSON(stdout, stderr, result)
	}
	status := "already registered"
	if result.Changed {
		status = "registered"
	}
	fmt.Fprintf(stdout, "%s: %s %s", status, result.Kind, result.Name)
	if result.Version != "" {
		fmt.Fprintf(stdout, "@%s", result.Version)
	}
	if result.Active {
		fmt.Fprint(stdout, " (active)")
	}
	fmt.Fprintln(stdout)
	return 0
}

func printPlatform(stdout, stderr io.Writer, parsed parsedArgs, services Services) int {
	if services.NewPlatform == nil {
		fmt.Fprintln(stderr, "platform service is not configured")
		return 1
	}
	if len(parsed.operands) != 1 {
		fmt.Fprint(stderr, platformHelpText)
		return ExitUsage
	}
	subcommand := parsed.operands[0]
	if subcommand == "status" {
		if parsed.fileSet {
			return platformUsageError(stderr, "platform status reads the last applied revision; do not pass -f")
		}
	} else if !parsed.fileSet || strings.TrimSpace(parsed.file) == "" {
		return platformUsageError(stderr, "platform command requires -f <platform.yaml>")
	}
	if parsed.yes && subcommand != "apply" {
		return platformUsageError(stderr, "--yes is only valid with platform apply")
	}
	service, err := services.NewPlatform(parsed.stateDir)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	ctx := context.Background()
	switch subcommand {
	case "status":
		plan, err := service.Status(ctx)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		return printPlatformStatus(stdout, stderr, plan, parsed.json)
	case "validate":
		resolved, _, err := service.ValidateFile(parsed.file)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		result := struct {
			Valid        bool   `json:"valid"`
			PlatformName string `json:"platformName"`
			Revision     string `json:"revision"`
		}{true, resolved.PlatformName, resolved.Revision}
		if parsed.json {
			return printJSON(stdout, stderr, result)
		}
		fmt.Fprintf(stdout, "valid: %s\nrevision: %s\n", resolved.PlatformName, resolved.Revision)
		return 0
	case "plan":
		plan, err := service.PlanFile(ctx, parsed.file)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		return printPlatformPlan(stdout, stderr, plan, parsed.json)
	case "apply":
		resolved, lock, err := service.ValidateFile(parsed.file)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		plan, err := service.Plan(ctx, resolved, lock)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		if plan.Changed() && !parsed.yes {
			fmt.Fprintf(stderr, "Apply %d planned change(s) to %s? [y/N] ", len(plan.Changes), plan.Target)
			input := services.Input
			if input == nil {
				input = strings.NewReader("")
			}
			answer, _ := bufio.NewReader(input).ReadString('\n')
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer != "y" && answer != "yes" {
				fmt.Fprintln(stderr, "apply cancelled")
				return 1
			}
		}
		result, err := service.ApplyPlanned(ctx, resolved, lock, plan)
		if err != nil {
			if parsed.json {
				_ = json.NewEncoder(stdout).Encode(result)
			} else if result.Plan.PlatformName != "" {
				printPlatformApplyHuman(stdout, result)
			}
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		if result.Ready {
			if err := service.Remember(resolved, lock); err != nil {
				fmt.Fprintf(stderr, "Platform applied but local discovery state could not be saved: %v\n", err)
				return 1
			}
		}
		if parsed.json {
			return printJSON(stdout, stderr, result)
		}
		printPlatformApplyHuman(stdout, result)
		return 0
	default:
		return platformUsageError(stderr, fmt.Sprintf("unknown platform command %q", subcommand))
	}
}

func printPlatformApplyHuman(output io.Writer, result platformapply.ApplyResult) {
	fmt.Fprintf(output, "platform: %s\nrevision: %s\ntarget: %s\nchanged: %t\nready: %t\nreadiness scope: %s\n", result.Plan.PlatformName, result.Plan.Revision, result.Plan.Target, result.Applied, result.Ready, result.ReadinessScope)
	for _, component := range result.Components {
		fmt.Fprintf(output, "- %s/%s: %s\n", component.Category, component.Name, component.State)
	}
}

// Status is an observation of installed components. It does not probe model
// providers or establish that a Work can run end to end.
func printPlatformStatus(stdout, stderr io.Writer, plan platformapply.Plan, jsonOutput bool) int {
	installationReady := !plan.Changed()
	for _, component := range plan.Components {
		switch component.State {
		case "available", "configured", "used":
		default:
			installationReady = false
		}
	}
	if jsonOutput {
		return printJSON(stdout, stderr, struct {
			platformapply.Plan
			InstallationReady bool   `json:"installationReady"`
			ReadinessScope    string `json:"readinessScope"`
			ProviderHealth    string `json:"providerHealth"`
			API               struct {
				LocalEndpointAfterConnect string `json:"localEndpointAfterConnect"`
				ConnectCommand            string `json:"connectCommand"`
				ConnectionState           string `json:"connectionState"`
			} `json:"api"`
		}{Plan: plan, InstallationReady: installationReady, ReadinessScope: platformapply.ReadinessScopeInstallation, ProviderHealth: "not-checked", API: struct {
			LocalEndpointAfterConnect string `json:"localEndpointAfterConnect"`
			ConnectCommand            string `json:"connectCommand"`
			ConnectionState           string `json:"connectionState"`
		}{"http://127.0.0.1:8088", "agenova api connect", "not-checked"}})
	}
	if code := printPlatformPlan(stdout, stderr, plan, false); code != 0 {
		return code
	}
	fmt.Fprintf(stdout, "installation ready: %t\nreadiness scope: %s\nprovider health: not checked\n", installationReady, platformapply.ReadinessScopeInstallation)
	fmt.Fprintln(stdout, "api after local connect: http://127.0.0.1:8088")
	fmt.Fprintln(stdout, "connect: agenova api connect (separate terminal; connection not checked)")
	return 0
}

func printPlatformPlan(stdout, stderr io.Writer, plan platformapply.Plan, jsonOutput bool) int {
	if jsonOutput {
		return printJSON(stdout, stderr, plan)
	}
	fmt.Fprintf(stdout, "platform: %s\nrevision: %s\ntarget: %s\nchanges: %d\n", plan.PlatformName, plan.Revision, plan.Target, len(plan.Changes))
	for _, change := range plan.Changes {
		fmt.Fprintf(stdout, "- %s %s: %s\n", change.Action, change.Component, change.Detail)
	}
	for _, component := range plan.Components {
		if component.Reference == "" {
			fmt.Fprintf(stdout, "- %s/%s: %s\n", component.Category, component.Name, component.State)
		} else {
			fmt.Fprintf(stdout, "- %s/%s: %s (%s)\n", component.Category, component.Name, component.State, component.Reference)
		}
	}
	return 0
}

func platformUsageError(stderr io.Writer, message string) int {
	fmt.Fprintln(stderr, message)
	fmt.Fprintln(stderr, "Run 'agenova platform --help' for usage.")
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
		if report.RequestRef != "" || report.ClaimID != "" || report.Phase != "" {
			if outputErr := printRunReport(stdout, report, parsed.json); outputErr != nil {
				fmt.Fprintln(stderr, outputErr)
				return 1
			}
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintln(stderr, err.Error())
		fmt.Fprintln(stderr, "Run 'agenova run --help' for usage.")
		return ExitUsage
	}
	if outputErr := printRunReport(stdout, report, parsed.json); outputErr != nil {
		fmt.Fprintln(stderr, outputErr)
		return 1
	}
	if strings.EqualFold(report.Decision, "Deny") {
		return 1
	}
	return 0
}

func printRunReport(stdout io.Writer, report RunReport, jsonOutput bool) error {
	if jsonOutput {
		if report.Evidence == nil {
			return fmt.Errorf("shared evidence view is unavailable")
		}
		return json.NewEncoder(stdout).Encode(report.Evidence)
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
	if report.Evidence != nil && report.Evidence.Outcome != nil {
		if report.Evidence.Outcome.Failure != "" {
			fmt.Fprintf(stdout, "failure: %s\n", report.Evidence.Outcome.Failure)
		}
		if report.Evidence.Outcome.Text != "" {
			fmt.Fprintf(stdout, "result: %s\n", report.Evidence.Outcome.Text)
		}
		if report.Evidence.Outcome.Model != nil {
			fmt.Fprintf(stdout, "model tokens: %d in / %d out\n", report.Evidence.Outcome.Model.InputTokens, report.Evidence.Outcome.Model.OutputTokens)
		}
	}
	return nil
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

func printAdapters(stdout, stderr io.Writer, parsed parsedArgs, factory AdapterLifecycleFactory) int {
	if factory == nil {
		fmt.Fprintln(stderr, "adapter lifecycle is not configured")
		return 1
	}
	if len(parsed.operands) == 0 {
		fmt.Fprint(stderr, adaptersHelpText)
		return ExitUsage
	}
	subcommand := parsed.operands[0]
	arguments := parsed.operands[1:]
	if parsed.nameSet && subcommand != "init" {
		return adapterUsageError(stderr, "--name is only valid with agenova adapters init")
	}
	lifecycle, err := factory(parsed.stateDir)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	switch subcommand {
	case "catalog":
		if len(arguments) != 0 {
			return adapterUsageError(stderr, "catalog accepts no arguments")
		}
		catalog := lifecycle.Catalog()
		if parsed.json {
			return printJSON(stdout, stderr, struct {
				Adapters []adapterregistry.Manifest `json:"adapters"`
			}{Adapters: catalog})
		}
		for _, manifest := range catalog {
			fmt.Fprintf(stdout, "%s@%s\t%s\n", manifest.ID, manifest.Version, joinCapabilities(manifest.Capabilities))
		}
		return 0
	case "list":
		if len(arguments) != 0 {
			return adapterUsageError(stderr, "list accepts no arguments")
		}
		lock, err := lifecycle.List()
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		if parsed.json {
			return printJSON(stdout, stderr, lock)
		}
		if len(lock.Adapters) == 0 {
			fmt.Fprintln(stdout, "No adapters are activated for this installation.")
			return 0
		}
		for _, adapter := range lock.Adapters {
			fmt.Fprintf(stdout, "%s@%s\t%s\n", adapter.ID, adapter.Version, joinCapabilities(adapter.Capabilities))
		}
		return 0
	case "inspect":
		if len(arguments) != 1 {
			return adapterUsageError(stderr, "inspect requires <id[@version]>")
		}
		result, err := lifecycle.Inspect(arguments[0])
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return ExitUsage
		}
		if parsed.json {
			return printJSON(stdout, stderr, result)
		}
		fmt.Fprintf(stdout, "adapter: %s@%s\nprotocol: %s\ncapabilities: %s\ninstalled: %t\n", result.Manifest.ID, result.Manifest.Version, result.Manifest.Protocol, joinCapabilities(result.Manifest.Capabilities), result.Installed)
		return 0
	case "install":
		if len(arguments) != 1 {
			return adapterUsageError(stderr, "install requires <id[@version]>")
		}
		result, err := lifecycle.Install(arguments[0])
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		if parsed.json {
			return printJSON(stdout, stderr, result)
		}
		status := "already installed"
		if result.Changed {
			status = "installed"
		}
		fmt.Fprintf(stdout, "%s: %s@%s\n", status, result.Adapter.ID, result.Adapter.Version)
		return 0
	case "init":
		if len(arguments) != 1 {
			return adapterUsageError(stderr, "init requires <id[@version]>")
		}
		fragment, err := lifecycle.Init(arguments[0], parsed.name)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return ExitUsage
		}
		if parsed.json {
			return printJSON(stdout, stderr, fragment)
		}
		data, err := yaml.Marshal(fragment)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		_, _ = stdout.Write(data)
		return 0
	default:
		return adapterUsageError(stderr, fmt.Sprintf("unknown adapters command %q", subcommand))
	}
}

func adapterUsageError(stderr io.Writer, message string) int {
	fmt.Fprintln(stderr, message)
	fmt.Fprintln(stderr, "Run 'agenova adapters --help' for usage.")
	return ExitUsage
}

func printJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

func joinCapabilities(values []platform.Capability) string {
	items := make([]string, len(values))
	for i, value := range values {
		items[i] = string(value)
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}

func parseArgs(argv []string) (parsedArgs, error) {
	parsed := parsedArgs{port: 8088}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case arg == "--help" || arg == "-h":
			parsed.help = true
		case arg == "--version" || arg == "-v":
			parsed.version = true
		case arg == "--json":
			parsed.json = true
		case arg == "--yes":
			parsed.yes = true
		case arg == "--port":
			if i+1 >= len(argv) || looksLikeFlag(argv[i+1]) {
				return parsedArgs{}, fmt.Errorf("flag --port requires a value")
			}
			i++
			if err := setPort(&parsed, argv[i]); err != nil {
				return parsedArgs{}, err
			}
		case strings.HasPrefix(arg, "--port="):
			if err := setPort(&parsed, strings.TrimPrefix(arg, "--port=")); err != nil {
				return parsedArgs{}, err
			}
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
		case arg == "--state-dir":
			if i+1 >= len(argv) || looksLikeFlag(argv[i+1]) {
				return parsedArgs{}, fmt.Errorf("flag --state-dir requires a value")
			}
			i++
			if err := setStateDir(&parsed, argv[i]); err != nil {
				return parsedArgs{}, err
			}
		case strings.HasPrefix(arg, "--state-dir="):
			if err := setStateDir(&parsed, strings.TrimPrefix(arg, "--state-dir=")); err != nil {
				return parsedArgs{}, err
			}
		case arg == "--name":
			if i+1 >= len(argv) || looksLikeFlag(argv[i+1]) {
				return parsedArgs{}, fmt.Errorf("flag --name requires a value")
			}
			i++
			if err := setName(&parsed, argv[i]); err != nil {
				return parsedArgs{}, err
			}
		case strings.HasPrefix(arg, "--name="):
			if err := setName(&parsed, strings.TrimPrefix(arg, "--name=")); err != nil {
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
			if parsed.command == "" {
				parsed.command = arg
			} else {
				parsed.operands = append(parsed.operands, arg)
			}
		}
	}
	if parsed.fileSet && parsed.command != "run" && parsed.command != "platform" && parsed.command != "policy" && parsed.command != "agent-template" && !parsed.help {
		return parsed, fmt.Errorf("-f is only valid with agenova run, platform, policy or agent-template")
	}
	if parsed.json && parsed.command != "run" && parsed.command != "work" && parsed.command != "adapters" && parsed.command != "platform" && parsed.command != "policy" && parsed.command != "agent-template" && !parsed.help {
		return parsed, fmt.Errorf("--json is not valid with agenova %s", parsed.command)
	}
	if parsed.stateSet && parsed.command != "adapters" && parsed.command != "platform" && parsed.command != "policy" && parsed.command != "agent-template" && parsed.command != "run" && parsed.command != "work" && parsed.command != "api" && !parsed.help {
		return parsed, fmt.Errorf("--state-dir is not valid with agenova %s", parsed.command)
	}
	if parsed.nameSet && parsed.command != "adapters" && !parsed.help {
		return parsed, fmt.Errorf("--name is only valid with agenova adapters init")
	}
	if parsed.backendSet && (parsed.command == "adapters" || parsed.command == "platform" || parsed.command == "policy" || parsed.command == "agent-template" || parsed.command == "work" || parsed.command == "api") && !parsed.help {
		return parsed, fmt.Errorf("--backend is not valid with agenova %s", parsed.command)
	}
	if parsed.yes && parsed.command != "platform" && !parsed.help {
		return parsed, fmt.Errorf("--yes is only valid with agenova platform apply")
	}
	if parsed.portSet && (parsed.command != "api" || len(parsed.operands) != 1 || parsed.operands[0] != "connect") && !parsed.help {
		return parsed, fmt.Errorf("--port is only valid with agenova api connect")
	}
	if parsed.command != "adapters" && parsed.command != "platform" && parsed.command != "policy" && parsed.command != "agent-template" && parsed.command != "work" && parsed.command != "api" && len(parsed.operands) > 0 && !parsed.help {
		return parsed, fmt.Errorf("unexpected argument %q", parsed.operands[0])
	}
	return parsed, nil
}

func setPort(parsed *parsedArgs, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("--port must be an integer from 1024 to 65535")
	}
	parsed.port = port
	parsed.portSet = true
	return nil
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

func setStateDir(parsed *parsedArgs, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("flag --state-dir requires a value")
	}
	parsed.stateDir = value
	parsed.stateSet = true
	return nil
}

func setName(parsed *parsedArgs, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("flag --name requires a value")
	}
	parsed.name = value
	parsed.nameSet = true
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
