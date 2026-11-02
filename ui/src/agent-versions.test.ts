// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {expect,it} from 'vitest';
import {latestAgentVersions} from './agent-versions';
import type {AgentEntry} from './connected-source';

export function registration(name:string,project='research',canSubmit=true):AgentEntry {
  return {project,canSubmit,template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name},spec:{artifact:{image:'example/agent@sha256:'+'a'.repeat(64)},entrypoint:{command:['/agent']},capabilityCeiling:{tools:['crm.read'],runtimeProfiles:['isolated']}}}};
}

it('selects numeric latest versions per project/family without changing registrations or grants',()=>{
  const entries=[registration('researcher'),registration('researcher-v2'),registration('researcher-advisor-v9'),registration('researcher-advisor-v10'),registration('researcher-advisor-v6','payments',false),registration('engineer','payments'),registration('incident-reader'),registration('my-researcher'),registration('invoice-v2'),registration('invoice-v12')];
  const original=JSON.stringify(entries);
  const visible=latestAgentVersions(entries);
  expect(visible.map(e=>[e.project,e.template.metadata.name])).toEqual([['research','researcher-advisor-v10'],['payments','researcher-advisor-v6'],['payments','engineer'],['research','incident-reader'],['research','my-researcher'],['research','invoice-v12']]);
  expect(visible[1].canSubmit).toBe(false);
  expect(visible[0]).toBe(entries[3]);
  expect(JSON.stringify(entries)).toBe(original);
});

it('does not hide an only visible old version or infer versions from arbitrary names',()=>{
  const entries=[registration('researcher-advisor-v3'),registration('researcher-experimental'),registration('invoice-v0'),registration('invoice-vnext'),registration('python3')];
  expect(latestAgentVersions(entries)).toEqual(entries);
});

it('does not mistake a higher legacy suffix for a newer reference release series',()=>{
  for(const entries of [[registration('researcher-v7'),registration('researcher-advisor-v6')],[registration('researcher-advisor-v6'),registration('researcher-v7')]]) {
    expect(latestAgentVersions(entries).map(e=>e.template.metadata.name)).toEqual(['researcher-advisor-v6']);
  }
  // Visibility is server-owned: never invent an Advisor registration in a
  // project where only the older reference series is actually visible.
  expect(latestAgentVersions([registration('researcher-v7')])[0].template.metadata.name).toBe('researcher-v7');
});
