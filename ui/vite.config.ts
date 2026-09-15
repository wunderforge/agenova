// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from 'vitest/config';
// Build-only Go validation: no HTTP service or parser/policy implementation in UI.
// @ts-expect-error JavaScript build helper has no public application API.
import { fixtureRows, consoleFixtureRows, root } from './scripts/contracts.mjs';
import { resolve } from 'node:path';

const moduleID = 'virtual:agenova-fixtures';
const consoleID = 'virtual:agenova-console-fixtures';
export default defineConfig({
  server: { proxy: { '/api': { target: 'http://127.0.0.1:8088', changeOrigin: false } } },
  preview: { proxy: { '/api': { target: 'http://127.0.0.1:8088', changeOrigin: false } } },
  plugins: [{
    name: 'agenova-v0-fixtures',
    resolveId(id) { if (id === moduleID || id === consoleID) return '\0' + id; },
    load(id) {
      if (id !== '\0' + moduleID && id !== '\0' + consoleID) return;
      const rows = id === '\0' + consoleID ? consoleFixtureRows() : fixtureRows();
      this.addWatchFile(resolve(root, 'harness/fixtures/contract/v0/manifest.json'));
      for (const row of rows) this.addWatchFile(resolve(root, 'harness/fixtures/contract/v0', row.input));
      return `export default ${JSON.stringify(rows)};`;
    },
  }],
  test: { environment: 'jsdom', include: ['src/**/*.test.{ts,tsx}'], restoreMocks: true },
});
