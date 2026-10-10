// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/wunderforge/agenova/internal/evidence"
)

func TestMemoryReaderCLIMetadataTextAndJSON(t *testing.T) {
	data, err := os.ReadFile("../../work/0179-scoped-memory/memory-reader-vectors.json")
	var vectors struct{ View evidence.View }
	if err != nil || json.Unmarshal(data, &vectors) != nil {
		t.Fatal("cannot load shared public view")
	}
	services := Services{ShowConnected: func(string, string) (evidence.View, error) { return vectors.View, nil }}
	for _, jsonOutput := range []bool{false, true} {
		var out, diagnostic bytes.Buffer
		if printWork(&out, &diagnostic, parsedArgs{operands: []string{"show", "memory-reader"}, json: jsonOutput}, services) != 0 {
			t.Fatal(diagnostic.String())
		}
		if strings.Contains(out.String(), "private") {
			t.Fatal("private content exported")
		}
		for _, status := range []string{"Written", "Found"} {
			if !strings.Contains(out.String(), status) {
				t.Fatal("Memory result omitted")
			}
		}
		if !jsonOutput && (!strings.Contains(out.String(), "status=Denied") || !strings.Contains(out.String(), "count=1") || !strings.Contains(out.String(), "content: withheld")) {
			t.Fatal("metadata parity or privacy marker omitted")
		}
	}
}
