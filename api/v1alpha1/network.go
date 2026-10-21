// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "fmt"

// NetworkProfilePublicWeb permits public TCP 443 and necessary DNS when the
// runtime supports enforcement. It is neither a domain nor an HTTP-method cap.
const NetworkProfilePublicWeb = "public-web"

func validateNetworkProfiles(path string, profiles []string, category ValidationCategory) *ValidationError {
	seen := make(map[string]struct{}, len(profiles))
	for index, profile := range profiles {
		field := fmt.Sprintf("%s[%d]", path, index)
		if profile != NetworkProfilePublicWeb {
			return validationError(category, field, "unsupported network profile")
		}
		if _, duplicate := seen[profile]; duplicate {
			return validationError(category, field, "duplicate network profile")
		}
		seen[profile] = struct{}{}
	}
	return nil
}
