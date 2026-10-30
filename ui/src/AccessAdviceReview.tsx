// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type {AccessPlan} from './connected-source';
export function AccessAdviceReview({plan}:{plan:AccessPlan}){
 const access=plan.request?.spec.requestedAccess;
 return <section className="portal-panel portal-advice" aria-label="Access advice">
  <div className="portal-advice-heading"><h2>{plan.status==='ready'?'Suggested permissions':'Clarification needed'}</h2><span className="portal-muted">{plan.engine==='model'?plan.model?.name||'Model advisor':'Registered task rules'}</span></div>
  <p>{plan.explanation}</p>
  {plan.questions.map((question,i)=><p key={i} role="status">{question}</p>)}
  {access&&<dl className="portal-advice-values"><dt>Tools</dt><dd>{access.tools?.join(', ')||'None'}</dd><dt>Resources</dt><dd>{access.resourceScopes?.join(', ')||'None'}</dd><dt>Model profile</dt><dd>{access.modelProfile||'Template default'}</dd><dt>Direct network</dt><dd>{access.networkProfiles?.join(', ')||'None'}</dd></dl>}
 </section>;
}
