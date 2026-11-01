// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"time"
)

// AccessAdvisorConfig configures optional pre-claim assistance, not authority.
type AccessAdvisorConfig struct {
	Enabled       bool     `json:"enabled" yaml:"enabled"`
	AllowedModes  []string `json:"allowedModes,omitempty" yaml:"allowedModes,omitempty"`
	DefaultMode   string   `json:"defaultMode,omitempty" yaml:"defaultMode,omitempty"`
	Engine        string   `json:"engine,omitempty" yaml:"engine,omitempty"`
	ModelProfile  string   `json:"modelProfile,omitempty" yaml:"modelProfile,omitempty"`
	MaxCalls      int      `json:"maxCalls,omitempty" yaml:"maxCalls,omitempty"`
	MaxConcurrent int      `json:"maxConcurrent,omitempty" yaml:"maxConcurrent,omitempty"`
	MaxRecords    int      `json:"maxRecords,omitempty" yaml:"maxRecords,omitempty"`
	Timeout       string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

func NormalizeAccessAdvisor(c *AccessAdvisorConfig) (AccessAdvisorConfig, error) {
	if c == nil || !c.Enabled {
		return AccessAdvisorConfig{AllowedModes: []string{"manual"}, DefaultMode: "manual"}, nil
	}
	n := *c
	n.AllowedModes = append([]string(nil), c.AllowedModes...)
	if len(n.AllowedModes) == 0 {
		n.AllowedModes = []string{"manual", "suggest", "automatic"}
	}
	if n.DefaultMode == "" {
		n.DefaultMode = "manual"
	}
	if n.Engine == "" {
		n.Engine = "rules"
	}
	if n.MaxCalls == 0 {
		n.MaxCalls = 32
	}
	if n.MaxConcurrent == 0 {
		n.MaxConcurrent = 1
	}
	if n.MaxRecords == 0 {
		n.MaxRecords = 128
	}
	if n.Timeout == "" {
		n.Timeout = "30s"
	}
	fail := func() (AccessAdvisorConfig, error) {
		return AccessAdvisorConfig{}, fmt.Errorf("invalid access Advisor configuration")
	}
	if validateUniqueNonBlankList("allowedModes", n.AllowedModes) != nil {
		return fail()
	}
	for _, m := range n.AllowedModes {
		if m != "manual" && m != "suggest" && m != "automatic" {
			return fail()
		}
	}
	if !contains(n.AllowedModes, n.DefaultMode) || (n.Engine != "rules" && n.Engine != "model") || n.Engine == "model" && (n.ModelProfile == "" || len(n.ModelProfile) > 128) || n.Engine == "rules" && n.ModelProfile != "" {
		return fail()
	}
	d, e := time.ParseDuration(n.Timeout)
	if e != nil || d < time.Millisecond || d > time.Minute || n.MaxCalls < 1 || n.MaxCalls > 1000 || n.MaxConcurrent < 1 || n.MaxConcurrent > 4 || n.MaxRecords < 1 || n.MaxRecords > 512 {
		return fail()
	}
	return n, nil
}

func validateAdvisorShape(n *yaml.Node, p string) *ValidationError {
	scalar := func(tag string) nodeValidator {
		return func(v *yaml.Node, path string) *ValidationError {
			if v.Kind != yaml.ScalarNode || v.Tag != tag {
				return validationError(ValidationCategoryInvalidValue, path, "invalid scalar type")
			}
			return nil
		}
	}
	return validateMapping(n, p, map[string]nodeValidator{"enabled": scalar("!!bool"), "allowedModes": validateStringSequence, "defaultMode": validateStringScalar, "engine": validateStringScalar, "modelProfile": validateStringScalar, "maxCalls": scalar("!!int"), "maxConcurrent": scalar("!!int"), "maxRecords": scalar("!!int"), "timeout": validateStringScalar}, nil)
}
