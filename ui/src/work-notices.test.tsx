// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import {afterEach,expect,it} from 'vitest';
import {render,screen,cleanup} from '@testing-library/react';
import {WorkNotices} from './WorkAuthority';
import {workStatus,isTerminal,type View} from './connected-source';
import {PlatformOverview} from './ProductPages';
afterEach(cleanup);
const work=()=>({requestRef:'w',request:{spec:{requestedAccess:{tools:[]}}},state:{decision:{result:'Allow'},effectiveAuthority:{tools:[]},claim:{phase:'Succeeded'}},facts:[],outcome:{status:'Succeeded',text:'No tool available.',completion:{status:'incomplete',reason:'missing-capability',target:'web.read'}}} as unknown as View);
it('incomplete is terminal but is not green business success',()=>{expect(workStatus(work())).toBe('Incomplete');expect(isTerminal(work())).toBe(true);});
it('missing capability notice is not a fabricated denial',()=>{render(<WorkNotices work={work()}/>);expect(screen.getByRole('alert').textContent).toBe('web.read unavailableDetails');expect(screen.queryByText('web.read blocked')).toBeNull();expect(screen.getByRole('link').getAttribute('href')).toBe('#/work/w/completion');});
it('does not call an actually granted tool unavailable',()=>{const w=work();w.state!.effectiveAuthority!.tools=['web.read'];render(<WorkNotices work={w}/>);expect(screen.queryByText('web.read unavailable')).toBeNull();});
it('native timeout stays an observation, not verified infra blocking',()=>{const w=work();w.facts=[{id:'f1',kind:'WorkerActivity',sequence:1,timestamp:'2026-10-23T09:00:00Z',requestRef:'w',operation:'NativeHTTPSObservation',reasonCode:'worker-native-tcp-timeout'}];render(<WorkNotices work={w}/>);expect(screen.getByText('Direct HTTPS timed out')).toBeTruthy();expect(screen.queryByText('Direct HTTPS blocked')).toBeNull();expect(screen.getByRole('link',{name:'Evidence'}).getAttribute('href')).toBe('#/work/w/activity/f1');});
it('real Deny fact retains the concise evidence banner',()=>{const w=work();w.outcome!.completion=undefined;w.outcome!.status='Failed';w.facts=[{id:'deny1',kind:'ToolDecision',sequence:1,timestamp:'2026-10-23T09:00:00Z',requestRef:'w',target:'web.read',result:'Deny'}];render(<WorkNotices work={w}/>);expect(screen.getByText('web.read blocked')).toBeTruthy();});
it('MCP connection type comes from adapter, not instance name',()=>{const setup={installation:{platform:'P',revision:'sha256:abc'},policy:{ID:'policy',Version:'1'},capabilities:{tool:'configured'},platformInfo:{operator:true,connections:[{category:'tool',name:'incident-evidence',backend:'incident-evidence',adapter:'agenova.io/tool/mcp-http@0.1.0',operation:'web.read'}]}};render(<PlatformOverview setup={setup as never}/>);expect(screen.getByText('MCP')).toBeTruthy();expect(screen.getByText('Instances')).toBeTruthy();});
