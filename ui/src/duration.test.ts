// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {expect,it} from 'vitest';
import {requestedMinutes} from './duration';
it.each([['30s',0.5],['1m30s',1.5],['1h',20],['500ms',1/120],['2m',2],['bad',20],['-1m',20]])('uses the template duration %s without expanding it', (value,expected)=>{
 expect(requestedMinutes(value)).toBe(expected);
});
