// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"errors"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/facts"
	"github.com/wunderforge/agenova/internal/runtime"
)

func TestIndependentImageFailuresHaveSafeSharedEvidence(t *testing.T) {
	for _, tc := range []struct {
		cause error
		code  string
	}{
		{runtime.ErrWorkerControlUnavailable, "runtime-worker-control-unavailable"},
		{runtime.ErrWorkerProtocol, "runtime-worker-protocol"},
		{runtime.ErrWorkerImageUnavailable, "runtime-image-unavailable"},
	} {
		code, reason := runFailure(errors.Join(tc.cause, errors.New("private-image-detail")), nil)
		if code != tc.code || strings.Contains(reason, "private-image") {
			t.Fatalf("%s: %s", code, reason)
		}
		view := evidence.View{Facts: []facts.Fact{{Kind: "RunOutcome", Operation: "Failed", ReasonCode: code, Reason: reason}}}
		evidence.Decorate(&view)
		if view.Diagnosis == nil || view.Diagnosis.Code != code || view.Diagnosis.Stage != "runtime" {
			t.Fatalf("diagnosis=%+v", view.Diagnosis)
		}
	}
}
