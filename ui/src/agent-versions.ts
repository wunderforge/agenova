// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { AgentEntry } from './connected-source';

// Display convention only, never registration identity or authorization.
// The published reference Researcher changed series when Advisor was introduced;
// old researcher-v7 must not supersede the newer researcher-advisor-v6 series.
// Other families are grouped only by an explicit trailing numeric -vN suffix.
function version(name: string): {family: string; number: bigint; advisor: boolean} {
  const researcher = /^researcher(?:-advisor)?(?:-v([1-9][0-9]*))?$/.exec(name);
  if (researcher) return {family: 'researcher', number: BigInt(researcher[1] || '0'), advisor: name.startsWith('researcher-advisor')};
  const match = /^(.*)-v([1-9][0-9]*)$/.exec(name);
  return {family: match?.[1] || name, number: BigInt(match?.[2] || '0'), advisor: false};
}

function latestPerFamily<T>(entries: T[], name: (entry:T)=>string, project: (entry:T)=>string): T[] {
  const latest = new Map<string, T>();
  for (const entry of entries) {
    const candidate = version(name(entry));
    const key = JSON.stringify([project(entry), candidate.family]);
    const previous = latest.get(key);
    const current = previous === undefined ? undefined : version(name(previous));
    if (!current || candidate.advisor && !current.advisor || candidate.advisor === current.advisor && candidate.number > current.number) latest.set(key, entry);
  }
  const chosen = new Set(latest.values());
  return entries.filter(entry => chosen.has(entry));
}

export function latestAgentVersions(entries: AgentEntry[]): AgentEntry[] {
  return latestPerFamily(entries, entry=>entry.template.metadata.name, entry=>entry.project);
}

export function latestAgentNames(names: string[]): string[] {
  return latestPerFamily(names, name=>name, ()=>'');
}

// Keep an explicitly selected old registration visible rather than displaying
// the new one while silently submitting the original immutable identity.
export function agentPickerNames(names: string[], selected?: string): string[] {
  const latest = latestAgentNames(names);
  return selected && !latest.includes(selected) ? [...latest, selected] : latest;
}
