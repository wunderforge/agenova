// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
// Release transport only: no cluster creation, Secret access or template writes.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export const SOURCE = 'delivery.agenova.io/source-sha';
export const COMPONENTS = {
  controlPlane: { dockerfile: 'deploy/production/release.Dockerfile', packages: ['./cmd/agenova-control-plane'] },
  worker: { dockerfile: 'harness/integration/agentsandbox/testworker/Dockerfile', packages: ['./harness/integration/agentsandbox/testworker', './cmd/agenova-demo-worker'] },
};
const shaPattern = /^[a-f0-9]{40}$/;
const digestPattern = /^sha256:[a-f0-9]{64}$/;
function requireValue(condition, message) { if (!condition) throw new Error(message); }
export function sha(value) { requireValue(shaPattern.test(value ?? ''), 'A full source commit SHA is required'); return value; }
export function canonical(value) {
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.keys(value).sort().map(k => `${JSON.stringify(k)}:${canonical(value[k])}`).join(',')}}`;
  return JSON.stringify(value);
}
function hash(value) { return createHash('sha256').update(canonical(value)).digest('hex'); }

// Do not print subprocess output/errors that could contain Pod configuration or tokens.
export function command(file, args, options = {}) {
  const output = execFileSync(file, args, { encoding: 'utf8', maxBuffer: 32 * 1024 * 1024, stdio: ['pipe', 'pipe', 'pipe'], ...options });
  return typeof output === 'string' ? output.trim() : '';
}
export function config(env = process.env) {
  const c = {
    region: env.AWS_REGION, account: env.AWS_ACCOUNT_ID,
    cluster: env.EKS_CLUSTER_NAME, namespace: env.K8S_NAMESPACE,
    deployment: env.CONTROL_PLANE_DEPLOYMENT, container: env.CONTROL_PLANE_CONTAINER,
    controlPlane: env.ECR_CONTROL_PLANE_REPOSITORY, worker: env.ECR_WORKER_REPOSITORY,
  };
  requireValue(/^[0-9]{12}$/.test(c.account ?? ''), 'Invalid AWS account');
  requireValue(/^[a-z]+-[a-z]+-[0-9]+$/.test(c.region ?? ''), 'Invalid AWS region');
  for (const key of ['cluster', 'namespace', 'deployment', 'container']) {
    requireValue(/^[a-z0-9][a-z0-9-]{0,99}$/.test(c[key] ?? ''), `Invalid ${key}`);
  }
  const prefix = `${c.account}.dkr.ecr.${c.region}.amazonaws.com/`;
  for (const key of ['controlPlane', 'worker']) {
    requireValue(c[key]?.startsWith(prefix) && /^[a-z0-9][a-z0-9/_-]+$/.test(c[key].slice(prefix.length)), `Invalid ${key} ECR repository`);
  }
  requireValue(c.controlPlane !== c.worker, 'Separate image repositories are required');
  return c;
}
export function releaseStopGate(env = process.env, deploy = false) {
  const notDisabled = value => value === undefined || value === '' || value === 'false';
  requireValue(notDisabled(env.SHARED_EKS_RELEASE_DISABLED), 'Publication is disabled or switch is invalid');
  if (deploy) requireValue(notDisabled(env.SHARED_EKS_DEPLOY_DISABLED), 'Deployment is disabled or switch is invalid');
}
export function writeGate(env = process.env, deploy = false) {
  requireValue(env.GITHUB_REF === 'refs/heads/main', 'Releases require main');
  requireValue(['push', 'workflow_dispatch'].includes(env.GITHUB_EVENT_NAME), 'Unsupported release event');
  releaseStopGate(env, deploy);
}
export function image(value, repo) {
  requireValue(value?.startsWith(`${repo}@`) && digestPattern.test(value.slice(repo.length + 1)), 'Expected repository-bound image digest');
  return value;
}

// Capture a secret-free projection; full Deployment JSON stays in process memory.
export function snapshot(object, c, baseline, allowUnknown = false) {
  const { metadata: m, spec: s } = object;
  requireValue(m?.name === c.deployment && m.namespace === c.namespace && !m.deletionTimestamp, 'Wrong or terminating Deployment');
  requireValue(m.uid && m.resourceVersion && s?.replicas === 1 && s.strategy?.type === 'Recreate', 'Expected single-replica Recreate Deployment');
  const t = structuredClone(s.template);
  const entries = t?.spec?.containers ?? [];
  const indexes = entries.flatMap((v, i) => v.name === c.container ? [i] : []);
  requireValue(indexes.length === 1, 'Ambiguous control-plane container');
  const index = indexes[0];
  const current = image(entries[index].image, c.controlPlane);
  const vars = entries[index].env ?? [];
  requireValue(vars.some(v => v.name === 'AGENOVA_AUTH_MODE' && v.value === 'member'), 'Refusing a non-member control plane');
  requireValue(vars.some(v => v.name === 'AGENOVA_MEMBER_CONFIG' && v.value), 'Missing member configuration');
  requireValue(entries[index].readinessProbe, 'A control-plane readiness probe is required');
  delete entries[index].image;
  const source = m.annotations?.[SOURCE] || baseline;
  if (!(allowUnknown && !source)) sha(source);
  const result = { uid: m.uid, resourceVersion: m.resourceVersion, index, image: current,
    binding: hash({ template: t, strategy: s.strategy, replicas: s.replicas }),
    source: source ?? null, recordedSource: m.annotations?.[SOURCE] ?? null };
  // Legacy image hints remain inside the configuration hash, but are not an
  // allowlist. Ordinary delivery must not depend on any user template/image.
  return result;
}
export function ready(object) {
  const s = object.status ?? {};
  return s.observedGeneration === object.metadata.generation && s.replicas === 1 && s.updatedReplicas === 1 && s.availableReplicas === 1 && (s.unavailableReplicas ?? 0) === 0;
}
export function workerSnapshot(object, c) {
  requireValue(object.metadata?.name === 'agenova-template-slot' && object.metadata.namespace === c.namespace &&
    object.metadata.labels?.['app.kubernetes.io/managed-by'] === 'agenova', 'Expected managed Worker template slot');
  const template = JSON.parse(object.data?.['template.json'] ?? '{}');
  return { image: image(template.spec?.artifact?.image, c.worker), binding: hash(template) };
}
// A retained immutable reference slot need not be the new project template's
// active runtime image. Report the actual installed compatibility binding.
export function installedWorkerImage(s, c) {
  return image(s.compatibleWorkerImage ?? s.worker.image, c.worker);
}
export function kube(c, run = command) {
  return (...args) => run('kubectl', ['--namespace', c.namespace, ...args]);
}
export function connect(c, run = command) {
  const identity = JSON.parse(run('aws', ['sts', 'get-caller-identity', '--output', 'json']));
  requireValue(identity.Account === c.account, 'Wrong AWS account');
  // KUBECONFIG must point to the disposable runner file, never a developer default.
  requireValue(process.env.KUBECONFIG, 'Explicit KUBECONFIG is required');
  run('aws', ['eks', 'update-kubeconfig', '--name', c.cluster, '--region', c.region, '--kubeconfig', process.env.KUBECONFIG]);
}

export function changedComponents(paths, dirs) {
  const result = { controlPlane: false, worker: false };
  for (const raw of paths) {
    const p = raw.replaceAll('\\', '/');
    if (p.endsWith('_test.go') || p.endsWith('.md') || p.startsWith('ui/tests/') || p.startsWith('ui/e2e/')) continue;
    for (const [name, component] of Object.entries(COMPONENTS)) {
      if (['go.mod', 'go.sum', '.dockerignore'].includes(p) || p === component.dockerfile ||
          (name === 'controlPlane' && (p.startsWith('ui/') || p === 'deploy/reference/Dockerfile')) ||
          dirs[name].some(dir => p.startsWith(`${dir}/`))) result[name] = true;
    }
  }
  return result;
}
export function plan(base, target, run = command) {
  sha(base); sha(target);
  requireValue(run('git', ['rev-parse', 'HEAD']) === target, 'Checkout differs from target');
  run('git', ['merge-base', '--is-ancestor', base, target]);
  const dirs = {};
  for (const [name, component] of Object.entries(COMPONENTS)) {
    // Derive from actual Go dependencies, not a permanently hardcoded internal/ map.
    dirs[name] = run('go', ['list', '-deps', '-f', '{{if and .Module (not .Standard)}}{{if .Module.Main}}{{.ImportPath}}{{end}}{{end}}', ...component.packages])
      .split(/\r?\n/).filter(v => v.startsWith('github.com/wunderforge/agenova/'))
      .map(v => v.slice('github.com/wunderforge/agenova/'.length));
  }
  const paths = run('git', ['diff', '--no-renames', '--name-only', '-z', base, target]).split('\0').filter(Boolean);
  return { base, target, changed: changedComponents(paths, dirs) };
}
export function eventEligible(env = process.env, run = command) {
  if (env.GITHUB_EVENT_NAME === 'workflow_dispatch') return true;
  requireValue(env.GITHUB_EVENT_NAME === 'push' && env.GITHUB_REF === 'refs/heads/main', 'Unsupported release preflight event');
  return Object.values(plan(sha(env.EVENT_BEFORE_SHA), sha(env.GITHUB_SHA), run).changed).some(Boolean);
}
export function existingDigest(repoName, tag, run = command) {
  try {
    const d = run('aws', ['ecr', 'describe-images', '--repository-name', repoName, '--image-ids', `imageTag=${tag}`, '--query', 'imageDetails[0].imageDigest', '--output', 'text']);
    requireValue(digestPattern.test(d), 'Invalid ECR digest');
    return d;
  } catch (error) {
    if (String(error.stderr ?? '').includes('(ImageNotFoundException)')) return null;
    throw new Error('ECR image lookup failed; only ImageNotFound permits building');
  }
}
export function publish(releasePlan, c, run = command) {
  sha(releasePlan.target);
  const tag = `sha-${releasePlan.target}`;
  const result = { ...releasePlan, images: {} };
  let loggedIn = false;
  for (const [name, component] of Object.entries(COMPONENTS)) {
    if (!releasePlan.changed[name]) continue;
    const repo = c[name];
    const registry = repo.split('/')[0];
    const repoName = repo.slice(registry.length + 1);
    console.log(`Checking immutable ${name} image for ${releasePlan.target}`);
    let digest = existingDigest(repoName, tag, run);
    if (!digest) {
      if (!loggedIn) {
        console.log('Authenticating ECR publisher (credential kept on stdin)');
        const password = run('aws', ['ecr', 'get-login-password', '--region', c.region]);
        run('docker', ['login', '--username', 'AWS', '--password-stdin', registry], { input: password });
        loggedIn = true;
      }
      console.log(`Building/publishing public-source ${name} image`);
      run('docker', ['buildx', 'build', '--platform', 'linux/amd64', '--file', component.dockerfile,
        '--label', `org.opencontainers.image.revision=${releasePlan.target}`,
        '--label', 'org.opencontainers.image.source=https://github.com/wunderforge/agenova',
        '--tag', `${repo}:${tag}`, '--push', '.'], { stdio: ['ignore', 'inherit', 'inherit'] });
      digest = existingDigest(repoName, tag, run);
      requireValue(digest, 'Published digest not available');
    }
    result.images[name] = image(`${repo}@${digest}`, repo);
  }
  result.workerRolloutRequired = false;
  return result;
}

function patchImage(s, next) {
  const path = `/spec/template/spec/containers/${s.index}/image`;
  return [
    { op: 'test', path: '/metadata/uid', value: s.uid },
    { op: 'test', path: '/metadata/resourceVersion', value: s.resourceVersion },
    { op: 'test', path, value: s.image },
    { op: 'replace', path, value: next },
  ];
}
export function deploy(release, expected, c, run = command, record = () => {}) {
  sha(release.target);
  requireValue(release.base === expected.source, 'Release baseline mismatch');
  if (!release.changed.controlPlane) {
    const evidence = { source: release.target, previousSource: expected.source, result: 'published-no-platform-change' };
    record(evidence);
    return evidence;
  }
  const next = release.changed.controlPlane ? image(release.images.controlPlane, c.controlPlane) : expected.image;
  const k = kube(c, run);
  const get = () => JSON.parse(k('get', 'deployment', c.deployment, '-o', 'json'));
  const patch = ops => k('patch', 'deployment', c.deployment, '--type=json', '--patch', JSON.stringify(ops), '-o', 'name');
  const original = get();
  const current = snapshot(original, c, expected.source);
  requireValue(canonical(current) === canonical(expected), 'Deployment changed since snapshot; re-run from a fresh snapshot');
  requireValue(ready(original), 'Current Deployment is not healthy');
  // Single-instance Recreate cannot safely replace a process owning Work.
  // This is a quiet-window check, not an admission lock or zero-downtime drain.
  const claims = JSON.parse(k('get', 'sandboxclaims.extensions.agents.x-k8s.io', '-o', 'json'));
  requireValue(Array.isArray(claims.items) && claims.items.length === 0, 'Active SandboxClaims prevent rollout; retry in a quiet window');
  const evidence = { source: release.target, previousSource: expected.source,
    previousImage: expected.image, desiredImage: next, result: 'planned' };
  record(evidence);
  let attempted = false;
  try {
    if (next !== current.image) {
      // Set before invoking: transport failure may happen after the API accepted the patch.
      attempted = true;
      patch(patchImage(current, next));
      evidence.result = 'rollout-started'; record(evidence);
    }
    k('rollout', 'status', `deployment/${c.deployment}`, '--timeout=180s');
    const object = get();
    const after = snapshot(object, c, expected.source);
    requireValue(after.uid === expected.uid && after.binding === expected.binding && after.image === next && after.recordedSource === expected.recordedSource && ready(object), 'Post-rollout health or binding mismatch');
    const annotations = { ...(object.metadata.annotations ?? {}), [SOURCE]: release.target };
    patch([
      { op: 'test', path: '/metadata/uid', value: after.uid },
      { op: 'test', path: '/metadata/resourceVersion', value: after.resourceVersion },
      { op: 'add', path: '/metadata/annotations', value: annotations },
    ]);
    evidence.result = 'ready'; record(evidence);
    return evidence;
  } catch {
    evidence.result = 'failed';
    if (attempted) {
      try {
        const obj = get();
        const latest = snapshot(obj, c, expected.source);
        requireValue(latest.uid === expected.uid && latest.binding === expected.binding && latest.recordedSource === expected.recordedSource && latest.image === next, 'Concurrent change prevents recovery');
        patch(patchImage(latest, expected.image));
        k('rollout', 'status', `deployment/${c.deployment}`, '--timeout=180s');
        const recovered = get();
        const s = snapshot(recovered, c, expected.source);
        requireValue(s.uid === expected.uid && s.recordedSource === expected.recordedSource && s.image === expected.image && s.binding === expected.binding && ready(recovered), 'Recovery not healthy');
        evidence.result = 'failed-image-restored';
      } catch { evidence.result = 'failed-recovery-needs-operator'; }
    }
    record(evidence);
    throw new Error(`Deployment failed: ${evidence.result}; see sanitized evidence`);
  }
}

function save(file, data) { writeFileSync(file, `${JSON.stringify(data, null, 2)}\n`); }
function load(file) { return JSON.parse(readFileSync(file, 'utf8')); }
export function main(args = process.argv.slice(2)) {
  const [action] = args;
  if (action === 'preflight') {
    // Offline event classification happens before any cloud role is assumed.
    // Manual dispatch deliberately allows inspecting cumulative pending changes.
    const eligible = eventEligible();
    requireValue(process.env.GITHUB_OUTPUT, 'Workflow output file is required');
    writeFileSync(process.env.GITHUB_OUTPUT, `eligible=${eligible}\n`, { flag: 'a' });
    return;
  }
  const c = config();
  writeGate(process.env, action === 'deploy');
  const target = sha(process.env.GITHUB_SHA);
  if (action === 'snapshot') {
    connect(c);
    const object = JSON.parse(kube(c)('get', 'deployment', c.deployment, '-o', 'json'));
    requireValue(ready(object), 'Existing Deployment is not ready');
    const s = snapshot(object, c, process.env.SHARED_EKS_BASE_SHA);
    save('snapshot.json', s);
  } else if (action === 'publish') {
    const s = load('snapshot.json');
    const releasePlan = plan(s.source, target);
    save('plan.json', releasePlan);
    const release = publish(releasePlan, c);
    release.previousControlPlaneImage = s.image;
    save('release.json', release);
  } else if (action === 'deploy') {
    const release = load('release.json');
    requireValue(release.target === target, 'Artifact target mismatch');
    command('git', ['fetch', '--no-tags', 'origin', 'main']);
    requireValue(command('git', ['rev-parse', 'origin/main']) === target, 'Main advanced; deploy the latest candidate instead');
    connect(c);
    deploy(release, load('snapshot.json'), c, command, data => save('deployment.json', data));
  } else throw new Error('Expected snapshot, publish or deploy');
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try { main(); } catch (error) {
    // execFile errors embed stdout, argv and sometimes object data. Never echo them.
    console.error(error.status !== undefined || error.code || error instanceof SyntaxError ? 'Release input or subprocess failed; no sensitive output emitted' : error.message);
    process.exitCode = 1;
  }
}
