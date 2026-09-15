// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
export const scenarios = ['canonical', 'narrowed', 'loading', 'not-found', 'malformed', 'unavailable'] as const;
export type Scenario = typeof scenarios[number];
export interface ConsoleRoute { kind: 'requests' | 'claims'; reference: string; scenario: Scenario }
export type RouteResult = { status: 'route'; route: ConsoleRoute } | { status: 'malformed-route' };

// Client presentation paths only: no server endpoint or identifier contract.
export function parseConsoleRoute(pathname: string, search: string): RouteResult {
  const match = /^\/console\/(requests|claims)\/([^/]+)$/.exec(pathname);
  if (!match) return { status: 'malformed-route' };
  try {
    const reference = decodeURIComponent(match[2]);
    if (!reference.trim() || /[\u0000-\u001f\u007f]/.test(reference)) return { status: 'malformed-route' };
    const params = new URLSearchParams(search);
    const scenario = params.get('scenario') ?? 'canonical';
    if (params.getAll('scenario').length > 1 || [...params.keys()].some(key => key !== 'scenario') ||
        !scenarios.includes(scenario as Scenario)) return { status: 'malformed-route' };
    return { status: 'route', route: { kind: match[1] as ConsoleRoute['kind'], reference, scenario: scenario as Scenario } };
  } catch { return { status: 'malformed-route' }; }
}
export function consoleHref(kind: ConsoleRoute['kind'], reference: string, scenario: Scenario = 'canonical') {
  return `/console/${kind}/${encodeURIComponent(reference)}${scenario === 'canonical' ? '' : `?scenario=${scenario}`}`;
}
