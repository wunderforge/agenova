// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Operator-side temporary OIDC authorization. Never changes permissions policies.
import {readFileSync,writeFileSync,existsSync,mkdirSync} from 'node:fs';
import {resolve,dirname} from 'node:path';
import {pathToFileURL} from 'node:url';
import {command,canonical,sha} from './release.mjs';
const repo='wunderforge/agenova', role='agenova-demo-ci-publisher';
const statePath=resolve('.tmp/cicd-verification/candidate-access.json');
const subject='token.actions.githubusercontent.com:sub';
function requireValue(ok,message){if(!ok)throw new Error(message);}
export function temporaryTrust(original,ref){
 requireValue(/^refs\/heads\/codex\/[a-z0-9-]+$/.test(ref),'Exact candidate branch required');
 const next=structuredClone(original);let changed=0;
 for(const statement of next.Statement??[]){
  const condition=statement.Condition?.StringEquals, current=condition?.[subject];
  if(typeof current==='string' && current==='repo:wunderforge@247966177/agenova@1263635683:ref:refs/heads/main'){
   requireValue(statement.Action==='sts:AssumeRoleWithWebIdentity' && condition['token.actions.githubusercontent.com:aud']==='sts.amazonaws.com','Unexpected trust shape');
   condition[subject]=[current,current.replace(/refs\/heads\/main$/,ref)];changed++;
  }
 }
 requireValue(changed===1,'Expected unchanged main publisher trust; coordinate existing changes first');return next;
}
const aws=(...args)=>command('aws',[...args,'--profile','agenova-neo','--region','ap-southeast-2','--output','json']);
const gh=(...args)=>command('gh',args);
const persist=s=>{mkdirSync(dirname(statePath),{recursive:true});writeFileSync(statePath,JSON.stringify(s,null,2)+'\n');};
function roleTrust(){return JSON.parse(aws('iam','get-role','--role-name',role)).Role.AssumeRolePolicyDocument;}
function setVar(name,value){gh('variable','set',name,'--repo',repo,'--body',value);}
function policies(){return JSON.parse(gh('api',`repos/${repo}/environments/shared-eks/deployment-branch-policies`)).branch_policies;}
export function main(args=process.argv.slice(2)){
 const [action,ref,target,uid,previousImage]=args;
 const identity=JSON.parse(aws('sts','get-caller-identity'));
 requireValue(identity.Account==='931228356546','Wrong AWS account');
 if(action==='authorize'){
  sha(target);requireValue(/^[a-f0-9-]{36}$/.test(uid??''),'Pinned Deployment UID required');
  requireValue(/^931228356546\.dkr\.ecr\.ap-southeast-2\.amazonaws\.com\/agenova-demo\/control-plane@sha256:[a-f0-9]{64}$/.test(previousImage??''),'Pinned initial control-plane digest required');
  requireValue(!existsSync(statePath),'An access receipt already exists; revoke that exact authorization first');
  const original=roleTrust(),temporary=temporaryTrust(original,ref);
  const s={ref,target,uid,original,temporary,branchPolicyCreated:null,trustApplied:false,revoked:false};persist(s);
  setVar('SHARED_EKS_CANDIDATE_ENABLED','false');
  aws('iam','update-assume-role-policy','--role-name',role,'--policy-document',JSON.stringify(temporary));s.trustApplied=true;persist(s);
  const name=ref.slice('refs/heads/'.length);
  if(!policies().some(p=>p.name===name&&p.type==='branch')){
   const created=JSON.parse(command('gh',['api',`repos/${repo}/environments/shared-eks/deployment-branch-policies`,'--method','POST','--input','-'],{input:JSON.stringify({name,type:'branch'})}));
   s.branchPolicyCreated=created.id;persist(s);
  }
  for(const [name,value] of Object.entries({SHARED_EKS_CANDIDATE_REF:ref,SHARED_EKS_CANDIDATE_SHA:target,SHARED_EKS_CANDIDATE_UID:uid,SHARED_EKS_CANDIDATE_PREVIOUS_IMAGE:previousImage}))setVar(name,value);
  setVar('SHARED_EKS_CANDIDATE_ENABLED','true');console.log(JSON.stringify({authorized:ref,source:target,permissionPoliciesChanged:false}));
 }else if(action==='revoke'){
  requireValue(existsSync(statePath),'No matching authorization receipt');const s=JSON.parse(readFileSync(statePath,'utf8'));
  setVar('SHARED_EKS_CANDIDATE_ENABLED','false');
  const current=roleTrust();
  requireValue(canonical(current)===canonical(s.temporary)||canonical(current)===canonical(s.original),'Publisher trust changed concurrently; manual reconciliation required');
  if(canonical(current)!==canonical(s.original))aws('iam','update-assume-role-policy','--role-name',role,'--policy-document',JSON.stringify(s.original));
  if(s.branchPolicyCreated&&policies().some(p=>p.id===s.branchPolicyCreated&&p.name===s.ref.slice('refs/heads/'.length)))gh('api',`repos/${repo}/environments/shared-eks/deployment-branch-policies/${s.branchPolicyCreated}`,'--method','DELETE');
  s.revoked=true;persist(s);console.log(JSON.stringify({revoked:s.ref,candidateEnabled:false,mainTrustRestored:true}));
 }else throw new Error('Expected authorize <ref> <sha> <uid> <old-image>, or revoke');
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){try{main();}catch(e){console.error(e.status!==undefined||e.code?'Temporary authorization operation failed; inspect the local non-secret receipt':e.message);process.exitCode=1;}}
