// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"github.com/wunderforge/agenova/internal/evidence"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestActivityIsBoundedRedactedAndWorkScoped(t *testing.T) {
	nested, _ := safeActivityText(`{"secret":{"nested":["FAKE-NESTED-CREDENTIAL"]},"safe":"actual result"}`)
	if strings.Contains(nested, "FAKE-NESTED") || !strings.Contains(nested, "actual result") {
		t.Fatal("structured credential was not masked")
	}
	s := NewMemoryActivityStore(64 << 10).(*memoryActivityStore)
	secret := `{"api_key":"FAKE-KEY","password":"FAKE-PASS"} Bearer FAKE-TOKEN https://user:FAKE-PASS@example.org/?token=visible`
	ref, err := s.Put("a", evidence.ActivityContent{Output: secret, Thinking: strings.Repeat("中", 5000)})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Get("a", ref)
	if err != nil || strings.Contains(c.Output, "FAKE-") || !c.Truncated || !utf8.ValidString(c.Thinking) || len(c.Thinking) > 12<<10 {
		t.Fatalf("redaction or bounds failed: %+v %v", c, err)
	}
	if _, err = s.Get("b", ref); err == nil {
		t.Fatal("cross Work content exposed")
	}
	entry := s.entries[ref]
	entry.expires = time.Now().Add(-time.Second)
	s.entries[ref] = entry
	if _, err = s.Get("a", ref); err == nil {
		t.Fatal("expired content exposed")
	}
	for i := 0; i < 20; i++ {
		_, err = s.Put("a", evidence.ActivityContent{Output: strings.Repeat("x", 12000)})
		if err != nil {
			return
		}
	}
	t.Fatal("content store exceeded capacity")
}
