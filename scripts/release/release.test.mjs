// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
import { SOURCE, config, writeGate, snapshot as deploymentSnapshot, workerSnapshot, ready, changedComponents, plan, publish, existingDigest, deploy, eventEligible } from './release.mjs';

const A = 'a'.repeat(40), B = 'b'.repeat(40);
const OLD = `sha256:${'1'.repeat(64)}`, NEW = `sha256:${'2'.repeat(64)}`;
const env = {
  AWS_ACCOUNT_ID: '931228356546', AWS_REGION: 'ap-southeast-2',
  EKS_CLUSTER_NAME: 'agenova-demo', K8S_NAMESPACE: 'agenova-system',
  CONTROL_PLANE_DEPLOYMENT: 'agenova-member-control-plane', CONTROL_PLANE_CONTAINER: 'control-plane',
  ECR_CONTROL_PLANE_REPOSITORY: '931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/control-plane',
  ECR_WORKER_REPOSITORY: '931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/worker',
};
const c = config(env);
function workerFixture() {
  return { metadata: { name: 'agenova-template-slot', namespace: c.namespace, labels: { 'app.kubernetes.io/managed-by': 'agenova' } },
    data: { 'template.json': JSON.stringify({ spec: { artifact: { image: `${c.worker}@${OLD}` } } }) } };
}
function snapshot(object, settings, baseline) {
  return deploymentSnapshot(object, settings, baseline);
}
const paths = { controlPlane: ['cmd/agenova-control-plane', 'internal/app', 'internal/workerprotocol'], worker: ['cmd/agenova-demo-worker', 'internal/workerprotocol', 'harness/integration/agentsandbox/testworker'] };
function fixture() {
  return {
    metadata: { name: c.deployment, namespace: c.namespace, uid: 'uid-1', resourceVersion: '1', generation: 1,
      annotations: { 'preserve.example/key': 'keep', [SOURCE]: A } },
    spec: { replicas: 1, strategy: { type: 'Recreate' }, template: { metadata: { labels: { app: 'member' } }, spec: {
      serviceAccountName: 'member', containers: [
        { name: 'proxy', image: 'nginx@sha256:preserved' },
        { name: c.container, image: `${c.controlPlane}@${OLD}`, readinessProbe: { httpGet: { path: '/healthz', port: 8080 } },
          env: [{ name: 'AGENOVA_AUTH_MODE', value: 'member' }, { name: 'AGENOVA_MEMBER_CONFIG', value: '/member/config' }],
          volumeMounts: [{ name: 'credential-ref-only', mountPath: '/credential' }] },
      ], volumes: [{ name: 'credential-ref-only', secret: { secretName: 'not-read-by-release' } }],
    } } },
    status: { observedGeneration: 1, replicas: 1, updatedReplicas: 1, availableReplicas: 1 },
  };
}
function candidate(changed = { controlPlane: true, worker: false }) {
  return { base: A, target: B, changed, images: { controlPlane: `${c.controlPlane}@${NEW}` }, workerRolloutRequired: false };
}
function fakeCluster(initial = fixture(), { failRollout = false, driftOnFailure = false, activeClaims = [] } = {}) {
  let object = structuredClone(initial), rollouts = 0;
  const calls = [];
  const run = (bin, args) => {
    calls.push([bin, ...args]);
    assert.equal(bin, 'kubectl');
    assert.deepEqual(args.slice(0, 2), ['--namespace', c.namespace]);
    const a = args.slice(2);
    if (a[0] === 'get' && a[1] === 'sandboxclaims.extensions.agents.x-k8s.io') return JSON.stringify({ items: activeClaims });
    if (a[0] === 'get' && a[1] === 'configmap') { assert.deepEqual(a, ['get', 'configmap', 'agenova-template-slot', '-o', 'json']); return JSON.stringify(workerFixture()); }
    if (a[0] === 'get') { assert.deepEqual(a, ['get', 'deployment', c.deployment, '-o', 'json']); return JSON.stringify(object); }
    if (a[0] === 'patch') {
      assert.equal(a[2], c.deployment);
      const ops = JSON.parse(a[a.indexOf('--patch') + 1]);
      // JSON Patch test failure is atomic; no earlier replace can partially apply.
      let next = structuredClone(object);
      for (const op of ops) {
        const parts = op.path.slice(1).split('/');
        let at = next;
        for (const p of parts.slice(0, -1)) at = at[p];
        const key = parts.at(-1);
        if (op.op === 'test') assert.deepEqual(at[key], op.value);
        else { assert.ok(['replace', 'add'].includes(op.op)); at[key] = op.value; }
      }
      next.metadata.resourceVersion = String(Number(object.metadata.resourceVersion) + 1);
      if (ops.some(op => op.op === 'replace')) next.metadata.generation++;
      object = next;
      return `deployment.apps/${c.deployment}`;
    }
    if (a[0] === 'rollout') {
      rollouts++;
      if (failRollout && rollouts === 1) {
        if (driftOnFailure) object.spec.template.spec.serviceAccountName = 'changed-by-operator';
        throw new Error('simulated timeout');
      }
      object.status.observedGeneration = object.metadata.generation;
      return 'ready';
    }
    throw new Error('Unexpected command');
  };
  return { run, calls, object: () => object };
}

