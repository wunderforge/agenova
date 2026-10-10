// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package authority

import (
	"reflect"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
)

func TestMemoryOperationIntersectionAndProofImmutability(t *testing.T) {
	request, template, _ := fixtures(t)
	request.Spec.RequestedAccess.MemoryOperations = []string{v0.MemoryWrite, v0.MemoryRead}
	template.Spec.CapabilityCeiling.MemoryOperations = []string{v0.MemoryRead}
	resolution, err := ResolveForIssuance(request, template, admit(t, request))
	if err != nil {
		t.Fatal(err)
	}
	grant, ok := resolution.AuthorityFor(request)
	if !ok || !reflect.DeepEqual(grant.MemoryOperations, []string{v0.MemoryRead}) {
		t.Fatalf("operations expanded: %+v", grant)
	}
	changes, ok := resolution.ChangesFor(request)
	if !ok || len(changes) != 1 || changes[0].Field != "memoryOperations" || changes[0].Requested != v0.MemoryWrite {
		t.Fatalf("missing narrowing evidence: %+v", changes)
	}
	grant.MemoryOperations[0] = v0.MemoryWrite
	again, ok := resolution.AuthorityFor(request)
	if !ok || again.MemoryOperations[0] != v0.MemoryRead {
		t.Fatal("public grant mutation changed proof")
	}
	request.Spec.RequestedAccess.MemoryOperations = []string{v0.MemoryRead}
	if _, ok := resolution.AuthorityFor(request); ok {
		t.Fatal("proof accepted changed operation intent")
	}
}

func TestMemoryOperationsNeedExplicitRequestCeilingAndScope(t *testing.T) {
	for _, name := range []string{"omitted request", "omitted ceiling", "omitted scope"} {
		t.Run(name, func(t *testing.T) {
			request, template, _ := fixtures(t)
			request.Spec.RequestedAccess.MemoryOperations = []string{v0.MemoryRead}
			template.Spec.CapabilityCeiling.MemoryOperations = []string{v0.MemoryRead}
			switch name {
			case "omitted request":
				request.Spec.RequestedAccess.MemoryOperations = nil
			case "omitted ceiling":
				template.Spec.CapabilityCeiling.MemoryOperations = nil
			case "omitted scope":
				request.Spec.RequestedAccess.MemoryScopes = nil
			}
			grant, err := Resolve(request, template, admit(t, request))
			if name == "omitted request" {
				if err != nil || len(grant.MemoryOperations) != 0 {
					t.Fatalf("legacy scopes gained operations: %+v, %v", grant, err)
				}
			} else if err == nil || grant != nil {
				t.Fatal("incomplete explicit memory grant accepted")
			}
		})
	}
}
