// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
export interface TaskPreset { id:string; label:string; type:string; tool:string; scope:string; example:string }
// Suggestions are derived from installed template capabilities, not agent names.
// Neither a preset nor its example grants authority or writes the objective.
export function taskPresets(tools:readonly string[]=[],scopes:readonly string[]=[]):TaskPreset[]{
 const candidates:TaskPreset[]=[
  {id:'web',label:'Market research',type:'public-research',tool:'web.read',scope:'web:public-pages',example:"Can Zoom's free plan support our team's weekly one-hour client meetings?"},
  {id:'crm',label:'Customer insights',type:'customer-insights',tool:'crm.read',scope:'crm:customer-retention',example:'Identify the three customers who need urgent follow-up, and explain the signals behind your priorities.'},
  {id:'public',label:'Product comparison',type:'public-research',tool:'web.fetch',scope:'web:product-comparison',example:'Compare the products using the configured public sources and summarize the key differences.'},
 ];
 const repository=scopes.find(s=>s.startsWith('repo:'));
 if(repository)candidates.push({id:'repository',label:'Incident investigation',type:'investigation',tool:tools.includes('git.read')?'git.read':'repo.read',scope:repository,example:'Investigate the incident using the permitted repository evidence and summarize your findings.'});
 return candidates.filter(p=>tools.includes(p.tool)&&scopes.includes(p.scope));
}
