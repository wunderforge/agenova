// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import canonicalRows from 'virtual:agenova-fixtures';
import { App } from './App';
import { createFixtureSource, storyboardRows } from './fixture-source';
import './style.css';

const rows = storyboardRows(canonicalRows);
const source = createFixtureSource(rows);
const selections = rows.map(row => ({ id: row.id, provenance: row.derivedFrom
  ? `Derived display test from ${row.derivedFrom}: ${row.input}. Not a canonical fixture.`
  : `Canonical fixture: harness/fixtures/contract/v0/${row.input}` }));
createRoot(document.getElementById('root')!).render(<StrictMode><App source={source} selections={selections}/></StrictMode>);
