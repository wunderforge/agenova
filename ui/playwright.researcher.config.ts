// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {defineConfig} from '@playwright/test';

// Explicit local acceptance only. The caller owns the already-running service.
export default defineConfig({
 testDir:'./local-acceptance',
 outputDir:'../.tmp/researcher-ui',
 reporter:[['list']],
 workers:1,
 use:{baseURL:'http://127.0.0.1:5226',viewport:{width:1440,height:1080},trace:'retain-on-failure',launchOptions:{args:['--disable-gpu']}},
});
