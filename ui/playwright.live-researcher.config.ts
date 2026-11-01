// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {defineConfig} from '@playwright/test';
// Explicit opt-in: run the documented, owned local preview first. No cloud URLs.
export default defineConfig({testDir:'./acceptance/live-researcher',outputDir:'../.tmp/live-researcher-browser',workers:1,reporter:'list',timeout:360000,
 use:{baseURL:process.env.AGENOVA_LOCAL_RESEARCHER_URL||'http://127.0.0.1:5225',viewport:{width:1440,height:1050},colorScheme:'dark',trace:'retain-on-failure'}});
