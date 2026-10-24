// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package connectedclient

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	v0 "github.com/wunderforge/agenova/api/v1alpha1"
	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

func nativeEvidenceFixture() string {
	base := installedEvidenceJSON("demo", "Succeeded")
	base = strings.Replace(base, `"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, `"requestedAccess":{"modelProfile":"coding-standard"},"runtime":{"profileRef":"standard-isolated","timeout":"1m"}}},"facts"`, 1)
	base = strings.ReplaceAll(base, `"effectiveAuthority":{"id":"authority:demo"`, `"effectiveAuthority":{"id":"authority:demo","modelProfile":"coding-standard"`)
	base = strings.Replace(base, `"outcome":{"status":"Succeeded"}`, `"outcome":{"status":"Succeeded","text":"Direct HTTPS timed out.","model":{"invocationId":"inv:finish","model":"test-model","inputTokens":1,"outputTokens":1}}`, 1)
	decision := `{"id":"decision","sequence":7,"timestamp":"2026-09-18T00:00:00Z","kind":"ModelDecision","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","policyRef":{"id":"policy:demo","version":"1"},"operation":"model.invoke","target":"coding-standard","result":"Allow"}`
	attempt := `{"id":"attempt","sequence":8,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderAttempt","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"model.invoke","target":"coding-standard","providerStatus":"Attempted"}`
	provider := `{"id":"provider","sequence":9,"timestamp":"2026-09-18T00:00:00Z","kind":"ProviderOutcome","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"model.invoke","target":"coding-standard","providerStatus":"Succeeded"}`
	action := `{"id":"action","sequence":10,"timestamp":"2026-09-18T00:00:00Z","kind":"WorkerActivity","requestRef":"demo","claimId":"claim:demo","invocationId":"inv:demo","operation":"ActionValidated","reasonCode":"agent-action-native"}`
	native := func(id, operation, code, reason string, sequence int) string {
		return fmt.Sprintf(`{"id":%q,"sequence":%d,"timestamp":"2026-09-18T00:00:00Z","kind":"WorkerActivity","requestRef":"demo","claimId":"claim:demo","operation":%q,"target":"direct-https","reasonCode":%q,"reason":%q,"backendIdentity":{"backend":"test-backend","workerId":"worker:demo"}}`, id, sequence, operation, code, reason)
	}
	start := native("native-start", "NativeHTTPSAttempt", "worker-native-attempt", "The original worker session selected a local HTTPS attempt; this record precedes the network request.", 11)
	end := native("native-end", "NativeHTTPSObservation", "worker-native-tcp-timeout", `Worker-reported native HTTPS observation (not independently verified CNI enforcement): {"phase":"tcp","result":"timeout","ip":"172.64.154.22","milliseconds":5000}`, 12)
	terminal := `{"id":"terminal","sequence":13,"timestamp":"2026-09-18T00:00:00Z","kind":"Runtime","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	outcome := `{"id":"outcome","sequence":14,"timestamp":"2026-09-18T00:00:00Z","kind":"RunOutcome","requestRef":"demo","claimId":"claim:demo","operation":"Succeeded"}`
	finish := func(source string) string {
		return strings.Replace(strings.ReplaceAll(source, `"inv:demo"`, `"inv:finish"`), `"id":"`, `"id":"finish:`, 1)
	}
	finishAction := strings.Replace(finish(action), `"agent-action-native"`, `"agent-action-valid"`, 1)
	final := `{"id":"final-answer","sequence":17,"timestamp":"2026-09-18T00:00:00Z","kind":"WorkerActivity","requestRef":"demo","claimId":"claim:demo","operation":"FinalAnswer"}`
	return withTeardown(withEvidenceFacts(base, receivedFact("demo"), resolutionFact("demo", "Allow"), modelAuthorityFact("demo"), boundFact("demo"), backendReadyFact("demo"), runningFact("demo"), decision, attempt, provider, action, start, end, finish(decision), finish(attempt), finish(provider), finishAction, final, terminal, outcome))
}

func TestNativeWorkerEvidenceRequiresBoundedCorrelatedOriginalWorker(t *testing.T) {
	base := nativeEvidenceFixture()
	if _, err := decodeView([]byte(base), "demo"); err != nil {
		t.Fatalf("canonical native evidence rejected: %v", err)
	}
	for name, mutate := range map[string]func(*evidence.View){
		"success without observation or finish": func(v *evidence.View) { v.Facts = append(v.Facts[:11], v.Facts[17:]...); v.Outcome.Model = nil },
		"success without finishing model":       func(v *evidence.View) { v.Facts = append(v.Facts[:12], v.Facts[17:]...); v.Outcome.Model = nil },
		"success without final answer":          func(v *evidence.View) { v.Facts = append(v.Facts[:16], v.Facts[17:]...) },
		"success without outcome model":         func(v *evidence.View) { v.Outcome.Model = nil },
		"nonfinishing final model action":       func(v *evidence.View) { v.Facts[15].ReasonCode = "agent-action-tool" },
		"interleaved successful model action": func(v *evidence.View) {
			insert := append(v.Facts[:0:0], v.Facts[6:10]...)
			for i := range insert {
				insert[i].ID += ":interleaved"
				insert[i].InvocationID = "inv:interleaved"
			}
			insert[3].ReasonCode = "agent-action-tool"
			combined := append(v.Facts[:0:0], v.Facts[:11]...)
			combined = append(combined, insert...)
			v.Facts = append(combined, v.Facts[11:]...)
			for i := range v.Facts {
				v.Facts[i].Sequence = uint64(i + 1)
			}
		},
		"no model selection":          func(v *evidence.View) { v.Facts[9].ReasonCode = "agent-action-tool" },
		"unbound report":              func(v *evidence.View) { v.Facts[11].BackendIdentity = nil },
		"different worker":            func(v *evidence.View) { v.Facts[11].BackendIdentity.WorkerID = "replacement" },
		"observation without attempt": func(v *evidence.View) { v.Facts = append(v.Facts[:10], v.Facts[11:]...) },
		"repeat observation":          func(v *evidence.View) { v.Facts[10].Operation = "NativeHTTPSObservation" },
		"mismatched code":             func(v *evidence.View) { v.Facts[11].ReasonCode = "worker-native-http-received" },
		"claimed CNI verdict":         func(v *evidence.View) { v.Facts[11].Reason = "Verified firewall blocked this packet" },
		"private IP": func(v *evidence.View) {
			v.Facts[11].Reason = strings.Replace(v.Facts[11].Reason, "172.64.154.22", "169.254.169.254", 1)
		},
		"not a grant": func(v *evidence.View) { v.Facts[11].Result = "Allow" },
	} {
		t.Run(name, func(t *testing.T) {
			var v evidence.View
			if err := json.Unmarshal([]byte(base), &v); err != nil {
				t.Fatal(err)
			}
			mutate(&v)
			b, _ := json.Marshal(v)
			if _, err := decodeView(b, "demo"); err == nil {
				t.Fatal("unsafe native evidence accepted")
			}
		})
	}
}

func TestFailedNativeAttemptMayLackObservation(t *testing.T) {
	var v evidence.View
	if err := json.Unmarshal([]byte(nativeEvidenceFixture()), &v); err != nil {
		t.Fatal(err)
	}
	v.Facts = append(v.Facts[:11], v.Facts[17:]...)
	v.State.Claim.Phase = v0.ClaimPhaseFailed
	v.Outcome.Status, v.Outcome.Failure, v.Outcome.Model = "Failed", "synthetic failure", nil
	v.Outcome.Text = ""
	for i := range v.Facts {
		if v.Facts[i].Kind == "Runtime" && v.Facts[i].Operation == "Succeeded" {
			v.Facts[i].Operation = "Failed"
		}
		if v.Facts[i].Kind == "RunOutcome" {
			v.Facts[i].Operation = "Failed"
			v.Facts[i].Reason = "synthetic failure"
		}
	}
	for i := range v.State.Evidence.RuntimeEvents {
		if v.State.Evidence.RuntimeEvents[i].Kind == "Succeeded" {
			v.State.Evidence.RuntimeEvents[i].Kind = "Failed"
		}
	}
	b, _ := json.Marshal(v)
	if _, err := decodeView(b, "demo"); err != nil {
		t.Fatalf("interrupted native attempt must remain readable: %v", err)
	}
}

func TestNativeObservationPhaseRelationships(t *testing.T) {
	for _, n := range []workerprotocol.NativeObservation{
		{Phase: "dns", Result: "error"}, {Phase: "dns", Result: "timeout"},
		{Phase: "tcp", Result: "timeout", IP: "172.64.154.22"}, {Phase: "tls", Result: "error", IP: "172.64.154.22"},
		{Phase: "http", Result: "received", IP: "172.64.154.22", StatusCode: 200},
		{Phase: "http", Result: "timeout", IP: "172.64.154.22", StatusCode: 200},
		{Phase: "http", Result: "error", IP: "172.64.154.22"},
	} {
		if !validNativeReport(&n) {
			t.Fatalf("valid report rejected: %+v", n)
		}
	}
	for _, n := range []workerprotocol.NativeObservation{
		{Phase: "dns", Result: "received", StatusCode: 200}, {Phase: "dns", Result: "error", IP: "172.64.154.22"},
		{Phase: "tcp", Result: "error"}, {Phase: "tls", Result: "received", IP: "172.64.154.22"},
		{Phase: "tcp", Result: "timeout", IP: "172.64.154.22", StatusCode: 200},
		{Phase: "http", Result: "received", IP: "172.64.154.22"},
		{Phase: "http", Result: "error", IP: "172.64.154.22", StatusCode: 99},
	} {
		if validNativeReport(&n) {
			t.Fatalf("impossible report accepted: %+v", n)
		}
	}
}
