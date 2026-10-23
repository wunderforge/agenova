// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"net/url"
	"unicode"

	"github.com/wunderforge/agenova/internal/platform"
)

// Explicit projections: no arbitrary adapter config, headers or credential refs.
type ConnectionInfo struct {
	Category  string `json:"category"`
	Name      string `json:"name"`
	Backend   string `json:"backend,omitempty"`
	Adapter   string `json:"adapter,omitempty"`
	Model     string `json:"model,omitempty"`
	Operation string `json:"operation,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Status    string `json:"status,omitempty"`
}
type PlatformInfo struct {
	Operator    bool             `json:"operator"`
	Connections []ConnectionInfo `json:"connections"`
}

func displayToken(s string) string {
	if len(s) > 256 {
		return ""
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return ""
		}
	}
	return s
}
func connectionEndpoint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return displayToken(u.String())
}
func ProjectPlatform(resolved platform.ResolvedPlatform) *PlatformInfo {
	out := &PlatformInfo{Connections: []ConnectionInfo{}}
	adapters := map[string]string{}
	for _, a := range resolved.Adapters {
		adapters[a.Name] = a.ID + "@" + a.Version
	}
	for _, p := range resolved.Profiles {
		model, _ := p.Config["model"].(string)
		out.Connections = append(out.Connections, ConnectionInfo{Category: string(p.Capability), Name: displayToken(p.Name), Backend: displayToken(p.BackendRef), Model: displayToken(model)})
	}
	for _, r := range resolved.ToolRoutes {
		out.Connections = append(out.Connections, ConnectionInfo{Category: "tool", Name: displayToken(r.Profile), Backend: displayToken(r.BackendRef), Operation: displayToken(r.Tool.Operation), Scope: displayToken(r.Tool.ResourceScope)})
	}
	for _, i := range resolved.Instances {
		endpoint, _ := i.Config["endpoint"].(string)
		out.Connections = append(out.Connections, ConnectionInfo{Category: string(i.Category), Name: displayToken(i.Name), Adapter: displayToken(adapters[i.AdapterRef]), Endpoint: connectionEndpoint(endpoint)})
	}
	return out
}

// Withhold installation topology entirely for non-operators. Their template
// ceiling and Policy are already independently permission-filtered by setup.
func ScopePlatform(view *Setup, operator bool) {
	if !operator && view.Observability != nil {
		o := *view.Observability
		o.Endpoint = ""
		o.DiagnosticURL = ""
		view.Observability = &o
	}
	if view.PlatformInfo == nil {
		return
	}
	copy := *view.PlatformInfo
	copy.Operator = operator
	copy.Connections = append([]ConnectionInfo{}, copy.Connections...)
	if !operator {
		copy.Connections = []ConnectionInfo{}
	}
	view.PlatformInfo = &copy
}
