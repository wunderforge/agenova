// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Shared v0 contract-validation primitives live in agent_template.go, which
// merged first (E1-T2). Per the fold agreement recorded on that contract, the
// second contract folds its copy into that single source rather than keeping a
// parallel definition. This file holds only what the shared source still
// lacks: the Duration serializers a declarative contract needs to round-trip.

import (
	"encoding/json"
	"time"

	"gopkg.in/yaml.v3"
)

// MarshalYAML re-emits the human-authored Go duration syntax so a serialized
// document stays re-parseable by the same contract. The default int64
// encoding would not be.
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(text)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

var _ yaml.Marshaler = Duration(0)
