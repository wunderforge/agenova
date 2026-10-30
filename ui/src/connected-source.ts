// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type { AgentTemplate, ClaimRequest, Fact, Principal, View as CanonicalView } from './contracts.generated';
import { shapeDiagnostics } from './shape-check';
import { displayWorkName } from './work-name';

// Live views must contain the canonical request and a fact array.
export type View = Omit<CanonicalView, 'request' | 'facts'> & {
  request: ClaimRequest;
  facts: Fact[];
};
export interface Setup {
	accessAdvisor?: AdvisorInfo;
  platformInfo?: { operator:boolean; connections: ConnectionInfo[] };
  installation: { kind: 'installed'; platform: string; revision: string };
  principal: Principal;
  template: AgentTemplate | null;
  project?: string;
  projects?: string[];
  templates?: string[];
  submissionProjects?: string[];
  organization?: { id: string; displayName: string; description?: string };
  observability?: { enabled: boolean; endpoint?: string; exportFailures: number; ingestion: string; diagnosticURL?: string };
  policy: {
    ID: string;
    Version: string;
    Rules: PolicyRule[];
  };
  capabilities: Record<string, string>;
}
export type AccessMode = 'manual' | 'suggest' | 'automatic';
export interface AdvisorInfo {enabled:boolean;allowedModes:AccessMode[];defaultMode:AccessMode;engine?:string;callsUsed:number;maxCalls:number}
export interface AccessPlan {id:string;status:'ready'|'clarification'|'failed';request?:ClaimRequest;profileID?:string;explanation:string;questions:string[];engine:string;guideDigest:string;guideVersion:string;intentDigest:string;model?:{name:string;inputTokens:number;outputTokens:number}}
export interface ConnectionInfo {category:string;name:string;backend?:string;adapter?:string;model?:string;operation?:string;scope?:string;tool?:string;endpoint?:string;status?:string}
export interface AgentEntry {project:string;template:AgentTemplate;canSubmit:boolean}
export interface PolicyRule {
  subject?: string; team: string; action: string; project: string; templateRef: string;
  constraints?: { tools?: { values: string[] | null }; resourceScopes?: { values: string[] | null }; networkProfiles?: { values: string[] | null }; memoryScopes?: { values: string[] | null }; modelProfiles?: { values: string[] | null }; runtimeProfiles?: { values: string[] | null }; maxTimeout?: string | number };
}
// Display-shape validation only; the Go policy engine remains the authority.
export function validPolicyRule(value: unknown): value is PolicyRule {
 if(!value || typeof value!=='object')return false;
 const rule=value as PolicyRule;
 if(![rule.team,rule.action,rule.project,rule.templateRef].every(item=>typeof item==='string') || rule.subject!==undefined&&typeof rule.subject!=='string')return false;
 if(rule.constraints===undefined)return true;
 if(!rule.constraints || typeof rule.constraints!=='object' || Array.isArray(rule.constraints))return false;
 return Object.entries(rule.constraints).every(([key,cap])=>key==='maxTimeout'
  ? typeof cap==='string'||typeof cap==='number'&&Number.isFinite(cap)
  : !!cap && typeof cap==='object' && 'values' in cap && (cap.values===null || Array.isArray(cap.values) && cap.values.every((item:unknown)=>typeof item==='string')));
}
// Suggestions only: the installed service remains the authority for admission.
export function suggestedProjects(setup: Setup): string[] {
  if (setup.submissionProjects !== undefined) return setup.submissionProjects;
  return [...new Set(setup.policy.Rules
    .filter(rule => rule.team === setup.principal.team && rule.action === 'claim.create' &&
      rule.templateRef === setup.template?.metadata.name)
    .map(rule => rule.project))];
}
export class SourceError extends Error {
  constructor(public readonly status: number, message: string, public readonly uncertain = false) {
    super(message);
  }
}