test('main defaults on; PRs, non-main, explicit opt-out and invalid switches fail closed', () => {
  const valid = { GITHUB_REF: 'refs/heads/main', GITHUB_EVENT_NAME: 'push' };
  writeGate(valid, true);
  writeGate({ GITHUB_REF: 'refs/heads/main', GITHUB_EVENT_NAME: 'push' }, true);
  writeGate({ ...valid, SHARED_EKS_RELEASE_ENABLED: 'false', SHARED_EKS_DEPLOY_ENABLED: 'false' }, true); // obsolete opt-in variables
  for (const mutation of [{ GITHUB_REF: 'refs/pull/1/merge' }, { GITHUB_EVENT_NAME: 'pull_request' }, { SHARED_EKS_RELEASE_DISABLED: 'true' }, { SHARED_EKS_DEPLOY_DISABLED: 'true' }, { SHARED_EKS_RELEASE_DISABLED: 'typo' }]) {
    assert.throws(() => writeGate({ ...valid, ...mutation }, true));
  }
});
test('reject foreign repositories and injected target strings', () => {
  for (const mutation of [{ AWS_ACCOUNT_ID: '123' }, { ECR_WORKER_REPOSITORY: 'attacker/worker' }, { K8S_NAMESPACE: 'x; kubectl delete' }, { ECR_WORKER_REPOSITORY: c.controlPlane }]) {
    assert.throws(() => config({ ...env, ...mutation }));
  }
});
test('docs and unit tests require no images', () => {
  assert.deepEqual(changedComponents(['README.md', 'docs/demo.md', 'internal/app/app_test.go', 'ui/README.md', 'ui/tests/portal.spec.ts'], paths), { controlPlane: false, worker: false });
});
test('event preflight rejects PRs and skips docs-only pushes without a cloud command', () => {
  const env = { GITHUB_REF: 'refs/heads/main', GITHUB_EVENT_NAME: 'push', EVENT_BEFORE_SHA: A, GITHUB_SHA: B };
  const run = (bin, args) => {
    assert.ok(['git', 'go'].includes(bin));
    if (args[0] === 'rev-parse') return B;
    if (args[0] === 'merge-base') return '';
    if (bin === 'go') return 'github.com/wunderforge/agenova/internal/app';
    if (args[0] === 'diff') return 'ui/README.md\0internal/app/app_test.go\0';
    throw Error('unexpected');
  };
  assert.equal(eventEligible(env, run), false);
  assert.throws(() => eventEligible({ ...env, GITHUB_EVENT_NAME: 'pull_request' }, run));
  assert.equal(eventEligible({ ...env, GITHUB_EVENT_NAME: 'workflow_dispatch' }, () => { throw Error('offline'); }), true);
});
test('control-plane-only changes build only control plane', () => {
  assert.deepEqual(changedComponents(['internal/app/app.go'], paths), { controlPlane: true, worker: false });
});
test('worker-only changes build only Worker', () => {
  assert.deepEqual(changedComponents(['cmd/agenova-demo-worker/main.go'], paths), { controlPlane: false, worker: true });
});
test('shared protocol, dependency locks and context exclusions build both', () => {
  for (const file of ['internal/workerprotocol/protocol.go', 'go.mod', 'go.sum', '.dockerignore']) {
    assert.deepEqual(changedComponents([file], paths), { controlPlane: true, worker: true });
  }
});
test('image Dockerfiles select their own image; Windows paths normalize', () => {
  assert.deepEqual(changedComponents(['deploy/reference/Dockerfile'], paths), { controlPlane: true, worker: false });
  assert.deepEqual(changedComponents(['deploy/production/release.Dockerfile', 'ui/src/Portal.tsx'], paths), { controlPlane: true, worker: false });
  assert.deepEqual(changedComponents(['harness\\integration\\agentsandbox\\testworker\\Dockerfile'], paths), { controlPlane: false, worker: true });
});
test('planning diffs from installed baseline and derives package dependencies', () => {
  const calls = [];
  const run = (bin, args) => {
    calls.push([bin, ...args]);
    if (args[0] === 'rev-parse') return B;
    if (args[0] === 'merge-base') return '';
    if (bin === 'go') return 'github.com/wunderforge/agenova/internal/workerprotocol';
    if (args[0] === 'diff') { assert.deepEqual(args.slice(-2), [A, B]); return 'internal/workerprotocol/protocol.go\0'; }
    throw new Error('Unexpected');
  };
  assert.deepEqual(plan(A, B, run).changed, { controlPlane: true, worker: true });
  assert.equal(calls.filter(v => v[0] === 'go').length, 2);
});
test('missing/non-ancestor baseline and mismatched checkout fail closed', () => {
  assert.throws(() => plan('', B, () => { throw new Error('must not run'); }));
  assert.throws(() => plan(A, B, () => A), /Checkout/);
  assert.throws(() => plan(A, B, (bin, args) => { if (args[0] === 'rev-parse') return B; throw new Error('not ancestor'); }), /not ancestor/);
});
test('snapshot binds configuration, excludes image only and emits no Pod config', () => {
  const f = fixture();
  const s = snapshot(f, c);
  f.spec.template.spec.containers[1].image = `${c.controlPlane}@${NEW}`;
  assert.equal(snapshot(f, c).binding, s.binding);
  f.spec.template.spec.containers[0].image = 'different-proxy';
  assert.notEqual(snapshot(f, c).binding, s.binding);
  assert.ok(!JSON.stringify(s).includes('credential'));
  assert.equal(s.index, 1);
});
test('first deployment requires explicit baseline; rejects unsafe installation shapes', () => {
  const f = fixture(); delete f.metadata.annotations[SOURCE];
  assert.throws(() => snapshot(f, c), /SHA/);
  assert.equal(snapshot(f, c, A).source, A);
  for (const change of [o => { o.spec.replicas = 2; }, o => { o.spec.strategy.type = 'RollingUpdate'; }, o => { o.spec.template.spec.containers[1].env = []; }, o => { delete o.spec.template.spec.containers[1].readinessProbe; }, o => { o.metadata.deletionTimestamp = 'now'; }]) {
    const obj = fixture(); change(obj); assert.throws(() => snapshot(obj, c));
  }
});
test('immutable ECR tags are reused without docker login/build/push', () => {
  const calls = [];
  const r = publish(candidate(), c, (bin, args) => { calls.push([bin, ...args]); assert.equal(bin, 'aws'); assert.equal(args[1], 'describe-images'); return NEW; });
  assert.equal(r.images.controlPlane, `${c.controlPlane}@${NEW}`);
  assert.equal(calls.length, 1);
});
test('only ImageNotFound triggers a build; authentication errors do not', () => {
  assert.equal(existingDigest('r', 't', () => { throw Object.assign(new Error(), { stderr: '(ImageNotFoundException)' }); }), null);
  assert.throws(() => existingDigest('r', 't', () => { throw Object.assign(new Error(), { stderr: '(AccessDeniedException)' }); }), /lookup failed/);
});
test('publication selects only changed image, full SHA tag, stdin login and digest', () => {
  const calls = []; let lookups = 0;
  const result = publish(candidate(), c, (bin, args, options) => {
    calls.push([bin, args, options]);
    if (args[1] === 'describe-images') { if (++lookups === 1) throw Object.assign(new Error(), { stderr: '(ImageNotFoundException)' }); return NEW; }
    if (args[1] === 'get-login-password') return 'synthetic-password';
    return '';
  });
  const login = calls.find(v => v[1][0] === 'login');
  assert.equal(login[2].input, 'synthetic-password');
  assert.ok(!login[1].includes('synthetic-password'));
  const build = calls.find(v => v[1][0] === 'buildx')[1];
  assert.ok(build.includes(`${c.controlPlane}:sha-${B}`));
  assert.ok(build.includes('linux/amd64'));
  assert.deepEqual(calls.find(v => v[1][0] === 'buildx')[2].stdio, ['ignore', 'inherit', 'inherit']);
  assert.ok(!result.images.worker);
});
test('no component changes perform no ECR/docker operations', () => {
  const result = publish(candidate({ controlPlane: false, worker: false }), c, () => { throw new Error('must not run'); });
  assert.deepEqual(result.images, {});
});
test('successful deployment changes only selected image and records source after readiness', () => {
  const f = fixture(), cluster = fakeCluster(f), evidence = [];
  deploy(candidate(), snapshot(f, c), c, cluster.run, v => evidence.push(structuredClone(v)));
  const actual = cluster.object();
  assert.equal(actual.spec.template.spec.containers[1].image, `${c.controlPlane}@${NEW}`);
  assert.equal(actual.metadata.annotations[SOURCE], B);
  assert.equal(actual.metadata.annotations['preserve.example/key'], 'keep');
  const expected = structuredClone(f.spec); expected.template.spec.containers[1].image = `${c.controlPlane}@${NEW}`;
  assert.deepEqual(actual.spec, expected);
  assert.equal(evidence.at(-1).result, 'ready');
  assert.ok(cluster.calls.every(v => !v.includes('secret') && !(v.includes('patch') && v.includes('configmap'))));
});
test('Worker publication does not block a control-plane release', () => {
  const f = fixture(), cluster = fakeCluster(f);
  deploy(candidate({ controlPlane: true, worker: true }), snapshot(f, c), c, cluster.run);
  assert.equal(cluster.object().spec.template.spec.containers[1].image, `${c.controlPlane}@${NEW}`);
  assert.ok(cluster.calls.every(v => !v.includes('configmap')));
});
test('Worker-only release never accesses cluster or advances platform source', () => {
  const f = fixture(), cluster = fakeCluster(f);
  const result = deploy(candidate({ controlPlane: false, worker: true }), snapshot(f, c), c, cluster.run);
  assert.equal(result.result, 'published-no-platform-change');
  assert.equal(cluster.calls.length, 0);
  assert.equal(cluster.object().metadata.annotations[SOURCE], A);
});
test('Worker registration must be managed and digest-pinned to the expected repository', () => {
  const w = workerFixture();
  assert.equal(workerSnapshot(w, c).image, `${c.worker}@${OLD}`);
  w.data['template.json'] = JSON.stringify({ spec: { artifact: { image: `${c.worker}:latest` } } });
  assert.throws(() => workerSnapshot(w, c), /repository-bound/);
  delete w.metadata.labels['app.kubernetes.io/managed-by'];
  assert.throws(() => workerSnapshot(w, c), /managed Worker/);
});
test('ordinary release does not interpret deprecated global image hints', () => {
  const f=fixture();
  f.spec.template.spec.containers[1].env.push({name:'AGENOVA_ALLOWED_WORKER_IMAGE',value:`${c.worker}@${NEW}`});
  const s=snapshot(f,c);
  assert.ok(!('worker' in s) && !('compatibleWorkerImage' in s));
  f.spec.template.spec.containers[1].env.at(-1).value='foreign:latest';
  assert.doesNotThrow(()=>snapshot(f,c));
});
test('wrong artifact base or foreign image rejected before cluster access', () => {
  const f = fixture(), cluster = fakeCluster(f);
  const wrong = candidate(); wrong.base = B;
  assert.throws(() => deploy(wrong, snapshot(f, c), c, cluster.run), /baseline/);
  wrong.base = A; wrong.images.controlPlane = `attacker/control-plane@${NEW}`;
  assert.throws(() => deploy(wrong, snapshot(f, c), c, cluster.run), /repository/);
  assert.equal(cluster.calls.length, 0);
});
test('concurrent resource-version, image or Pod configuration changes reject before patch', () => {
  for (const mutate of [o => { o.metadata.resourceVersion = '2'; }, o => { o.spec.template.spec.containers[1].image = `${c.controlPlane}@${NEW}`; }, o => { o.spec.template.spec.serviceAccountName = 'other'; }]) {
    const f = fixture(), expected = snapshot(f, c); mutate(f); const cluster = fakeCluster(f);
    assert.throws(() => deploy(candidate(), expected, c, cluster.run), /changed since snapshot/);
    assert.equal(cluster.calls.filter(v => v.includes('patch')).length, 0);
  }
});
test('unhealthy baseline rejected', () => {
  const f = fixture(); f.status.availableReplicas = 0;
  assert.equal(ready(f), false);
  const cluster = fakeCluster(f);
  assert.throws(() => deploy(candidate(), snapshot(f, c), c, cluster.run), /not healthy/);
});
test('rollout failure restores previous image and still fails the release', () => {
  const f = fixture(), cluster = fakeCluster(f, { failRollout: true }), evidence = [];
  assert.throws(() => deploy(candidate(), snapshot(f, c), c, cluster.run, v => evidence.push(structuredClone(v))), /failed-image-restored/);
  assert.equal(cluster.object().spec.template.spec.containers[1].image, `${c.controlPlane}@${OLD}`);
  assert.equal(cluster.object().metadata.annotations[SOURCE], A);
  assert.equal(evidence.at(-1).result, 'failed-image-restored');
});
test('recovery refuses to overwrite concurrent operator configuration changes', () => {
  const f = fixture(), cluster = fakeCluster(f, { failRollout: true, driftOnFailure: true });
  assert.throws(() => deploy(candidate(), snapshot(f, c), c, cluster.run), /needs-operator/);
  assert.equal(cluster.calls.filter(v => v.includes('patch')).length, 1);
  assert.equal(cluster.object().spec.template.spec.serviceAccountName, 'changed-by-operator');
});
test('same-digest rerun does not replace image or alter Pod template', () => {
  const f = fixture(); f.spec.template.spec.containers[1].image = `${c.controlPlane}@${NEW}`;
  const cluster = fakeCluster(f);
  deploy(candidate(), snapshot(f, c), c, cluster.run);
  assert.deepEqual(cluster.object().spec.template, f.spec.template);
  assert.equal(cluster.calls.filter(v => v.includes('patch')).length, 1); // source annotation only
});
test('lost patch response can recover only the exact attempted image', () => {
  const f = fixture(), cluster = fakeCluster(f); let lost = false;
  const run = (bin, args) => {
    const response = cluster.run(bin, args);
    if (args.includes('patch') && !lost) { lost = true; throw new Error('response lost after apply'); }
    return response;
  };
  assert.throws(() => deploy(candidate(), snapshot(f, c), c, run), /failed-image-restored/);
  assert.equal(cluster.object().spec.template.spec.containers[1].image, `${c.controlPlane}@${OLD}`);
});
test('active or malformed claim inventory prevents rollout before mutations', () => {
  const f = fixture(), cluster = fakeCluster(f, { activeClaims: [{ metadata: { name: 'running-work' } }] });
  assert.throws(() => deploy(candidate(), snapshot(f, c), c, cluster.run), /Active SandboxClaims/);
  assert.equal(cluster.calls.filter(v => v.includes('patch')).length, 0);
});
test('workflow guards and credential separation remain explicit', () => {
  const yaml = readFileSync(new URL('../../.github/workflows/shared-eks-release.yml', import.meta.url), 'utf8');
  assert.ok(!yaml.includes('pull_request:'));
  assert.match(yaml, /cancel-in-progress: false/);
  assert.match(yaml, /vars.SHARED_EKS_RELEASE_DISABLED == 'false'/);
  assert.match(yaml, /vars.SHARED_EKS_DEPLOY_DISABLED == 'false'/);
  assert.ok(!yaml.includes('vars.SHARED_EKS_RELEASE_ENABLED'));
  assert.match(yaml, /needs.validate.outputs.eligible == 'true'/);
  assert.ok(yaml.indexOf('release.mjs preflight') < yaml.indexOf('configure-aws-credentials'));
  assert.ok(!yaml.includes('blocked-worker-migration'));
  const publishing = yaml.split('\n  publish:')[1].split('\n  deploy:')[0];
  assert.ok(!/^    environment:/m.test(publishing));
  assert.match(publishing, /AWS_PUBLISH_ROLE_ARN/);
  const deployment = yaml.split('\n  deploy:')[1];
  assert.match(deployment, /environment: shared-eks/);
  assert.match(deployment, /AWS_DEPLOY_ROLE_ARN/);
  for (const match of yaml.matchAll(/uses: ([^\n ]+)/g)) assert.match(match[1], /@[a-f0-9]{40}$/);
  const ci = readFileSync(new URL('../../.github/workflows/ci.yml', import.meta.url), 'utf8');
  const baseline = ci.split('\n  baseline:')[1].split('\n  candidate-release:')[0];
  assert.ok(!baseline.includes('id-token: write'));
  assert.match(ci, /candidate-release:[\s\S]*github.event_name == 'workflow_dispatch'/);
  assert.match(ci, /github.sha == vars.SHARED_EKS_CANDIDATE_SHA/);
  assert.ok(!ci.includes('configure-aws-credentials'));
  assert.ok(ci.includes('node --test scripts/release/release.test.mjs'));
});

