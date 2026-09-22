// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';
import './incident-role-demo.css';

type Role = 'developer' | 'sre';
const request = {
  template: 'payment-incident-responder',
  objective: 'Investigate the payment retry incident; recommend a safe recovery action.',
  scope: 'service:payments',
  tools: ['diagnostics.read', 'git.read', 'github.pull-request', 'deploy.rollback'],
};
const identities = {
  developer: {
    label: '开发工程师', team: 'Payments Development', principal: 'demo:payments-developer',
    allowed: ['diagnostics.read', 'git.read', 'github.pull-request'],
    action: '准备修复 PR', denied: 'deploy.rollback',
    explanation: '可以阅读事故证据、检查代码并准备修复 PR；不能直接回滚生产部署。',
  },
  sre: {
    label: '值班 SRE', team: 'Reliability', principal: 'demo:on-call-sre',
    allowed: ['diagnostics.read', 'git.read', 'deploy.rollback'],
    action: '执行受控回滚', denied: 'github.pull-request',
    explanation: '可以阅读同样的事故证据，并在治理边界内回滚；不能以开发者身份提交修复 PR。',
  },
} as const;

export function IncidentRoleDemo() {
  const [role, setRole] = useState<Role>('developer');
  const identity = identities[role];
  return <div className="incident-demo">
    <div className="incident-demo-intro">
      <span className="incident-demo-kicker">SIMULATED SCENARIO · NOT CONNECTED EVIDENCE</span>
      <h1>一次事故，两种临时授权</h1>
      <p>支付服务重试超时。两个团队复用同一个 Agent 模板和同一份任务请求；提交者身份不同，最终可执行的操作不同。</p>
    </div>
    <div className="incident-demo-warning" role="note"><strong>演示模拟</strong> · 此页只展示目标产品的权限设计。切换身份不会改变 Connected 服务的可信身份、PolicyBundle 或实际 Work；下方没有发生真实 PR 或回滚。</div>
    <section className="incident-demo-card incident-demo-identity" aria-label="模拟提交者">
      <div><span className="incident-demo-overline">01 / WHO REQUESTS</span><h2>模拟提交者</h2><p>选择角色，观察相同工作如何获得不同的短期能力。</p></div>
      <div className="incident-demo-role-switch" role="group" aria-label="模拟身份切换">
        {(['developer', 'sre'] as const).map(key => <button key={key} type="button" aria-pressed={role === key} onClick={() => setRole(key)}>{identities[key].label}<small>{identities[key].team}</small></button>)}
      </div>
    </section>
    <div className="incident-demo-grid">
      <section className="incident-demo-card"><span className="incident-demo-overline">02 / SAME WORK</span><h2>不变的请求与模板</h2>
        <dl><div><dt>AgentTemplate</dt><dd><code>{request.template}</code></dd></div><div><dt>任务目标</dt><dd>{request.objective}</dd></div><div><dt>资源范围</dt><dd><code>{request.scope}</code></dd></div></dl>
        <p className="incident-demo-small">请求的工具（不是授权）：</p><div className="incident-demo-chips">{request.tools.map(tool => <code key={tool}>{tool}</code>)}</div>
      </section>
      <section className="incident-demo-card incident-demo-grant" aria-live="polite"><span className="incident-demo-overline">03 / DIFFERENT AUTHORITY</span><h2>{identity.label}的模拟授权</h2>
        <p className="incident-demo-principal">{identity.principal} · {identity.team}</p>
        <div className="incident-demo-decision allow"><span>允许</span><strong>{identity.action}</strong></div>
        <div className="incident-demo-decision deny"><span>拒绝</span><strong>{identity.denied}</strong></div>
        <p>{identity.explanation}</p>
      </section>
    </div>
    <section className="incident-demo-card incident-demo-boundary"><span className="incident-demo-overline">04 / WHY IT MATTERS</span><h2>共用诊断能力，不共享常驻权限</h2>
      <div className="incident-demo-chips">{identity.allowed.map(tool => <code key={tool}>{tool}</code>)}</div>
      <p>两人都能看同一事故的诊断信息。请求本身不能授予权限；最终授权应由可信身份、组织策略、模板上限与任务需求共同收窄。这里是模拟结果，真实的按团队能力上限和身份接入仍待实现。</p>
    </section>
  </div>;
}
