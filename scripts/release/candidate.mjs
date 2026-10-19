// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Explicit, SHA-pinned first compatible-pair rollout. No Secret or registry writes.
import {readFileSync, writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {config, command, connect, sha, image, canonical, snapshot, workerSnapshot, ready, publish, SOURCE} from './release.mjs';

function requireValue(ok, message) { if (!ok) throw new Error(message); }
export function candidateGate(env = process.env) {
  requireValue(env.GITHUB_EVENT_NAME === 'workflow_dispatch', 'Candidate releases are manual only');
  requireValue(env.SHARED_EKS_CANDIDATE_ENABLED === 'true', 'Candidate release disabled');
  const ref = env.SHARED_EKS_CANDIDATE_REF;
  requireValue(/^refs\/heads\/codex\/[a-z0-9-]+$/.test(ref ?? '') && env.GITHUB_REF === ref, 'Exact authorized candidate branch required');
  requireValue(sha(env.GITHUB_SHA) === sha(env.SHARED_EKS_CANDIDATE_SHA), 'Candidate SHA changed; authorize the exact tested commit');
  requireValue(/^[a-f0-9-]{36}$/.test(env.SHARED_EKS_CANDIDATE_UID ?? ''), 'Pinned Deployment UID required');
}
const hash = value => createHash('sha256').update(canonical(value)).digest('hex');
const save = (name, value) => writeFileSync(name, JSON.stringify(value, null, 2)+'\n');
const load = name => JSON.parse(readFileSync(name, 'utf8'));
export function pairSnapshot(object, c) {
  const copy = structuredClone(object);
  const container = copy.spec?.template?.spec?.containers?.find(v => v.name === c.container);
  const indexes = container?.env?.flatMap((v,i) => v.name === 'AGENOVA_ALLOWED_WORKER_IMAGE' ? [i] : []) ?? [];
  requireValue(indexes.length === 1, 'One explicit compatible Worker image required');
  const workerIndex = indexes[0];
  const workerImage = image(container.env[workerIndex].value, c.worker);
  // Only this one non-secret value and the CP image may change. Everything else
  // remains hashed, including Secret references, auth, mounts, probes and sidecars.
  container.env[workerIndex].value = 'compatible-pair-binding';
  return {...snapshot(copy, c, undefined, true), workerIndex, workerImage};
}
function cluster(c, run) {
  const k = (...args) => run('kubectl', ['--namespace', c.namespace, ...args]);
  const get = () => JSON.parse(k('get','deployment',c.deployment,'-o','json'));
  const dependencies = () => ({
    worker: workerSnapshot(JSON.parse(k('get','configmap','agenova-template-slot','-o','json')), c),
    platformHash: hash(JSON.parse(k('get','configmap','agenova-platform','-o','json')).data),
  });
  const patch = ops => k('patch','deployment',c.deployment,'--type=json','--patch',JSON.stringify(ops),'-o','name');
  return {k,get,dependencies,patch};
}
function imagePatch(s, cp, worker) {
  const root = `/spec/template/spec/containers/${s.index}`;
  return [
    {op:'test',path:'/metadata/uid',value:s.uid},
    {op:'test',path:'/metadata/resourceVersion',value:s.resourceVersion},
    {op:'test',path:root+'/image',value:s.image},
    {op:'test',path:root+`/env/${s.workerIndex}/name`,value:'AGENOVA_ALLOWED_WORKER_IMAGE'},
    {op:'test',path:root+`/env/${s.workerIndex}/value`,value:s.workerImage},
    {op:'replace',path:root+'/image',value:cp},
    {op:'replace',path:root+`/env/${s.workerIndex}/value`,value:worker},
  ];
}
function exactState(s, expected, cp, worker, source) {
  return s.uid === expected.uid && s.binding === expected.binding && s.image === cp &&
    s.workerIndex === expected.workerIndex && s.workerImage === worker && s.recordedSource === source;
}
export function deployPair(release, expected, c, run = command, record = () => {}) {
  sha(release.target);
  requireValue(release.mode === 'first-compatible-pair' && release.base === expected.source, 'Wrong candidate manifest');
  const cp = image(release.images.controlPlane, c.controlPlane), worker = image(release.images.worker, c.worker);
  const {k,get,dependencies,patch} = cluster(c, run);
  const initial = get(), current = {...pairSnapshot(initial,c), ...dependencies()};
  requireValue(canonical(current) === canonical(expected) && ready(initial), 'Target drifted or is not healthy');
  const evidence = {source:release.target,previousSource:expected.source,previousImage:expected.image,
    previousWorkerImage:expected.workerImage,desiredImage:cp,desiredWorkerImage:worker,
    registry:'unchanged; new immutable project template registration is a separate API operation',result:'planned'};
  record(evidence);
  let attempted = false;
  try {
    attempted = true;
    patch(imagePatch(current,cp,worker));
    evidence.result='rollout-started'; record(evidence);
    k('rollout','status',`deployment/${c.deployment}`,'--timeout=180s');
    const object=get(), after=pairSnapshot(object,c);
    requireValue(exactState(after,expected,cp,worker,expected.recordedSource) && ready(object), 'Pair rollout binding/health mismatch');
    requireValue(canonical(dependencies()) === canonical({worker:expected.worker,platformHash:expected.platformHash}), 'Registry/Platform drift');
    patch([
      {op:'test',path:'/metadata/uid',value:after.uid},
      {op:'test',path:'/metadata/resourceVersion',value:after.resourceVersion},
      {op:'add',path:'/metadata/annotations',value:{...(object.metadata.annotations??{}),[SOURCE]:release.target}},
    ]);
    const final=get();
    requireValue(exactState(pairSnapshot(final,c),expected,cp,worker,release.target) && ready(final), 'Final pair/source verification failed');
    evidence.result='ready'; record(evidence); return evidence;
  } catch {
    evidence.result='failed';
    if (attempted) {
      try {
        const obj=get(), latest=pairSnapshot(obj,c);
        requireValue(exactState(latest,expected,cp,worker,expected.recordedSource), 'Concurrent update prevents pair recovery');
        requireValue(canonical(dependencies()) === canonical({worker:expected.worker,platformHash:expected.platformHash}), 'Dependency drift prevents recovery');
        patch(imagePatch(latest,expected.image,expected.workerImage));
        k('rollout','status',`deployment/${c.deployment}`,'--timeout=180s');
        const restored=get();
        requireValue(exactState(pairSnapshot(restored,c),expected,expected.image,expected.workerImage,expected.recordedSource) && ready(restored), 'Recovery verification failed');
        requireValue(canonical(dependencies()) === canonical({worker:expected.worker,platformHash:expected.platformHash}), 'Dependency drift after recovery');
        evidence.result='failed-pair-restored';
      } catch { evidence.result='failed-recovery-needs-operator'; }
    }
    record(evidence); throw new Error(`Candidate rollout failed: ${evidence.result}`);
  }
}
export function main(args=process.argv.slice(2)) {
  candidateGate();
  const c=config(), target=sha(process.env.GITHUB_SHA), action=args[0];
  // Do not simulate main or an origin SHA. Candidate first publication deliberately
  // builds BOTH images and records an unknown prior source as null, not a fake base.
  if(action==='snapshot') {
    connect(c);
    const {get,dependencies}=cluster(c,command), object=get();
    const s={...pairSnapshot(object,c),...dependencies()};
    requireValue(s.uid===process.env.SHARED_EKS_CANDIDATE_UID && ready(object), 'Wrong/unhealthy pinned target');
    requireValue(s.image===image(process.env.SHARED_EKS_CANDIDATE_PREVIOUS_IMAGE,c.controlPlane), 'Current image differs from authorized starting image');
    save('snapshot.json',s);
  } else if(action==='publish') {
    requireValue(command('git',['rev-parse','HEAD'])===target,'Checkout differs from candidate');
    const s=load('snapshot.json');
    const releasePlan={mode:'first-compatible-pair',base:s.source,target,changed:{controlPlane:true,worker:true}};
    save('plan.json',releasePlan);
    const release=publish(releasePlan,c);
    release.previousControlPlaneImage=s.image; release.previousWorkerImage=s.workerImage;
    release.candidatePair={controlPlane:release.images.controlPlane,worker:release.images.worker};
    save('release.json',release);
  } else if(action==='deploy') {
    const release=load('release.json'); requireValue(release.target===target,'Artifact target mismatch');
    const ref=process.env.GITHUB_REF;
    const remote=command('git',['ls-remote','--exit-code','origin',ref]).split(/\s+/)[0];
    requireValue(remote===target,'Candidate branch advanced; rerun the tested revision');
    connect(c); deployPair(release,load('snapshot.json'),c,command,d=>save('deployment.json',d));
  } else throw new Error('Expected snapshot, publish or deploy');
}
if(process.argv[1] && import.meta.url===pathToFileURL(resolve(process.argv[1])).href) {
  try{ main(); }catch(error){ console.error(error.status!==undefined || error.code || error instanceof SyntaxError ? 'Candidate subprocess/input failed; sensitive output suppressed' : error.message); process.exitCode=1; }
}
