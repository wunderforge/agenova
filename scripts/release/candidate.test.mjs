// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFileSync} from 'node:fs';
import {candidateGate,pairSnapshot,deployPair} from './candidate.mjs';
import {temporaryTrust} from './candidate-access.mjs';
import {canonical,SOURCE,workerSnapshot} from './release.mjs';
const A='a'.repeat(40), B='b'.repeat(40), old='sha256:'+'1'.repeat(64), next='sha256:'+'2'.repeat(64);
const c={namespace:'agenova-system',deployment:'agenova-member-control-plane',container:'control-plane',
 controlPlane:'931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/control-plane',
 worker:'931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/worker'};
function fixture(){return {metadata:{name:c.deployment,namespace:c.namespace,uid:'11111111-1111-1111-1111-111111111111',resourceVersion:'1',generation:1},
 spec:{replicas:1,strategy:{type:'Recreate'},template:{metadata:{labels:{app:'member'}},spec:{serviceAccountName:'unchanged',
 containers:[{name:c.container,image:c.controlPlane+'@'+old,readinessProbe:{httpGet:{path:'/readyz',port:8080}},
 env:[{name:'AGENOVA_AUTH_MODE',value:'member'},{name:'AGENOVA_MEMBER_CONFIG',value:'/etc/member/config'},{name:'AGENOVA_ALLOWED_WORKER_IMAGE',value:c.worker+'@'+old}],volumeMounts:[{name:'credentials-reference',mountPath:'/private'}]},
 {name:'proxy',image:'nginx@preserved'}],volumes:[{name:'credentials-reference',secret:{secretName:'never-read'}}]}}},
 status:{observedGeneration:1,replicas:1,updatedReplicas:1,availableReplicas:1}};}
