// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import type {AccessPlan} from './connected-source';
export function AccessAdviceReview({plan}:{plan:AccessPlan}){
 return <div className="portal-advice" aria-label="Access advice">
  <div className="portal-advice-heading"><h3>{plan.status==='ready'?'Suggested permissions':'Clarification needed'}</h3><span className="portal-muted">{plan.engine==='model'?plan.model?.name||'Model advisor':'Registered task rules'}</span></div>
  {plan.explanation&&<details className="portal-advice-reason"><summary>Why these permissions?</summary><p>{plan.explanation}</p></details>}
  {plan.questions.map((question,i)=><p key={i} role="status">{question}</p>)}
 </div>;
}
