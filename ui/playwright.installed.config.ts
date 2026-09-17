// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from '@playwright/test';

// Opt-in real installed-service check. Start `agenova api connect` and
// `npm --prefix ui run dev -- --port 5177 --strictPort` first.
export default defineConfig({
  testDir: './installed',
  outputDir: '../.tmp/ui-installed',
  reporter: [['list']],
  use: { baseURL: 'http://127.0.0.1:5177', viewport: { width: 1100, height: 1000 }, trace: 'retain-on-failure' },
});
