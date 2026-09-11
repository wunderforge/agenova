// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { shapes } from './contracts.generated';
import type { Diagnostic } from './evidence-source';

interface Shape {
  kind: string;
  ref?: string;
  item?: Shape;
  values?: readonly string[];
  fields?: Record<string, { shape: Shape; optional?: boolean }>;
}
const definitions: Record<string, Shape> = shapes;
const object = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);

// Wire-shape/display integrity only. Semantic validation stays in canonical Go
// parsers at the fixture build boundary. This function never authorizes a run.
export function shapeDiagnostics(type: 'ClaimRequest' | 'IssuedState', data: unknown): Diagnostic[] {
  const issues: Diagnostic[] = [];
  const issue = (category: string, fieldPath: string) => issues.push({ category, fieldPath });
  const jsonValue = (value: unknown): boolean => value === null || typeof value === 'string' || typeof value === 'boolean' ||
    (typeof value === 'number' && Number.isFinite(value)) ||
    (Array.isArray(value) && value.every(jsonValue)) || (object(value) && Object.values(value).every(jsonValue));
  function visit(s: Shape, value: unknown, path: string) {
    switch (s.kind) {
      case 'ref': visit(definitions[s.ref!], value, path); break;
      case 'nullable': if (value !== null) visit(s.item!, value, path); break;
      case 'string': if (typeof value !== 'string') issue('invalid-value', path); break;
      case 'enum': if (typeof value !== 'string' || !s.values!.includes(value)) issue('invalid-value', path); break;
      case 'json-map': if (value !== null && (!object(value) || !jsonValue(value))) issue('invalid-value', path); break;
      case 'array':
        if (!Array.isArray(value)) issue('invalid-value', path);
        else value.forEach((item, i) => visit(s.item!, item, `${path}[${i}]`));
        break;
      case 'object':
        if (!object(value)) { issue('invalid-value', path); break; }
        for (const key of Object.keys(value)) {
          if (!Object.hasOwn(s.fields!, key)) issue('unknown-field', path === '$' ? key : `${path}.${key}`);
        }
        for (const [key, field] of Object.entries(s.fields!)) {
          const child = path === '$' ? key : `${path}.${key}`;
          if (!Object.hasOwn(value, key) || value[key] === undefined || value[key] === null) {
            if (!field.optional) issue('missing-source-field', child);
          } else visit(field.shape, value[key], child);
        }
        break;
      default: throw new Error('Unsupported generated shape');
    }
  }
  visit(definitions[type], data, '$');
  return issues;
}
