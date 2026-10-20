// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState } from 'react';
import { PortalIcon } from './PortalIcon';

type Panel = 'guide' | 'help' | 'settings';
export function PortalUtilities({ density, setDensity }: { density: string; setDensity: (value: 'comfortable' | 'compact') => void }) {
  const [panel, setPanel] = useState<Panel>();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey || event.altKey || event.target instanceof HTMLElement && event.target.closest('input,textarea,select,[contenteditable=true]')) return;
      if (event.key === '/' && !dialog.current?.open) {
        const search = document.querySelector<HTMLInputElement>('input[aria-label="Search work"]');
        if (search) { event.preventDefault(); search.focus(); }
      }
      if (event.key === '?' && !dialog.current?.open) { event.preventDefault(); setPanel('help'); }
    };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, []);
  useEffect(() => { if (panel && dialog.current && !dialog.current.open) dialog.current.showModal(); }, [panel]);
  const close = () => { dialog.current?.close(); setPanel(undefined); };
  return <>
    <div className="portal-utilities" aria-label="Workspace help and settings">
      <button type="button" onClick={() => setPanel('guide')}><PortalIcon name="book"/>Quick guide</button>
      <button type="button" onClick={() => setPanel('help')}><PortalIcon name="help"/>Help & shortcuts</button>
      <button type="button" onClick={() => setPanel('settings')}><PortalIcon name="settings"/>View settings</button>
    </div>
    {panel && <dialog ref={dialog} className="portal-utility-dialog" aria-labelledby="portal-dialog-title" onCancel={close} onClose={() => setPanel(undefined)}>
      <div className="portal-dialog-head"><h2 id="portal-dialog-title">{panel === 'guide' ? 'Quick guide' : panel === 'help' ? 'Help & shortcuts' : 'View settings'}</h2><button type="button" onClick={close} aria-label="Close dialog">×</button></div>
      {panel === 'guide' ? <><p>One task. Its own permissions. Inspectable evidence.</p><ol>
        <li><strong>Describe the work.</strong> Choose an agent template and request only the access the task needs.</li>
        <li><strong>Check the decision.</strong> Requested access is not a grant. Inspect the policy decision and effective limits.</li>
        <li><strong>Follow the evidence.</strong> Open a Work, inspect recorded calls and distinguish access checks from execution outcomes.</li>
      </ol><a href="https://github.com/wunderforge/agenova#readme" target="_blank" rel="noopener noreferrer">Read the documentation ↗</a></>
        : panel === 'help' ? <><p>Inspect a Work by selecting its name. Use Inspect beside an activity record to open its details.</p><dl><dt><kbd>/</kbd></dt><dd>Focus Work search on the list page</dd><dt><kbd>?</kbd></dt><dd>Open this help</dd><dt><kbd>Esc</kbd></dt><dd>Close a dialog or profile menu</dd><dt><kbd>Tab</kbd></dt><dd>Move between links and controls</dd></dl><a href="https://github.com/wunderforge/agenova/issues" target="_blank" rel="noopener noreferrer">Report a problem ↗</a></>
          : <><p>Presentation settings for this tab only. These do not change platform configuration or permissions.</p><fieldset><legend>Table density</legend>{(['comfortable', 'compact'] as const).map(value => <label key={value}><input type="radio" name="table-density" value={value} checked={density === value} onChange={() => setDensity(value)}/>{value === 'comfortable' ? 'Comfortable' : 'Compact'}</label>)}</fieldset><p className="portal-muted">Appearance: June light. Activity chart timezone: UTC.</p></>}
    </dialog>}
  </>;
}

export function ProfileMenu({ name, team, initials, accountNavigation, sidebar = false }: { name: string; team: string; initials: string; accountNavigation?: string; sidebar?: boolean }) {
  const menu = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    const outside = (event: PointerEvent) => { if (menu.current && !menu.current.contains(event.target as Node)) menu.current.open = false; };
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && menu.current?.open) { menu.current.open = false; menu.current.querySelector('summary')?.focus(); } };
    document.addEventListener('pointerdown', outside); document.addEventListener('keydown', escape);
    return () => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape); };
  }, []);
  return <details className={`portal-profile${sidebar ? ' sidebar-profile' : ''}`} ref={menu}>
    <summary aria-label={sidebar ? 'Open workspace account menu' : 'Open profile menu'} title={name}><span className="portal-avatar">{initials}</span>{sidebar && <span className="portal-profile-label"><strong>{name}</strong><small>{team}</small></span>}{sidebar && <span className="portal-profile-chevron">⌄</span>}</summary>
    <div className="portal-profile-popover" aria-label="Account details"><strong>{name}</strong><small>{team}</small>
      <a href="#/identity" onClick={() => { if (menu.current) menu.current.open = false; }}>View identity</a>
      {accountNavigation === 'admin' && <a href="/admin">Administration</a>}
      <a href="#/platform" onClick={() => { if (menu.current) menu.current.open = false; }}>Platform connection</a>
      {accountNavigation && <form method="post" action="/logout"><button type="submit">Log out</button></form>}
    </div>
  </details>;
}
