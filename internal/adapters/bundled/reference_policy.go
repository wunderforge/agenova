// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package bundled

import (
	"fmt"

	v1alpha1 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/policy"
)

// ReferencePolicyCatalog is the deliberately small catalog shipped with the
// reference distribution. The platform service itself has no policy IDs.
type ReferencePolicyCatalog struct{}

func (ReferencePolicyCatalog) Require(ref v1alpha1.PlatformPolicyReference) error {
	bundle := policy.ReferenceBundle()
	if ref.ID != bundle.ID || ref.Version != bundle.Version {
		return fmt.Errorf("%s@%s is not available; reference install provides %s@%s", ref.ID, ref.Version, bundle.ID, bundle.Version)
	}
	return nil
}
