// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { View } from './connected-source';

export interface PermissionRow { id:string; label:string; requested:string; granted:string; state:'granted'|'not-granted'|'pending'|'unknown' }

// Historical request and issued authority only. Catalog capabilities, task text,
// current policy and execution success cannot manufacture a requested permission.
export function workPermissions(work:View):PermissionRow[]{
 const requested=work.request.spec.requestedAccess;
 const authority=work.state?.effectiveAuthority;
 const denied=work.state?.decision.result==='Deny';
 const rows:PermissionRow[]=[];
 function add(id:string,label:string,value:string|undefined|null,granted:string|undefined|null){
  if(!value)return;
  const state:PermissionRow['state']=authority ? granted?'granted':'not-granted' : denied?'not-granted':work.state?'unknown':'pending';
  rows.push({id,label,requested:value,granted:state==='granted'?granted!:state==='not-granted'?'Not granted':state==='pending'?'Pending':'Not recorded',state});
 }
 for(const [key,label] of [['tools','Tools'],['resourceScopes','Resources'],['networkProfiles','Network'],['memoryScopes','Memory']] as const){
  for(const value of requested?.[key]||[])add(`${key}:${value}`,label,value,authority?.[key]?.includes(value)?value:undefined);
 }
 add('model','Model',requested?.modelProfile,authority?.modelProfile);
 add('runtime','Runtime',work.request.spec.runtime?.profileRef,authority?.runtime?.profileRef);
 add('timeout','Time limit',work.request.spec.runtime?.timeout,authority?.runtime?.timeout);
 return rows;
}
