// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from '@playwright/test';
export default defineConfig({testDir:'./product',outputDir:'../.tmp/product-acceptance',workers:1,fullyParallel:false,timeout:45000,reporter:[['list']],use:{baseURL:'http://127.0.0.1:5210',viewport:{width:1440,height:1040},trace:'retain-on-failure'}});
