// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGatewayDecoderRetainsFiniteInputsAndGrantIntersection(t *testing.T) {
	for _, wanted := range []Tool{
		{Operation: "crm.read", Description: "Internal customer records", ResourceScope: "crm:customer-retention", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}},
		{Operation: "web.fetch", Description: "Fixed public page", ResourceScope: "web:fixed-page", Parameter: "url", AllowedValues: []string{NativeTarget}},
	} {
		raw, err := ResearchSchema([]Tool{wanted}, false)
		if err != nil {
			t.Fatal(err)
		}
		other := Tool{Operation: "ungranted.read", Description: "Not part of this grant", ResourceScope: "other:private", Parameter: "file", AllowedValues: []string{"private"}}
		constrained, err := GatewayResearchSchema(raw, []Tool{wanted, other})
		if err != nil {
			t.Fatal(err)
		}
		var shape struct {
			AnyOf []researchVariant `json:"anyOf"`
		}
		json.Unmarshal(constrained, &shape)
		call := shape.AnyOf[1].Properties
		if !reflect.DeepEqual(call.Input.Enum, wanted.AllowedValues) || !reflect.DeepEqual(call.Tool.Enum, []string{wanted.Operation}) || !reflect.DeepEqual(call.Resource.Enum, []string{wanted.ResourceScope}) {
			t.Fatalf("lost finite inputs or widened the model catalog: %+v", call)
		}
	}
}

func TestGatewayDecoderSeparatesCallsFromCompletionWithoutOfferingNative(t *testing.T) {
	tools := []Tool{{Operation: "web.read", Description: "Public page", ResourceScope: "web:public-pages", Parameter: "url", InputMode: "text", MaxBytes: 1024}}
	for _, finishing := range []bool{false, true} {
		raw, err := ResearchSchema(tools, finishing)
		if err != nil {
			t.Fatal(err)
		}
		bounded, err := GatewayResearchSchema(raw, tools)
		if err != nil {
			t.Fatal(err)
		}
		var shape struct {
			AnyOf      []researchVariant `json:"anyOf"`
			Properties json.RawMessage   `json:"properties"`
		}
		if json.Unmarshal(bounded, &shape) != nil || shape.Properties != nil {
			t.Fatal("mixed union/properties")
		}
		want := 2
		if finishing {
			want = 1
		}
		if len(shape.AnyOf) != want {
			t.Fatal("wrong call/finish availability")
		}
		for _, v := range shape.AnyOf {
			if len(v.Properties.Action.Enum) != 1 {
				t.Fatal("ambiguous variant")
			}
			switch v.Properties.Action.Enum[0] {
			case "tool":
				if len(v.Properties.Completion.Enum) != 1 || v.Properties.Completion.Enum[0] != "" || v.Properties.Answer.Enum[0] != "" {
					t.Fatal("call permits answer metadata")
				}
			case "finish":
				if v.Properties.Answer.MaxLength != 1200 || v.Properties.Target.MaxLength != 128 {
					t.Fatal("local response budget was not enforced")
				}
				if len(v.Properties.Tool.Enum) != 1 || v.Properties.Tool.Enum[0] != "" || len(v.Properties.Completion.Enum) != 3 {
					t.Fatal("finish permits a call")
				}
			default:
				t.Fatal("native capability was introduced")
			}
		}
	}
	if _, err := GatewayResearchSchema([]byte(`{"type":"string"}`), nil); err == nil {
		t.Fatal("non-Researcher schema accepted")
	}
}
