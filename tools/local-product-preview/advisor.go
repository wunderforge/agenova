// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/accessadvisor"
	"github.com/wunderforge/agenova/internal/app"
	identitydb "github.com/wunderforge/agenova/internal/authentication/postgres"
	"github.com/wunderforge/agenova/internal/modelprovider"
	"github.com/wunderforge/agenova/internal/policy"
	"io"
	"os"
	"time"
)

func localAdvisor(configPath, model string) (*accessadvisor.Service, error) {
	if configPath == "" {
		return nil, nil
	}
	data, e := os.ReadFile(configPath)
	if e != nil || len(data) > 4096 {
		return nil, errors.New("local Advisor configuration unavailable or oversized")
	}
	var cfg v0.AccessAdvisorConfig
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid local Advisor configuration")
	}
	cfg, e = v0.NormalizeAccessAdvisor(&cfg)
	if e != nil {
		return nil, e
	}
	if cfg.Engine == "rules" {
		return accessadvisor.Configured(&cfg, nil)
	}
	provider, e := newLocalAdvisorProvider(cfg.ModelProfile, model)
	if e != nil {
		return nil, e
	}
	return accessadvisor.Configured(&cfg, provider)
}

func seedAdvisor(ctx context.Context, store identitydb.Store, p v0.Principal) error {
	data, e := os.ReadFile("harness/examples/access-advisor/researcher.yaml")
	if e != nil {
		return e
	}
	template, validationErr := v0.ParseAgentTemplateYAML(data)
	if validationErr != nil {
		return validationErr
	}
	guide, e := os.ReadFile("harness/examples/access-advisor/ACCESS.md")
	if e != nil {
		return e
	}
	template.Spec.AccessGuide, e = v0.ParseAccessGuideMarkdown(guide)
	if e != nil {
		return e
	}
	if err := v0.ValidateAgentTemplate(template); err != nil {
		return err
	}
	registry := identitydb.ProjectRegistry{Store: store, Team: p.Team, Project: "research"}
	if old, err := registry.Template(template.Metadata.Name); err == nil {
		if v0.AccessGuideDigest(old) != v0.AccessGuideDigest(template) {
			return errors.New("local guide changed; register a new immutable template identity")
		}
		return nil
	}
	raw, _ := json.Marshal(template)
	if _, e = store.ApplyGovernance(ctx, p, "research", "template", raw, app.ValidateControlledTemplate); e != nil {
		return e
	}
	bundle, e := registry.ActivePolicy()
	if e != nil {
		return e
	}
	expected := &v0.PolicyReference{ID: bundle.ID, Version: bundle.Version}
	bundle.Version = "advisor-local-1"
	bundle.Rules = append(bundle.Rules, policy.Rule{Team: p.Team, Project: "research", TemplateRef: template.Metadata.Name, Action: "claim.create", Constraints: &policy.AuthorityConstraints{NetworkProfiles: &policy.StringSetConstraint{Values: []string{}}}})
	document := policy.Document{APIVersion: policy.DocumentAPIVersion, Kind: policy.DocumentKind}
	document.Metadata.Name = bundle.ID
	document.Metadata.Version = bundle.Version
	document.Spec.Rules = bundle.Rules
	doc, e := json.Marshal(document)
	if e != nil {
		return e
	}
	_, e = store.ApplyGovernance(ctx, p, "research", "policy", doc, nil, expected)
	return e
}

func newLocalAdvisorProvider(profile, model string) (modelprovider.Client, error) {
	return modelprovider.New(modelprovider.Config{Endpoint: "http://127.0.0.1:11434/v1", Models: map[string]string{profile: model}, MaxTokens: 256, Timeout: time.Minute})
}
