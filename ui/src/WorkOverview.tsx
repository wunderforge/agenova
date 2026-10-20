// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// The list may be session-scoped or paged. Never present it as platform-wide analytics.
export function WorkOverview({ statuses }: { statuses: string[] }) {
  const metrics = [
    ['Loaded Work', statuses.length, 'In this list', 'total'],
    ['Succeeded', statuses.filter(s => s === 'Succeeded').length, 'Execution completed', 'succeeded'],
    ['Denied', statuses.filter(s => s === 'Denied').length, 'Stopped before execution', 'denied'],
    ['Failed', statuses.filter(s => s === 'Failed').length, 'Requires investigation', 'failed'],
  ] as const;
  return <div className="portal-overview" aria-label="Loaded work summary">{metrics.map(([label, count, hint, kind]) =>
    <div key={label} data-kind={kind}><span>{label}</span><strong>{count}</strong><small>{hint}</small></div>
  )}</div>;
}
