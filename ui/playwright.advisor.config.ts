// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {defineConfig} from '@playwright/test';
import {fileURLToPath} from 'node:url';
export default defineConfig({
 testDir:'./acceptance/advisor',outputDir:process.env.AGENOVA_TEST_LOCAL_ADVISOR==='1'?'../.tmp/advisor-browser-model':'../.tmp/advisor-browser',workers:1,reporter:'list',timeout:process.env.AGENOVA_TEST_LOCAL_ADVISOR==='1'?120000:30000,
 use:{baseURL:'http://127.0.0.1:5222',viewport:{width:1440,height:1050},trace:'retain-on-failure',colorScheme:'dark'},
 webServer:{command:`"${fileURLToPath(new URL('../.tmp/access-advisor-preview.exe',import.meta.url))}" --volatile --listen 127.0.0.1:5222 --advisor-config harness/examples/access-advisor/advisor-${process.env.AGENOVA_TEST_LOCAL_ADVISOR==='1'?'model':'rules'}.json`,cwd:fileURLToPath(new URL('../',import.meta.url)),url:'http://127.0.0.1:5222/healthz',reuseExistingServer:false,timeout:30000},
});
