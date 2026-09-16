// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

export const maxWorkName = 80;
// Presentation only: never replace instructions or request/claim identity.
export function displayWorkName(name: unknown, instructions: unknown, fallback: string): string {
  const explicit = typeof name === 'string' ? name.trim().replace(/\s+/g, ' ') : '';
  const text = typeof instructions === 'string' ? instructions.trim() : '';
  const firstLine = text.split(/\r?\n/)[0];
  const first = firstLine.match(/^.*?(?:[.!?](?=\s|$)|[。！？])/)?.[0] || firstLine;
  const title = explicit || first?.replace(/\s+/g, ' ') || fallback;
  const chars = Array.from(title);
  return chars.length > maxWorkName ? chars.slice(0, maxWorkName - 1).join('').trimEnd() + '…' : title;
}
