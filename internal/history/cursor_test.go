// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package history

import (
	"bytes"
	"testing"
)

func TestPrivatePageCursorTamperAndKeyRotation(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	in := map[string]string{"position": "private-ordering"}
	token, err := SealPage(key, in)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]string
	if OpenPage(key, token, &out) != nil || out["position"] != in["position"] {
		t.Fatal("cursor did not roundtrip")
	}
	if OpenPage(key, token+"x", &out) == nil || OpenPage(bytes.Repeat([]byte{2}, 32), token, &out) == nil {
		t.Fatal("invalid cursor accepted")
	}
}
