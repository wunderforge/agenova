// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import { displayWorkName } from './work-name';
describe('work display name', () => {
  it('prefers an explicit name without changing the instructions', () => {
    expect(displayWorkName('  Retry investigation  ', 'Read all artifacts. Recommend a fix.', 'id')).toBe('Retry investigation');
  });
  it('uses the first sentence for legacy tasks', () => {
    expect(displayWorkName(undefined, 'Investigate payment retries. Read the artifacts.', 'id')).toBe('Investigate payment retries.');
    expect(displayWorkName(undefined, '调查支付超时。阅读日志并给出建议。', 'id')).toBe('调查支付超时。');
  });
  it('does not split a URL or a filename at its dots', () => {
    expect(displayWorkName('', 'Read github.com/org/repo and retry.go. Explain failures.', 'id')).toBe('Read github.com/org/repo and retry.go.');
  });
  it('uses the first line and caps long text', () => {
    expect(displayWorkName(null, 'Short objective\nLong instructions', 'id')).toBe('Short objective');
    expect(Array.from(displayWorkName('', '字'.repeat(100), 'id'))).toHaveLength(80);
  });
  it('falls back to identity for absent or non-text input', () => {
    expect(displayWorkName({}, 12, 'work-id')).toBe('work-id');
    expect(displayWorkName('  ', '  ', 'work-id')).toBe('work-id');
  });
});
