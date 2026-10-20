// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

export interface ActivityEntry { timestamp?: string; status: string }
const series = ['Succeeded', 'Denied', 'Failed', 'In progress', 'Other'] as const;
const category = (status: string) => series.includes(status as typeof series[number]) ? status
  : ['Running', 'Starting', 'Finishing', 'Pending'].includes(status) ? 'In progress' : 'Other';

// This is a projection of the loaded request list, not global telemetry or spans.
export function activityBuckets(entries: ActivityEntry[], day: string) {
  const buckets = Array.from({ length: 24 }, () => Object.fromEntries(series.map(key => [key, 0])));
  for (const entry of entries) {
    if (!entry.timestamp || !Number.isFinite(Date.parse(entry.timestamp))) continue;
    const date = new Date(entry.timestamp);
    if (date.toISOString().slice(0, 10) === day) buckets[date.getUTCHours()][category(entry.status)]++;
  }
  return buckets;
}

export function WorkActivityChart({ entries }: { entries: ActivityEntry[] }) {
  const [chosenDay, setChosenDay] = useState('');
  const dated = entries.filter(entry => entry.timestamp && Number.isFinite(Date.parse(entry.timestamp)));
  const days = [...new Set(dated.map(entry => new Date(entry.timestamp!).toISOString().slice(0, 10)))].sort().reverse();
  const day = days.includes(chosenDay) ? chosenDay : days[0] || '';
  const buckets = activityBuckets(dated, day);
  const totals = buckets.map(bucket => series.reduce((sum, key) => sum + bucket[key], 0));
  const max = Math.max(1, ...totals);
  const total = totals.reduce((sum, count) => sum + count, 0);
  return <section className="portal-activity-chart" aria-label="Work activity">
    <div className="portal-chart-heading"><h2>Work activity</h2>
      {days.length > 0 && <label className="portal-chart-date"><span className="portal-sr-only">Activity date</span>
        <select aria-label="Activity date" value={day} onChange={event => setChosenDay(event.target.value)}>{days.map(value => <option key={value}>{value}</option>)}</select><span>UTC</span>
      </label>}
      <div className="portal-chart-legend">{series.map(key => <span key={key} data-series={key}><i/>{key}</span>)}</div>
    </div>
    {day ? <><div className="portal-chart-plot" role="img" aria-label={`${total} loaded requests on ${day}, grouped by submission hour in UTC and current status`}>
      {buckets.map((bucket, hour) => <div className="portal-chart-column" key={hour} title={`${String(hour).padStart(2, '0')}:00 UTC · ${totals[hour]} requests · ${series.map(key => `${bucket[key]} ${key}`).join(', ')}`}>
        {series.map(key => bucket[key] > 0 && <span key={key} data-series={key} style={{ height: `${bucket[key] / max * 72}px` }}/>)}</div>)}
    </div><div className="portal-chart-axis" aria-hidden="true"><span>00:00</span><span>06:00</span><span>12:00</span><span>18:00</span><span>23:00</span></div></>
      : <p className="portal-chart-empty">No timestamped requests in this loaded list.</p>}
    <p className="portal-chart-note">Loaded list only · grouped by request time · colors show current status{entries.length > dated.length && ` · ${entries.length - dated.length} without a recorded request time`}</p>
  </section>;
}
