// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"gopkg.in/yaml.v3"
	"strconv"
)

const DefaultModelCalls = 6

func ModelCallLimit(value int) int {
	if value == 0 {
		return DefaultModelCalls
	}
	return value
}

func validateModelCallsScalar(node *yaml.Node, path string) *ValidationError {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return validationError(ValidationCategoryInvalidValue, path, "expected a positive model-call count")
	}
	n, err := strconv.Atoi(node.Value)
	if err != nil || n < 1 || n > 1000 {
		return validationError(ValidationCategoryInvalidValue, path, "model-call count must be between 1 and 1000")
	}
	return nil
}
