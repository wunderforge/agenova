// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,fireEvent,render,screen,within} from '@testing-library/react';
import {AgentDirectory} from './ProductPages';
import {connectedSource} from './connected-source';
import type {AgentEntry} from './connected-source';

afterEach(cleanup);
function entry(name:string,project='research'):AgentEntry {
  return {project,canSubmit:true,template:{apiVersion:'agenova.io/v1alpha1',kind:'AgentTemplate',metadata:{name},spec:{artifact:{image:'example/agent@sha256:'+'a'.repeat(64)},entrypoint:{command:['/agent']},capabilityCeiling:{tools:['crm.read']}}}};
}
it('defaults to latest, preserves exact Run target and restores historical registrations with one control',async()=>{
  history.replaceState(null,'','/?mode=connected#/agents');
  vi.spyOn(connectedSource,'agents').mockResolvedValue([entry('researcher-advisor-v2'),entry('researcher-advisor-v6'),entry('engineer','payments')]);
  render(<AgentDirectory/>);
  await screen.findByRole('button',{name:'researcher-advisor-v6'});
  expect(screen.queryByRole('button',{name:'researcher-advisor-v2'})).toBeNull();
  const run=screen.getByRole('link',{name:'Run researcher-advisor-v6 in research'});
  expect(run.getAttribute('href')).toContain('template=researcher-advisor-v6');
  fireEvent.click(screen.getByRole('checkbox',{name:'Show older versions'}));
  expect(screen.getByRole('button',{name:'researcher-advisor-v2'})).toBeTruthy();
  fireEvent.change(screen.getByRole('searchbox',{name:'Search agents'}),{target:{value:'project:payments'}});
  expect(within(screen.getByRole('table',{name:'Agent catalog'})).getByRole('button',{name:'engineer'})).toBeTruthy();
  expect(screen.queryByRole('button',{name:'researcher-advisor-v6'})).toBeNull();
});