test('actual workflow conditions apply emergency stops outside both release modes', () => {
  const yaml = readFileSync(new URL('../../.github/workflows/shared-eks-release.yml', import.meta.url), 'utf8');
  const condition = job => {
    const section = yaml.split(`\n  ${job}:`)[1].split(/\n  [a-z]+:/)[0];
    const multiline = section.match(/\n    if: >-\n([\s\S]*?)(?=\n    [a-z])/);
    return multiline ? multiline[1].trim() : section.match(/\n    if: (.*)/)[1];
  };
  for (const candidate of [false,true]) {
    const ref = candidate ? 'refs/heads/codex/exact-candidate' : 'refs/heads/main';
    const base = {needs:{validate:{outputs:{eligible:'true'}},publish:{outputs:{changed:'true'}}},
      github:{ref,sha:B,event_name:candidate?'workflow_dispatch':'push'}, inputs:{candidate,publish:true,deploy:true},
      vars:{SHARED_EKS_RELEASE_DISABLED:'',SHARED_EKS_DEPLOY_DISABLED:'',SHARED_EKS_CANDIDATE_ENABLED:'true',SHARED_EKS_CANDIDATE_REF:ref,SHARED_EKS_CANDIDATE_SHA:B}};
    const evaluate = (job, changes={}) => runInNewContext(condition(job), {...base,vars:{...base.vars,...changes}}, {timeout:100});
    for (const job of ['snapshot','publish','deploy']) {
      assert.equal(evaluate(job),true,`${job} default mode=${candidate}`);
      for(const value of ['true','typo']) assert.equal(evaluate(job,{SHARED_EKS_RELEASE_DISABLED:value}),false,`${job} release stop mode=${candidate}`);
    }
    assert.equal(evaluate('snapshot',{SHARED_EKS_DEPLOY_DISABLED:'true'}),true);
    assert.equal(evaluate('publish',{SHARED_EKS_DEPLOY_DISABLED:'true'}),true);
    for(const value of ['true','typo']) assert.equal(evaluate('deploy',{SHARED_EKS_DEPLOY_DISABLED:value}),false,`deploy stop mode=${candidate}`);
  }
});
