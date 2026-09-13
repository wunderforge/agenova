// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir: './smoke',
  outputDir: '../.tmp/ui-smoke',
  reporter: [['list']],
  use: { baseURL: 'http://127.0.0.1:4173', viewport: { width: 1100, height: 1000 }, trace: 'retain-on-failure' },
  globalSetup: './smoke/server.ts',
});
