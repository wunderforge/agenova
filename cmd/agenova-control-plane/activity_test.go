// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/console"
	"testing"
)

type activityAccess struct{}

func (activityAccess) Authorize(context.Context, v0.Principal, string, string) error { return nil }

func TestActivityCaptureIsExplicitAndAuthenticated(t *testing.T) {
	for _, mode := range []string{"", "disabled"} {
		o := console.Options{}
		if err := configureActivityCapture(&o, mode); err != nil || o.ActivityStore != nil {
			t.Fatal("default capture changed", err)
		}
	}
	for _, mode := range []string{"memory", "unknown"} {
		o := console.Options{}
		if err := configureActivityCapture(&o, mode); err == nil || o.ActivityStore != nil {
			t.Fatal("unprotected capture enabled")
		}
	}
	o := console.Options{Access: activityAccess{}}
	if err := configureActivityCapture(&o, "memory"); err != nil || o.ActivityStore == nil {
		t.Fatal("authenticated opt-in was not wired", err)
	}
}
