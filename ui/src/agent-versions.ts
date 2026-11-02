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

export function latestAgentVersions(entries: AgentEntry[]): AgentEntry[] {
  const latest = new Map<string, AgentEntry>();
  for (const entry of entries) {
    const candidate = version(entry.template.metadata.name);
    const key = JSON.stringify([entry.project, candidate.family]);
    const previous = latest.get(key);
    const current = previous && version(previous.template.metadata.name);
    if (!current || candidate.advisor && !current.advisor || candidate.advisor === current.advisor && candidate.number > current.number) latest.set(key, entry);
  }
  const chosen = new Set(latest.values());
  return entries.filter(entry => chosen.has(entry));
}
