// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

import { lstatSync, readFileSync } from 'node:fs';

// Read by the local Vite server only. Browser modules never import this file,
// receive the token, or persist it in URL/storage.
export function serverBearerToken(path: string | undefined): string | undefined {
  if (!path) return undefined;
  let stat;
  try {
    stat = lstatSync(path);
  } catch {
    throw new Error('OIDC token file is unavailable.');
  }
  if (!stat.isFile() || stat.isSymbolicLink()) throw new Error('OIDC token file must be a regular file.');
  if (process.platform !== 'win32' && (stat.mode & 0o077) !== 0) {
    throw new Error('OIDC token file permissions must not grant group or other access.');
  }
  let token: string;
  try {
    token = readFileSync(path, { encoding: 'utf8', flag: 'r' }).trim();
  } catch {
    throw new Error('OIDC token file could not be read.');
  }
  if (!token || token.length >= 64 * 1024 || /\s/.test(token)) {
    throw new Error('OIDC token file must contain one bounded bearer token.');
  }
  return token;
}
