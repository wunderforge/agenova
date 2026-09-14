// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { ReactNode } from 'react';

// Connected routes intentionally do not import illustrative work or fixture
// records. A live surface must be added only with its accepted source and
// separate loading, empty, and failure states; absence is not an empty result.
interface Surface {
  title: string;
  heading: string;
  description: string;
}
const surfaces: Record<string, Surface> = {
  work: { title: 'Work', heading: 'Work history is not connected yet', description: 'You will see your active and completed work here once this view is connected.' },
  new: { title: 'New work', heading: 'Starting work is not available yet', description: 'You can explore the request form in Demo. Connected requests cannot be sent from this page yet.' },
  detail: { title: 'Work details', heading: 'Work details are not connected yet', description: 'This view cannot show a real assignment or its access and activity yet.' },
  agents: { title: 'Agents', heading: 'Agents are not connected yet', description: 'You will be able to choose from your available agents here.' },
  policy: { title: 'Policy', heading: 'Access rules are not connected yet', description: 'Your organization’s rules are not available in this view yet.' },
  activity: { title: 'Activity', heading: 'Activity is not connected yet', description: 'You will be able to inspect recorded actions here once this view is connected.' },
  identity: { title: 'Identity', heading: 'Your identity is not connected yet', description: 'This view does not know who you are. Demo uses an example requester.' },
};
const progress: { label: string; detail: string; route: string }[] = [
  { label: 'See work and progress', detail: 'Active work, completed work, and outcomes', route: 'work' },
  { label: 'Start new work', detail: 'Send a request and follow its result', route: 'work/new' },
  { label: 'Choose an agent', detail: 'Available agents and their intended use', route: 'agents' },
  { label: 'Review access rules', detail: 'What a requester is allowed to start', route: 'policy' },
  { label: 'Inspect recorded actions', detail: 'Decisions and activity across work', route: 'platform/activity' },
];
function NotConnected() { return <span className="portal-badge not-connected">Not connected</span>; }
function EmptyState({ surface, onDemo }: { surface: Surface; onDemo: () => void }) {
  return <><h1>{surface.title}</h1><div className="portal-connected-empty" role="status"><NotConnected/><h2>{surface.heading}</h2><p>{surface.description}</p>
    <button className="portal-button" type="button" onClick={onDemo}>Explore in Demo</button></div></>;
}
function ProgressPage({ onDemo }: { onDemo: () => void }) {
  return <><h1>Platform</h1><p className="portal-connected-intro">What this connection can show.</p>
    <div className="portal-connected-summary" role="status"><NotConnected/><div><strong>No live views are connected yet</strong><p>Demo remains available to explore the intended experience. It does not contain your organization’s work.</p></div></div>
    <div className="portal-table-wrap"><table className="portal-table portal-connected-table"><thead><tr><th>What you can do</th><th>When connected</th><th>Current status</th></tr></thead><tbody>
      {progress.map(item => <tr key={item.route}><td>{item.label}</td><td>{item.detail}</td><td><NotConnected/></td></tr>)}
    </tbody></table></div><button className="portal-button portal-connected-demo" type="button" onClick={onDemo}>Explore in Demo</button></>;
}
export function ConnectedPortal({ parts, onDemo }: { parts: string[]; onDemo: () => void }): ReactNode {
  const section = parts[0] || 'work';
  if (section === 'platform' && parts[1] !== 'activity') return <ProgressPage onDemo={onDemo}/>;
  let surface: Surface;
  if (section === 'work') surface = parts[1] === 'new' ? surfaces.new : parts[1] ? surfaces.detail : surfaces.work;
  else if (section === 'agents') surface = surfaces.agents;
  else if (section === 'policy') surface = surfaces.policy;
  else if (section === 'platform') surface = surfaces.activity;
  else surface = surfaces.identity;
  return <EmptyState surface={surface} onDemo={onDemo}/>;
}
