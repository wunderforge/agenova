// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { ReactNode } from 'react';

// Both sources share the reading order, not their data or interpretation.
export function WorkDetailLayout({ status, failure, result, activity, access, details, activityHref, compact=false }: {
  status: ReactNode; failure?: ReactNode; result?: ReactNode; activity: ReactNode;
  access: ReactNode; details: ReactNode; activityHref: string; compact?:boolean;
}) {
  if(compact)return <div className="portal-work-reading portal-work-focused">
    {failure}{result}
    <details className="portal-details portal-work-evidence"><summary>Activity &amp; evidence</summary>
      <div className="portal-disclosure-body">{status}{activity}
        <details className="portal-details portal-work-access"><summary>Access &amp; limits</summary><div className="portal-disclosure-body">{access}</div></details>
        <details className="portal-details portal-work-execution"><summary>Execution details</summary><div className="portal-disclosure-body">{details}</div></details>
        <a className="portal-full-record" href={activityHref}>Full activity record</a>
      </div>
    </details>
  </div>;
  return <div className="portal-work-reading">
    {status}
    {failure}
    {result}
    {activity}
    <div className="portal-work-inspect">
      <details className="portal-details portal-work-access">
        <summary><span>Access &amp; limits</span><small>Scope, tools and time limit</small></summary>
        <div className="portal-disclosure-body">{access}</div>
      </details>
      <details className="portal-details portal-work-execution">
        <summary><span>Execution details</span><small>Lifecycle, identifiers and policy version</small></summary>
        <div className="portal-disclosure-body">{details}</div>
      </details>
    </div>
    <a className="portal-full-record" href={activityHref}>Full activity record</a>
  </div>;
}
