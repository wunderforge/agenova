// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type {View} from './connected-source';
export function AccessPlanningEvidence({work}:{work:View}){
 const a=work.accessAdvice;if(!a)return null;
 return <details className="portal-details"><summary>Permission preparation</summary><div className="portal-disclosure-body"><dl className="portal-advice-values">
  <dt>Mode</dt><dd>{a.mode==='automatic'?'Automatic':'Suggestion accepted'}</dd><dt>Planner</dt><dd>{a.plannerModel||'Registered task rules'}</dd><dt>Profile</dt><dd>{a.profileID}</dd><dt>Guide version</dt><dd>{a.guideVersion}</dd><dt>Guide digest</dt><dd className="portal-break">{a.guideDigest}</dd><dt>Planning tokens</dt><dd>{a.plannerInputTokens||0} input / {a.plannerOutputTokens||0} output</dd>
 </dl></div></details>;
}