async function json(path: string, signal?: AbortSignal, body?: unknown, adviceID?:string): Promise<unknown> {
  // The installed service can synchronously configure the runtime before
  // acknowledging submission. Reads remain short; writes share the server's
  // bounded setup window so a Work is not launched after the UI reports fail.
  const timeout = AbortSignal.timeout(body ? 240_000 : 8_000);
  let response: Response;
  try { response = await fetch(path, {
    signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
    cache: 'no-store',
    ...(body ? {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(adviceID ? {'X-Agenova-Advice-ID':adviceID}: {}) },
      body: JSON.stringify(body),
    } : {}),
  }); } catch { throw new SourceError(0, body ? 'Submission result is not confirmed. Check this request before creating another.' : 'The connection is unavailable. Try again.', !!body); }
  if (!response.ok) {
    if(path.startsWith('/api/access-')) {
      let message='Access planning did not complete. No confirmed Work was returned.';let uncertain=response.status>=500;
      const knownCodes=['advisor_failed','advisor_unavailable','advisor_invalid','advisor_disabled','advisor_budget','advisor_busy','mode_disabled','invalid_task','invalid_advice_context','required_access_missing','authority_request_rejected','advice_stale','advice_expired','advice_conflict','advice_pending','advice_not_found','explicit_access','capability_unavailable','access_denied','viewer_required','request_conflict','storage_unavailable'];
      try{const detail=await response.json();if(knownCodes.includes(detail.code)&&typeof detail.message==='string'&&detail.message.length<=256){message=detail.message;uncertain=!['advisor_failed','advisor_invalid','advisor_budget','advisor_busy','mode_disabled','invalid_task','required_access_missing','advice_stale'].includes(detail.code)&&response.status>=500;}}catch{/* Preserve bounded generic diagnosis. */}
      throw new SourceError(response.status,message,uncertain);
    }
    // Never echo raw upstream errors, which can contain infrastructure details.
    const messages: Record<number, string> = {
      400: 'Request validation failed. Review the task and requested access.',
      401: 'Your session has expired. Sign in again to continue.',
      403: 'Access denied. Your identity does not permit this project operation.',
      404: 'This work was not found.', 409: 'This request reference already exists with different content.',
      422: 'The request could not be admitted. Review Policy, template limits and installed capabilities.',
      429: 'Capacity is currently limited. Check the existing request before retrying.',
    };
    const uncertain = !!body && (response.status >= 500 || response.status === 409);
    throw new SourceError(response.status, messages[response.status] || (uncertain ? 'Submission result is not confirmed. Check the existing request before retrying.' : 'The connection is unavailable. Try again.'), uncertain);
  }
  try { return await response.json(); }
  catch { throw new SourceError(502, 'The connection returned an incomplete response.', !!body); }
}

function view(data: unknown): View {
  if (shapeDiagnostics('View', data).length || (data as View).version !== 'agenova.evidence/v0') {
    throw new SourceError(502, 'The connection returned an incomplete work record.');
  }
  return data as View;
}

export interface HistoryPage {
 items: { requestRef: string; claimId?: string; taskType: string; status: string; completion?: {status:string;reason?:string;target?:string}; created: string; updated: string; traceId?: string; diagnosis?: { stage: string; code: string } }[];
 nextCursor?: string;
}
export const connectedSource = {
 async advise(request:ClaimRequest,signal?:AbortSignal):Promise<AccessPlan>{
  const data=await json('/api/access-advice',signal,{mode:'suggest',request}) as AccessPlan;
  if(!data||!['ready','clarification','failed'].includes(data.status)||typeof data.id!=='string'||!Array.isArray(data.questions)||data.status==='ready'&&(!data.request||shapeDiagnostics('ClaimRequest',data.request).length))throw new SourceError(502,'Advisor response is incomplete.');
  return data;
 },
 async autoSubmit(request:ClaimRequest,signal?:AbortSignal):Promise<View>{
  const data=await json('/api/access-submit',signal,{mode:'automatic',request}) as View&AccessPlan;
  if(data?.status==='clarification')throw new SourceError(422,data.questions?.join(' ')||'Clarify this task before starting.');
  try{return view(data);}catch{throw new SourceError(502,'Submission result is not confirmed. Check this request before retrying.',true);}
 },
 async agents(signal?:AbortSignal):Promise<AgentEntry[]> {
  const data=await json('/api/agents',signal);
  if(!Array.isArray(data)||data.length>200||!data.every(e=>e&&typeof e.project==='string'&&e.project.length<=128&&typeof e.canSubmit==='boolean'&&!shapeDiagnostics('AgentTemplate',e.template).length))throw new SourceError(502,'Agent catalog is incomplete.');
  return data;
 },
 async history(cursor = '', signal?: AbortSignal): Promise<HistoryPage> {
  const data = await json(`/api/history?limit=20&cursor=${encodeURIComponent(cursor)}`, signal) as HistoryPage;
  if (!data || !Array.isArray(data.items) || data.items.length > 20 ||
   !data.items.every(item => item && typeof item.requestRef === 'string' && typeof item.status === 'string' && typeof item.updated === 'string') ||
   (data.nextCursor !== undefined && (typeof data.nextCursor !== 'string' || data.nextCursor.length > 512))) {
   throw new SourceError(502, 'The connection returned an incomplete history page.');
  }
  return data;
 },
  async setup(signal?: AbortSignal, context?: { project?: string; template?: string }): Promise<Setup> {
    const params = new URLSearchParams();
    for (const key of ['project','template'] as const) { const value = context ? context[key] : new URLSearchParams(window.location.search).get(key); if (value) params.set(key,value); }
    const data = await json('/api/setup' + (params.size ? '?' + params.toString() : ''), signal);
    if (!data || typeof data !== 'object') throw new SourceError(502, 'Platform setup is unavailable.');
    const setup = data as Setup;
    if(setup.policy?.Rules===null)setup.policy.Rules=[]; // Go nil slice is an empty deny-all rule collection.
    if (shapeDiagnostics('Principal', setup.principal).length ||
        setup.installation?.kind !== 'installed' || !setup.installation.platform ||
        !/^sha256:[a-f0-9]{64}$/.test(setup.installation.revision) ||
        (setup.template !== null && shapeDiagnostics('AgentTemplate', setup.template).length > 0) ||
        [setup.projects, setup.templates, setup.submissionProjects].some(list => list !== undefined && (!Array.isArray(list) || list.length > 100 || !list.every(value => typeof value === 'string'))) ||
        !setup.policy || typeof setup.policy.ID !== 'string' ||
        typeof setup.policy.Version !== 'string' || !Array.isArray(setup.policy.Rules) ||
        !setup.policy.Rules.every(validPolicyRule) ||
        (setup.organization!==undefined && (!setup.organization || typeof setup.organization.id!=='string' || typeof setup.organization.displayName!=='string' || setup.organization.description!==undefined&&typeof setup.organization.description!=='string')) ||
        (setup.observability!==undefined && (!setup.observability || typeof setup.observability.enabled!=='boolean' || typeof setup.observability.exportFailures!=='number' || !Number.isFinite(setup.observability.exportFailures) || typeof setup.observability.ingestion!=='string' || [setup.observability.endpoint,setup.observability.diagnosticURL].some(value=>value!==undefined&&typeof value!=='string'))) ||
        !setup.capabilities || typeof setup.capabilities !== 'object' ||
        !Object.values(setup.capabilities).every(value => typeof value === 'string')) {
      throw new SourceError(502, 'This connection is not an installed Agenova Platform, or its setup is incomplete.');
    }
    if(setup.platformInfo && (typeof setup.platformInfo.operator!=='boolean'||!Array.isArray(setup.platformInfo.connections)||setup.platformInfo.connections.length>400||!setup.platformInfo.connections.every(c=>c&&typeof c.category==='string'&&typeof c.name==='string'&&Object.values(c).every(v=>typeof v==='string'&&v.length<=512))))throw new SourceError(502,'Platform configuration summary is incomplete.');
    return setup;
  },
  async list(signal?: AbortSignal): Promise<View[]> {
    const data = await json('/api/requests', signal);
    if (!Array.isArray(data)) throw new SourceError(502, 'The connection returned an incomplete work list.');
    return data.map(view);
  },
  async request(ref: string, signal?: AbortSignal): Promise<View> {
    return view(await json(`/api/requests/${encodeURIComponent(ref)}/evidence`, signal));
  },
  async submit(request: ClaimRequest, signal?: AbortSignal, adviceID?:string): Promise<View> {
    const result = await json('/api/requests', signal, request,adviceID);
    try { return view(result); } catch { throw new SourceError(502, 'Submission result is not confirmed. The response was incomplete; check the existing request.', true); }
  },
  async submission(name: string, signal?: AbortSignal): Promise<View> {
    return view(await json(`/api/submissions/${encodeURIComponent(name)}/evidence`, signal));
  },
};

