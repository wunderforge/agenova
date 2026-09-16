// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

export function TaskInstructions({ text }: { text?: unknown }) {
  if (typeof text !== 'string' || !text.trim()) return null;
  return <details className="portal-details portal-task-instructions">
    <summary>Task instructions</summary><pre>{text}</pre>
  </details>;
}
export function CopyRequestID({ value }: { value: string }) {
  const [result, setResult] = useState('');
  return <div className="portal-copy-reference"><button type="button" onClick={async () => {
    try { await navigator.clipboard.writeText(value); setResult('Copied'); }
    catch { setResult('Could not copy'); }
  }}>Copy request ID</button><span role="status">{result}</span></div>;
}
