// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { StrictMode, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import canonicalRows from 'virtual:agenova-fixtures';
import consoleRows from 'virtual:agenova-console-fixtures';
import { App } from './App';
import { createFixtureSource, storyboardRows } from './fixture-source';
import './style.css';
import './console.css';
import { ClaimConsole, ConsoleNavigation } from './ClaimConsole';
import { parseConsoleRoute } from './console-route';
import { createConsoleFixtureSource, fixtureConsoleKeys } from './console-fixture-source';
import { Portal } from './Portal';

const rows = storyboardRows(canonicalRows);
const source = createFixtureSource(rows);
const selections = rows.map(row => ({ id: row.id, provenance: row.derivedFrom
  ? `Derived display test from ${row.derivedFrom}: ${row.input}. Not a canonical fixture.`
  : `Canonical fixture: harness/fixtures/contract/v0/${row.input}` }));
function Root() {
  const [location, setLocation] = useState(() => window.location.pathname + window.location.search);
  useEffect(() => {
    const changed = () => setLocation(window.location.pathname + window.location.search);
    window.addEventListener('popstate', changed);
    return () => window.removeEventListener('popstate', changed);
  }, []);
  const url = useMemo(() => new URL(location, window.location.origin), [location]);
  const parsed = useMemo(() => parseConsoleRoute(url.pathname, url.search), [url]);
  const consoleSource = useMemo(() => createConsoleFixtureSource(consoleRows, parsed.status === 'route' ? parsed.route.scenario : 'canonical'), [parsed]);
  if (url.pathname === '/') return <Portal mode={url.searchParams.get('mode') === 'connected' ? 'connected' : 'demo'}/>;
  if (url.pathname === '/fixtures') return <App source={source} selections={selections}/>;
  if (parsed.status === 'malformed-route') return <main className="console"><h1>Claim Console</h1><ConsoleNavigation/><section role="alert"><h2>Malformed console route</h2><p>Use a request-reference or claim-ID console route. No fixture was selected.</p></section></main>;
  return <ClaimConsole source={consoleSource} route={parsed.route} keys={fixtureConsoleKeys}/>;
}
createRoot(document.getElementById('root')!).render(<StrictMode><Root/></StrictMode>);
