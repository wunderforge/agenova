// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { preview } from 'vite';
import { fileURLToPath } from 'node:url';

// Own the preview server directly: avoid Windows shell/npm process-tree teardown.
export default async function setup() {
  const server = await preview({
    configFile: false,
    root: fileURLToPath(new URL('../', import.meta.url)),
    preview: { host: '127.0.0.1', port: 4173, strictPort: true },
  });
  return async () => {
    if ('closeAllConnections' in server.httpServer) server.httpServer.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.httpServer.close(error => error ? reject(error) : resolve()));
  };
}
