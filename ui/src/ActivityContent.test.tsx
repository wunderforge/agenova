// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {afterEach,expect,it,vi} from 'vitest';
import {act,cleanup,render,screen} from '@testing-library/react';
import {ActivityContent} from './ActivityContent';
afterEach(()=>{cleanup();vi.unstubAllGlobals();});
it('shows actual response, provider thinking only when present, and encoded Work-bound content',async()=>{
 const fetcher=vi.fn().mockResolvedValue({ok:true,json:async()=>({kind:'model-response',source:'model-provider',output:'{"action":"tool","tool":"web.search"}',decisionSummary:'Need a current source.',elapsedMs:1230,model:'actual-model'})});
 vi.stubGlobal('fetch',fetcher);
 render(<ActivityContent work="work-a" reference="content:abc"/>);
 expect(await screen.findByText('Need a current source.')).toBeTruthy();
 expect(screen.getByText(/"web.search"/)).toBeTruthy();
 expect(screen.queryByText(/Thinking/)).toBeNull();
 expect(fetcher.mock.calls[0][0]).toBe('/api/requests/work-a/content/content%3Aabc');
});
it('does not reconstruct missing historical content or render provider HTML',async()=>{
 vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:true,json:async()=>({kind:'tool-response',source:'configured-tool',output:'<script>fake()</script>',thinking:'Returned provider text'})}));
 const {container}=render(<ActivityContent work="work-a" reference="content:abc"/>);
 expect(await screen.findByText('<script>fake()</script>')).toBeTruthy();expect(container.querySelector('script')).toBeNull();
 expect(screen.getByText('Thinking · Provider returned')).toBeTruthy();
});
it('ignores a late body from a previously selected record',async()=>{
 let resolveFirst!:(value:unknown)=>void;
 const first=new Promise(resolve=>{resolveFirst=resolve;});
 vi.stubGlobal('fetch',vi.fn().mockResolvedValueOnce({ok:true,json:()=>first}).mockResolvedValueOnce({ok:true,json:async()=>({kind:'model-response',source:'model-provider',output:'Second record'})}));
 const view=render(<ActivityContent work="a" reference="content:one"/>);
 await act(async()=>{});
 view.rerender(<ActivityContent work="a" reference="content:two"/>);
 expect(await screen.findByText('Second record')).toBeTruthy();
 await act(async()=>{resolveFirst({kind:'model-response',source:'model-provider',output:'Stale first record'});});
 expect(screen.getByText('Second record')).toBeTruthy();
 expect(screen.queryByText('Stale first record')).toBeNull();
});
