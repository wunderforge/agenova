// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {describe,it,expect} from 'vitest';
import {taskPresets} from './task-presets';
describe('capability-derived reusable task presets',()=>{
 it('requires both the installed tool and template scope',()=>{
  expect(taskPresets(['web.read'],[])).toEqual([]);
  expect(taskPresets([],['web:public-pages'])).toEqual([]);
  expect(taskPresets(['web.read'],['web:public-pages']).map(p=>p.id)).toEqual(['web']);
 });
 it('works for any agent and keeps fixed sources distinct from search',()=>{
  const presets=taskPresets(['web.read','web.fetch','crm.read','git.read'],['web:public-pages','web:product-comparison','crm:customer-retention','repo:acme/payments']);
  expect(presets.map(p=>p.id)).toEqual(['web','crm','public','repository']);
  expect(presets.map(p=>p.label)).toEqual(['Market research','Customer insights','Product comparison','Incident investigation']);
  expect(presets.find(p=>p.id==='web')?.example).not.toMatch(/GitHub|GitLab/);
 });
});
