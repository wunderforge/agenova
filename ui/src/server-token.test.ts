// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// @vitest-environment node
import { chmodSync, symlinkSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { serverBearerToken } from '../server-token';

describe('server bearer token', () => {
  it('reads one protected token without changing its contents', () => {
    const directory = mkdtempSync(join(tmpdir(), 'agenova-token-'));
    const path = join(directory, 'token');
    writeFileSync(path, 'signed.jwt.value\n', { mode: 0o600 });
    expect(serverBearerToken(path)).toBe('signed.jwt.value');
    expect(serverBearerToken(undefined)).toBeUndefined();
  });

  it('rejects symlinks, whitespace and unsafe permissions', () => {
    const directory = mkdtempSync(join(tmpdir(), 'agenova-token-'));
    const target = join(directory, 'target');
    writeFileSync(target, 'signed.jwt.value', { mode: 0o600 });
    const link = join(directory, 'link');
    symlinkSync(target, link);
    expect(() => serverBearerToken(link)).toThrow(/regular file/);
    writeFileSync(target, 'signed jwt value', { mode: 0o600 });
    expect(() => serverBearerToken(target)).toThrow(/one bounded bearer token/);
    if (process.platform !== 'win32') {
      writeFileSync(target, 'signed.jwt.value', { mode: 0o600 });
      chmodSync(target, 0o644);
      expect(() => serverBearerToken(target)).toThrow(/permissions/);
    }
  });
});
