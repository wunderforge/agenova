// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from 'vitest/config';
// Build-only Go validation: no HTTP service or parser/policy implementation in UI.
// @ts-expect-error JavaScript build helper has no public application API.
import { fixtureRows, root } from './scripts/contracts.mjs';
import { resolve } from 'node:path';

const moduleID = 'virtual:agenova-fixtures';
export default defineConfig({
  plugins: [{
    name: 'agenova-v0-fixtures',
    resolveId(id) { if (id === moduleID) return '\0' + moduleID; },
    load(id) {
      if (id !== '\0' + moduleID) return;
      const rows = fixtureRows();
      this.addWatchFile(resolve(root, 'harness/fixtures/contract/v0/manifest.json'));
      for (const row of rows) this.addWatchFile(resolve(root, 'harness/fixtures/contract/v0', row.input));
      return `export default ${JSON.stringify(rows)};`;
    },
  }],
  test: { environment: 'jsdom', include: ['src/**/*.test.{ts,tsx}'], restoreMocks: true },
});
