// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it } from 'vitest';
import { localAPITarget } from '../local-api-target';

describe('local API development proxy target', () => {
  it('defaults to the installed API loopback tunnel', () => {
    expect(localAPITarget(undefined)).toBe('http://127.0.0.1:8088');
  });

  it('allows an explicit literal loopback address and port', () => {
    expect(localAPITarget('http://127.0.0.1:18081')).toBe('http://127.0.0.1:18081');
    expect(localAPITarget('http://[::1]:18081/')).toBe('http://[::1]:18081');
  });

  it.each([
    '',
    'http://localhost:8088',
    'http://0.0.0.0:8088',
    'http://192.168.1.10:8088',
    'https://127.0.0.1:8088',
    'http://127.0.0.1',
    'http://user:password@127.0.0.1:8088',
    'http://127.0.0.1:8088/api',
    'http://127.0.0.1:8088/?token=example',
    'http://127.0.0.1:8088/#fragment',
  ])('rejects unsafe or malformed override %s', value => {
    expect(() => localAPITarget(value)).toThrow('AGENOVA_API_URL must be an HTTP loopback origin');
  });
});
