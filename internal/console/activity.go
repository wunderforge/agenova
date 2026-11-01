// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/wunderforge/agenova/internal/evidence"
	"github.com/wunderforge/agenova/internal/workerprotocol"
)

type activityEntry struct {
	work    string
	content evidence.ActivityContent
	expires time.Time
}
type memoryActivityStore struct {
	mu       sync.Mutex
	entries  map[string]activityEntry
	bytes    int
	maxBytes int
}

// NewMemoryActivityStore is bounded, volatile and opt-in. Expired content is
// unavailable, never reconstructed from a later model response.
func NewMemoryActivityStore(maxBytes int) evidence.ActivityStore {
	if maxBytes < 64<<10 || maxBytes > 64<<20 {
		maxBytes = 16 << 20
	}
	return &memoryActivityStore{entries: map[string]activityEntry{}, maxBytes: maxBytes}
}

var sensitiveAssignment = regexp.MustCompile(`(?i)(authorization|api[_-]?key|access[_-]?token|token|password|client_secret|secret|credential|signature)(["\x27]?\s*[:=]\s*["\x27]?)(?:Bearer\s+)?[^\s&,;"<>]+`)
var urlUserinfo = regexp.MustCompile(`(https?://)[^\s/"<>@]+@`)
var bearerValue = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._~+/-]+`)
var privateBlock = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
var secretPrefix = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|AIza[A-Za-z0-9_-]{20,})\b`)
var sensitiveJSONKey = regexp.MustCompile(`(?i)^(authorization|api[_-]?key|access[_-]?token|password|client_secret|secret|credential|signature|token)$`)

func safeActivityText(text string) (string, bool) {
	// Redact whole sensitive JSON values, including objects/arrays. Regex-only
	// replacement can leave part of a structured credential visible.
	var value any
	changed, deep := false, false
	if json.Unmarshal([]byte(text), &value) == nil {
		var visit func(any, int)
		visit = func(v any, depth int) {
			switch n := v.(type) {
			case map[string]any:
				for key, child := range n {
					if sensitiveJSONKey.MatchString(key) {
						n[key] = "[redacted]"
						changed = true
					} else if depth >= 64 {
						n[key] = "[nested content truncated]"
						changed = true
						deep = true
					} else {
						visit(child, depth+1)
					}
				}
			case []any:
				for i, child := range n {
					if depth >= 64 {
						n[i] = "[nested content truncated]"
						changed = true
						deep = true
					} else {
						visit(child, depth+1)
					}
				}
			}
		}
		visit(value, 0)
		if changed {
			if raw, err := json.Marshal(value); err == nil {
				text = string(raw)
			}
		}
	}
	text = privateBlock.ReplaceAllString(text, "[private key redacted]")
	text = sensitiveAssignment.ReplaceAllString(text, "$1$2[redacted]")
	text = bearerValue.ReplaceAllString(text, "Bearer [redacted]")
	text = secretPrefix.ReplaceAllString(text, "[redacted]")
	text = urlUserinfo.ReplaceAllString(text, "$1[redacted]@")
	truncated := len(text) > 12<<10
	if truncated {
		text = text[:12<<10]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	return text, truncated || deep
}
func sanitizeActivity(c evidence.ActivityContent) evidence.ActivityContent {
	for _, value := range []*string{&c.Input, &c.Output, &c.Thinking, &c.DecisionSummary, &c.Model, &c.FinishReason} {
		clean, short := safeActivityText(*value)
		*value = clean
		c.Truncated = c.Truncated || short
	}
	return c
}
func (s *memoryActivityStore) Put(work string, c evidence.ActivityContent) (string, error) {
	c = sanitizeActivity(c)
	raw, _ := json.Marshal(c)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, value := range s.entries {
		if now.After(value.expires) {
			b, _ := json.Marshal(value.content)
			s.bytes -= len(b)
			delete(s.entries, key)
		}
	}
	if len(s.entries) >= 4096 || s.bytes+len(raw) > s.maxBytes {
		return "", errors.New("activity content capacity reached")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", errors.New("activity content entropy unavailable")
	}
	ref := "content:" + hex.EncodeToString(nonce[:])
	s.entries[ref] = activityEntry{work, c, now.Add(24 * time.Hour)}
	s.bytes += len(raw)
	return ref, nil
}
func (s *memoryActivityStore) Get(work, ref string) (evidence.ActivityContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[ref]
	if !ok || entry.work != work || time.Now().After(entry.expires) {
		return evidence.ActivityContent{}, errors.New("activity content unavailable")
	}
	// Value contains strings and scalar metadata only.
	return entry.content, nil
}
func (s *Service) captureActivity(work string, c evidence.ActivityContent) string {
	if s.activity == nil {
		return ""
	}
	ref, err := s.activity.Put(work, c)
	if err != nil {
		return ""
	}
	return ref
}
func (s *Service) activityContent(work, ref string) (evidence.ActivityContent, error) {
	if s.activity == nil {
		return evidence.ActivityContent{}, errors.New("activity capture is not enabled")
	}
	return s.activity.Get(work, ref)
}

func activityInput(operation, scope, input string) string {
	// JSON preserves argument boundaries; credentials are removed at storage.
	var parameters any = input
	if json.Valid([]byte(input)) {
		var decoded any
		if json.Unmarshal([]byte(input), &decoded) == nil {
			parameters = decoded
		}
	}
	data, _ := json.Marshal(map[string]any{"operation": operation, "resourceScope": scope, "parameters": parameters})
	return strings.TrimSpace(string(data))
}

func safeFeedbackReason(code string) string {
	return workerprotocol.FeedbackReason(code)
}
