// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"encoding/json"
	"errors"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"io"
	"os"
)

func readTemplateForGuide(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("template document unavailable")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("template document unavailable or too large")
	}
	return data, nil
}

func packageGuide(document []byte, path string) ([]byte, error) {
	if path == "" {
		return document, nil
	}
	template, validationErr := v0.ParseAgentTemplateYAML(document)
	if validationErr != nil {
		return nil, validationErr
	}
	if template.Spec.AccessGuide != nil {
		return nil, errors.New("template already has an access guide; do not replace it implicitly")
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.New("access guide file is unavailable")
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, (32<<10)+1))
	if e != nil {
		return nil, errors.New("access guide file is unavailable")
	}
	guide, e := v0.ParseAccessGuideMarkdown(data)
	if e != nil {
		return nil, e
	}
	template.Spec.AccessGuide = guide
	if e := v0.ValidateAgentTemplate(template); e != nil {
		return nil, e
	}
	return json.Marshal(template)
}
