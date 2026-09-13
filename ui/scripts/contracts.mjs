// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { execFileSync, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { readFileSync, writeFileSync, mkdirSync, unlinkSync } from 'node:fs';

export const root = fileURLToPath(new URL('../../', import.meta.url));
const env = { ...process.env, GOCACHE: process.env.GOCACHE || join(root, '.tmp/gocache') };
export function go(...args) {
  return execFileSync('go', args, { cwd: root, env, encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
}
export function fixtureRows() {
  return JSON.parse(go('run', './ui/contractgen', 'fixtures', '.'));
}
if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const mode = process.argv[2];
  if (!['generate', 'check'].includes(mode)) throw new Error('expected generate or check');
  go('run', './ui/contractgen', mode, '.');
  if (mode === 'check') {
    console.log(go('test', '-count=1', '-v', './ui/contractgen'));
    // Prove the real check rejects stale bindings without changing tracked output.
    const target = join(root, 'ui/src/contracts.generated.ts');
    const original = readFileSync(target);
    const candidate = join(root, `.tmp/drift-candidate-${process.pid}.ts`);
    mkdirSync(join(root, '.tmp'), { recursive: true });
    try {
      writeFileSync(candidate, original.toString().replace('requestRef: string', 'requestRef: number'));
      const result = spawnSync('go', ['run', './ui/contractgen', 'check', '.', candidate], { cwd: root, env, encoding: 'utf8' });
      if (result.status === 0 || !result.stderr?.includes('contract drift')) throw new Error('stale binding was not rejected by drift check');
      console.log('[pass] deliberate requestRef binding drift rejected');
    } finally { unlinkSync(candidate); }
    go('run', './ui/contractgen', 'check', '.');
    console.log('[pass] unchanged canonical bindings still pass');
  }
}
