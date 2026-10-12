// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from '@playwright/test';

// Opt-in live test. Start the three isolated local APIs and Vite proxies first.
export default defineConfig({
  testDir: './role-demo',
  outputDir: '../.tmp/ui-role-demo',
  reporter: [['list']],
  use: { viewport: { width: 1280, height: 960 }, trace: 'retain-on-failure' },
});
