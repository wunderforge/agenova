// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentAccessGuide is author knowledge, never a policy or grant. It is packaged
// in an immutable template; no worker or server follows document URLs at runtime.
type AgentAccessGuide struct {
	Version  string              `json:"version" yaml:"version"`
	Markdown string              `json:"markdown,omitempty" yaml:"markdown,omitempty"`
	Profiles []TaskAccessProfile `json:"profiles" yaml:"profiles"`
}

type TaskAccessProfile struct {
	ID              string               `json:"id" yaml:"id"`
	TaskType        string               `json:"taskType" yaml:"taskType"`
	Description     string               `json:"description" yaml:"description"`
	RequestedAccess ClaimRequestedAccess `json:"requestedAccess" yaml:"requestedAccess"`
}

func ValidateAccessGuide(t *AgentTemplate) *ValidationError {
	if t == nil || t.Spec.AccessGuide == nil {
		return nil
	}
	g := t.Spec.AccessGuide
	bad := func(message string) *ValidationError {
		return validationError(ValidationCategoryInvalidValue, "spec.accessGuide", message)
	}
	if strings.TrimSpace(g.Version) == "" || len(g.Version) > 64 || len(g.Markdown) > 16<<10 || len(g.Profiles) < 1 || len(g.Profiles) > 16 || t.Spec.CapabilityCeiling == nil {
		return bad("guide version, size or profiles are invalid")
	}
	seen := map[string]bool{}
	c := t.Spec.CapabilityCeiling
	for _, p := range g.Profiles {
		if !platformLocalName.MatchString(p.ID) || seen[p.ID] || strings.TrimSpace(p.TaskType) == "" || len(p.TaskType) > 128 || strings.TrimSpace(p.Description) == "" || len(p.Description) > 1024 {
			return bad("profile identity or description is invalid")
		}
		seen[p.ID] = true
		a := p.RequestedAccess
		for _, s := range []struct {
			want, ceiling []string
			resource      bool
		}{{a.Tools, c.Tools, false}, {a.ResourceScopes, c.ResourceScopes, true}, {a.MemoryScopes, c.MemoryScopes, false}, {a.NetworkProfiles, c.NetworkProfiles, false}} {
			if len(s.want) > 64 || validateRequestList("spec.accessGuide", s.want) != nil {
				return bad("profile list is invalid")
			}
			for _, v := range s.want {
				within := contains(s.ceiling, v)
				if s.resource {
					if strings.Contains(v, "*") {
						return bad("profile resources must be concrete")
					}
					for _, limit := range s.ceiling {
						within = within || strings.Count(limit, "*") == 1 && strings.HasSuffix(limit, "*") && limit != "*" && strings.HasPrefix(v, strings.TrimSuffix(limit, "*"))
					}
				}
				if !within {
					return bad("profile exceeds template ceiling")
				}
			}
		}
		if a.ModelProfile != "" && !contains(c.ModelProfiles, a.ModelProfile) {
			return bad("profile model exceeds template ceiling")
		}
	}
	return nil
}

func AccessGuideDigest(t *AgentTemplate) string {
	if t == nil || t.Spec.AccessGuide == nil {
		return ""
	}
	// Include the complete template, so ceiling/default/image drift invalidates a draft.
	b, _ := json.Marshal(t)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ParseAccessGuideMarkdown accepts bounded YAML frontmatter and a prose body.
// It parses knowledge only and does not execute Skills or read referenced files.
func ParseAccessGuideMarkdown(data []byte) (*AgentAccessGuide, error) {
	if len(data) > 32<<10 {
		return nil, fmt.Errorf("access guide exceeds size limit")
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, fmt.Errorf("access guide requires YAML frontmatter")
	}
	split := bytes.Index(data[4:], []byte("\n---\n"))
	if split < 0 {
		return nil, fmt.Errorf("access guide frontmatter is not closed")
	}
	end := 4 + split
	var node yaml.Node
	if yaml.Unmarshal(data[4:end], &node) != nil || len(node.Content) != 1 {
		return nil, fmt.Errorf("invalid guide metadata")
	}
	// Only version and structured profiles belong in author frontmatter.
	if e := validateMapping(node.Content[0], "guide", map[string]nodeValidator{"version": validateStringScalar, "profiles": validateAccessProfilesShape}, nil); e != nil {
		return nil, e
	}
	var g AgentAccessGuide
	if node.Content[0].Decode(&g) != nil {
		return nil, fmt.Errorf("invalid guide metadata")
	}
	g.Markdown = strings.TrimSpace(string(data[end+5:]))
	return &g, nil
}

func validateAccessGuideShape(n *yaml.Node, p string) *ValidationError {
	return validateMapping(n, p, map[string]nodeValidator{"version": validateStringScalar, "markdown": validateStringScalar, "profiles": validateAccessProfilesShape}, nil)
}
func validateAccessProfilesShape(n *yaml.Node, p string) *ValidationError {
	if n.Kind != yaml.SequenceNode {
		return validationError(ValidationCategoryInvalidValue, p, "expected profile sequence")
	}
	for i, v := range n.Content {
		if e := validateMapping(v, fmt.Sprintf("%s[%d]", p, i), map[string]nodeValidator{"id": validateStringScalar, "taskType": validateStringScalar, "description": validateStringScalar, "requestedAccess": validateClaimAccessShape}, nil); e != nil {
			return e
		}
	}
	return nil
}