const worker={metadata:{name:'agenova-template-slot',namespace:c.namespace,labels:{'app.kubernetes.io/managed-by':'agenova'}},data:{'template.json':JSON.stringify({metadata:{name:'engineer'},spec:{artifact:{image:c.worker+'@'+old}}})}};
const platform={data:{'effective-platform.json':'synthetic-installed-platform'}};
function expected(o){return {...pairSnapshot(o,c),worker:workerSnapshot(worker,c),platformHash:createHash('sha256').update(canonical(platform.data)).digest('hex')};}
function release(){return {mode:'first-compatible-pair',base:null,target:B,images:{controlPlane:c.controlPlane+'@'+next,worker:c.worker+'@'+next}};}
function cluster(initial=fixture(), options={}){
 let object=structuredClone(initial),rollouts=0;const calls=[];
 const run=(bin,args)=>{
  calls.push([bin,...args]);assert.equal(bin,'kubectl');assert.deepEqual(args.slice(0,2),['--namespace',c.namespace]);const a=args.slice(2);
  if(a[0]==='get'&&a[1]==='deployment')return JSON.stringify(object);
  if(a[0]==='get'&&a[1]==='configmap')return JSON.stringify(a[2]==='agenova-template-slot'?worker:platform);
  if(a[0]==='patch'){
   assert.equal(a[1],'deployment');assert.equal(a[2],c.deployment);
   const ops=JSON.parse(a[a.indexOf('--patch')+1]),clone=structuredClone(object);
   for(const op of ops){const parts=op.path.slice(1).split('/');let at=clone;for(const k of parts.slice(0,-1))at=at[k];const key=parts.at(-1);if(op.op==='test')assert.deepEqual(at[key],op.value);else{assert.ok(['add','replace'].includes(op.op));at[key]=op.value;}}
   clone.metadata.resourceVersion=String(Number(object.metadata.resourceVersion)+1);if(ops.some(o=>o.op==='replace'))clone.metadata.generation++;object=clone;
   if(options.lostPatch&&calls.filter(v=>v.includes('patch')).length===1)throw new Error('lost after accepted');
   return 'patched';
  }
  if(a[0]==='rollout'){
   rollouts++;if(options.fail&&rollouts===1){if(options.drift)object.spec.template.spec.serviceAccountName='other';throw new Error('timeout');}
   object.status.observedGeneration=object.metadata.generation;
   if(options.rebaselineAfterRecovery&&rollouts===2)object.metadata.annotations={[SOURCE]:A};
   return 'ready';
  }
  throw new Error('unexpected command');
 };
 return {run,calls,object:()=>object};
}
test('candidate requires explicit manual exact branch, SHA and UID; never PR/main push',()=>{
 const env={GITHUB_EVENT_NAME:'workflow_dispatch',GITHUB_REF:'refs/heads/codex/demo-cicd-integration-20261019',GITHUB_SHA:B,
 SHARED_EKS_CANDIDATE_ENABLED:'true',SHARED_EKS_CANDIDATE_REF:'refs/heads/codex/demo-cicd-integration-20261019',SHARED_EKS_CANDIDATE_SHA:B,SHARED_EKS_CANDIDATE_UID:fixture().metadata.uid};
 candidateGate(env);
 for(const change of [{GITHUB_EVENT_NAME:'pull_request'},{GITHUB_EVENT_NAME:'push'},{GITHUB_REF:'refs/heads/main'},{GITHUB_SHA:A},{SHARED_EKS_CANDIDATE_ENABLED:'false'},{SHARED_EKS_CANDIDATE_REF:'refs/heads/*'},{SHARED_EKS_CANDIDATE_UID:''}])assert.throws(()=>candidateGate({...env,...change}));
});
test('first pair preserves unknown source as null; binds all but two explicit image fields',()=>{
 const f=fixture(),s=expected(f);assert.equal(s.source,null);assert.ok(!JSON.stringify(s).includes('never-read'));
 f.spec.template.spec.containers[0].image=c.controlPlane+'@'+next;f.spec.template.spec.containers[0].env[2].value=c.worker+'@'+next;
 assert.equal(expected(f).binding,s.binding);
 for(const edit of [o=>o.spec.template.spec.containers[1].image='other',o=>o.spec.template.spec.serviceAccountName='other',o=>o.spec.template.spec.containers[0].env[0].value='reference']){const o=fixture();edit(o);try{assert.notEqual(expected(o).binding,s.binding);}catch(e){assert.match(e.message,/member control plane/);}}
});
test('pair rollout atomically changes image and worker binding, not immutable registry or credentials',()=>{
 const f=fixture(),cl=cluster(f),result=deployPair(release(),expected(f),c,cl.run);
 assert.equal(result.result,'ready');assert.equal(cl.object().metadata.annotations[SOURCE],B);
 const want=structuredClone(f.spec);want.template.spec.containers[0].image=c.controlPlane+'@'+next;want.template.spec.containers[0].env[2].value=c.worker+'@'+next;
 assert.deepEqual(cl.object().spec,want);assert.ok(cl.calls.every(v=>!v.includes('secret')&&!(v.includes('patch')&&v.includes('configmap'))));
});
test('drift, wrong base, malformed image and unhealthy target deny before mutation',()=>{
 for(const change of [o=>o.metadata.resourceVersion='2',o=>o.spec.template.spec.serviceAccountName='other',o=>o.status.availableReplicas=0]){const f=fixture(),s=expected(f);change(f);const cl=cluster(f);assert.throws(()=>deployPair(release(),s,c,cl.run));assert.equal(cl.calls.filter(v=>v.includes('patch')).length,0);}
 const f=fixture(),cl=cluster(f),r=release();r.base=A;assert.throws(()=>deployPair(r,expected(f),c,cl.run));r.base=null;r.images.worker='attacker:latest';assert.throws(()=>deployPair(r,expected(f),c,cl.run));assert.equal(cl.calls.length,0);
});
test('failed pair and lost patch acknowledgement restore both old image values and remain failed',()=>{
 for(const options of [{fail:true},{lostPatch:true}]){const f=fixture(),cl=cluster(f,options);assert.throws(()=>deployPair(release(),expected(f),c,cl.run),/failed-pair-restored/);assert.deepEqual(cl.object().spec,f.spec);assert.equal(cl.object().metadata.annotations?.[SOURCE],undefined);}
});
test('concurrent configuration changes and post-recovery rebaselining require operator',()=>{
 for(const options of [{fail:true,drift:true},{fail:true,rebaselineAfterRecovery:true}]){const f=fixture(),cl=cluster(f,options);assert.throws(()=>deployPair(release(),expected(f),c,cl.run),/needs-operator/);}
});
test('member release bundles Portal; candidate source/schema migration remains operator-owned',()=>{
 const docker=readFileSync(new URL('../../deploy/production/release.Dockerfile',import.meta.url),'utf8');assert.match(docker,/COPY --from=portal \/src\/ui\/dist \/opt\/agenova\/portal/);
 assert.match(docker,/COPY --from=go-tools \/usr\/local\/go \/usr\/local\/go/);assert.match(docker,/COPY internal \/src\/internal/);
 const script=readFileSync(new URL('./candidate.mjs',import.meta.url),'utf8');assert.ok(!script.includes('identity-migrate'));assert.ok(!script.includes("'get','secret'"));
});
test('temporary publisher trust adds only exact candidate and leaves original main policy untouched',()=>{
 const original={Version:'2012-10-17',Statement:[{Effect:'Allow',Action:'sts:AssumeRoleWithWebIdentity',Principal:{Federated:'unchanged'},Condition:{StringEquals:{'token.actions.githubusercontent.com:aud':'sts.amazonaws.com','token.actions.githubusercontent.com:sub':'repo:wunderforge@247966177/agenova@1263635683:ref:refs/heads/main'}}}]};
 const next=temporaryTrust(original,'refs/heads/codex/demo-cicd-integration-20261019');
 assert.equal(typeof original.Statement[0].Condition.StringEquals['token.actions.githubusercontent.com:sub'],'string');
 assert.deepEqual(next.Statement[0].Condition.StringEquals['token.actions.githubusercontent.com:sub'],['repo:wunderforge@247966177/agenova@1263635683:ref:refs/heads/main','repo:wunderforge@247966177/agenova@1263635683:ref:refs/heads/codex/demo-cicd-integration-20261019']);
 assert.throws(()=>temporaryTrust(original,'refs/heads/*'));assert.throws(()=>temporaryTrust(next,'refs/heads/codex/other'));
});
