// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// This value is read by the Vite server, not exposed through a VITE_ browser
// variable. The local reference API has no production user authentication, so
// the development proxy must never forward to a remote or public address.
export function localAPITarget(configured: string | undefined): string {
  const input = configured === undefined ? 'http://127.0.0.1:8088' : configured;
  let url: URL;
  try {
    url = new URL(input);
  } catch {
    throw new Error('AGENOVA_API_URL must be an HTTP loopback origin with an explicit port.');
  }
  if (url.protocol !== 'http:' ||
      (url.hostname !== '127.0.0.1' && url.hostname !== '[::1]') ||
      !url.port || url.username || url.password ||
      url.pathname !== '/' || url.search || url.hash) {
    throw new Error('AGENOVA_API_URL must be an HTTP loopback origin with an explicit port and no path, credentials, query, or fragment.');
  }
  return url.origin;
}
