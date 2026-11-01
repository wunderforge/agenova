// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package workerprotocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRawChoiceDiagnosticsAndSchemaDoNotAcceptWrappers(t *testing.T) {
	tool := Tool{Operation: "crm.read", ResourceScope: "crm:customer-retention", Description: "customers", Parameter: "dataset_id", AllowedValues: []string{"customer-retention"}}
	bad := `{"action":"tool","tool":"crm.read","resource":"crm:customer-retention","input":"dataset_id=customer-retention"}`
	if _, e := ParseAgentAction(bad, []Tool{tool}, DiagnosticsProtocol); e == nil {
		t.Fatal("wrapper accepted")
	}
	_, reason := AgentActionIssue(bad, []Tool{tool}, DiagnosticsProtocol)
	if !strings.Contains(reason, "Invalid input for crm.read") || !strings.Contains(ToolInputRepair(bad, []Tool{tool}), `"input":"customer-retention"`) {
		t.Fatal("missing exact correction")
	}
	for _, text := range []string{strings.Replace(bad, "crm.read", "unknown.read", 1), strings.Replace(bad, "crm:customer-retention", "foreign:scope", 1)} {
		if ToolInputRepair(text, []Tool{tool}) != "" {
			t.Fatal("invented a route for unknown tool/scope")
		}
	}
	raw, _ := DiagnosticsSchema([]Tool{tool}, false)
	schema, e := GatewayResearchSchema(raw, []Tool{tool})
	if e != nil {
		t.Fatal(e)
	}
	var out researchVariant
	out = researchVariant{}
	json.Unmarshal(schema, &out)
	if !containsSchemaValue(out.Properties.Input.Enum, "customer-retention") || containsSchemaValue(out.Properties.Input.Enum, "dataset_id=customer-retention") {
		t.Fatal("fixed input grammar lost enum")
	}
	mixed := []Tool{tool, {Operation: "web.search", ResourceScope: "web:public-pages", Description: "search", Parameter: "query", InputMode: "text", MaxBytes: 512}}
	raw, _ = DiagnosticsSchema(mixed, false)
	schema, e = GatewayResearchSchema(raw, mixed)
	if e != nil {
		t.Fatal(e)
	}
	out = researchVariant{}
	json.Unmarshal(schema, &out)
	if len(out.Properties.Input.Enum) != 0 || !strings.Contains(out.Properties.Input.Description, "crm.read") || !strings.Contains(out.Properties.Input.Description, "customer-retention") {
		t.Fatal("mixed grammar either restricted web text or lost raw choice guidance")
	}
	if _, e = ParseAgentAction(`{"action":"tool","tool":"web.search","resource":"web:public-pages","input":"weather Sydney"}`, mixed, DiagnosticsProtocol); e != nil {
		t.Fatal("free text regressed", e)
	}
}

func TestLargeFixedCatalogRetainsBoundedSchemaFallback(t *testing.T) {
	tool := Tool{Operation: "crm.read", ResourceScope: "crm:customers", Description: "synthetic datasets", Parameter: "dataset_id"}
	for i := 0; i < 64; i++ {
		tool.AllowedValues = append(tool.AllowedValues, fmt.Sprintf("dataset-%02d-%s", i, strings.Repeat("a", 180)))
	}
	raw, err := DiagnosticsSchema([]Tool{tool}, false)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := GatewayResearchSchema(raw, []Tool{tool})
	if err != nil || len(schema) > MaxSchemaBytes {
		t.Fatalf("large supported catalog regressed: %v", err)
	}
	if _, err := ParseAgentAction(`{"action":"tool","tool":"crm.read","resource":"crm:customers","input":"not-a-catalog-value"}`, []Tool{tool}, DiagnosticsProtocol); err == nil {
		t.Fatal("schema fallback weakened exact validation")
	}
}