export function workTitle(work: View): string {
  const input = work.request.spec.task?.input;
  return displayWorkName(input?.workName, input?.objective, work.requestRef);
}
export function workStatus(work: View): string {
 if (work.recovery) return 'Interrupted';
  if (work.state?.decision.result === 'Deny') return 'Denied';
  if (work.state?.decision.result === 'ApprovalRequired') return 'Approval required';
  // Queued work can be cancelled before allocation, leaving the canonical
  // claim Pending. The final outcome is authoritative for that work status.
  if (work.outcome?.status === 'Cancelled') return 'Cancelled';
  if(work.outcome?.status==='Succeeded'&&work.outcome.completion?.status==='incomplete')return 'Incomplete';
  if(work.outcome?.status==='Succeeded'&&work.outcome.completion?.status==='partial')return 'Partial';
  const phase = work.state?.claim?.phase;
  if (phase === 'Bound') return 'Starting';
  // Terminal claim authority can precede the final work/cleanup report.
  if (phase === 'Succeeded' && !work.outcome) return 'Finishing';
  return phase || 'Pending';
}
export function isTerminal(work: View): boolean {
  return ['Succeeded', 'Incomplete','Partial','Failed', 'Expired', 'Denied', 'Approval required', 'Cancelled', 'Interrupted'].includes(workStatus(work));
}

export function traceURL(traceId?: string, destination?: string): string | undefined {
 if (!traceId || !/^[a-f0-9]{32}$/.test(traceId)) return undefined;
 if (!destination && !import.meta.env.VITE_GRAFANA_URL) return undefined;
 let base: URL; try { base = new URL(destination || import.meta.env.VITE_GRAFANA_URL, window.location.origin); } catch { return undefined; }
 if (base.username || base.password || base.search || base.hash || !(base.protocol === 'https:' || base.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(base.hostname))) return undefined;
 return new URL(base.pathname.replace(/\/$/, '') + '/explore?schemaVersion=1&panes=' + encodeURIComponent(JSON.stringify({ work: { datasource: 'tempo', queries: [{ datasource: { type: 'tempo', uid: 'tempo' }, query: traceId, queryType: 'traceql', refId: 'A' }], range: { from: 'now-7d', to: 'now' } } })), base).href;
}
