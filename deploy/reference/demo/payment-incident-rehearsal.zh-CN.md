# 支付重试故障调查：中期录屏脚本

## 场景

支付服务发出告警：重试请求可能超过五秒总时限。值班工程师让同一个 `engineer` 模板调查原因，并提出修复建议。本次 Work **只读调查**，不改代码、不退款、不回滚。

这是一类真实企业故障工作的缩小版，不是接入真实支付系统：`git.read` 返回明确标注的合成 README、超时日志和重试逻辑片段。kind worker 和 Ollama 推理是真实运行的。

## 演示配置

- Platform：`deploy/reference/platform.kind.yaml`，选择 kind、Agent Sandbox 和本机 Ollama。
- Policy：`deploy/reference/demo/policy.yaml`，固定可信 Team A 可在 `payments` 项目提交 `engineer`；其他匹配不到的提交默认拒绝。当前 Policy **只决定能否提交**，不提供团队专属的工具上限。
- AgentTemplate：`deploy/reference/demo/engineer.yaml`，`git.read` 是工具上限；真实 worker 镜像为 `agenova-testworker:kind`。
- Work：`deploy/reference/demo/payment-incident-work.yaml`，请求 `git.read` 和 `git.write`；后者超出模板上限，必须从实际授权中收窄。模型最终只能基于读到的材料提出建议。

## 录屏顺序

按照 [kind + Ollama CLI 手册](../../../docs/reference-cli-kind-ollama.md)准备 Docker、kind、Agent Sandbox、Ollama、镜像与 CLI。开始录屏前先运行一次 `ollama run llama3.1:latest "Reply OK only."`，确认模型已加载；本地演练的首次调用曾在两分钟后超时，预热后复测成功，但尚未单独证明超时原因。然后在仓库根目录运行：

```powershell
agenova platform validate -f deploy/reference/platform.kind.yaml
agenova platform plan -f deploy/reference/platform.kind.yaml
agenova platform apply -f deploy/reference/platform.kind.yaml
agenova platform status
agenova policy apply -f deploy/reference/demo/policy.yaml
agenova agent-template apply -f deploy/reference/demo/engineer.yaml
agenova run -f deploy/reference/demo/payment-incident-work.yaml --json
agenova work show investigate-payment-retry-incident-recording --json
```

录屏重点：同一份 evidence 中的 `Allow`、请求的 `git.write` 被收窄、kind worker、真实模型调用、mock `git.read` 的工具决策、任务回答及清理。不要把“请求了写权限”说成“已经执行了写操作”。

若要展示 UI，在另一终端运行 `agenova api connect`，再按 CLI 手册启动本地 React Portal 的 Connected 模式，打开上述 Work。UI 与 CLI 查询的是同一安装服务的同一份记录。重跑时先修改 `metadata.name`，因为同一服务会话内 request ID 不可重复。

## 2026-09-22 本机预演结果

- 相同配置的新 request `investigate-payment-retry-incident-r2` 和 `-r3` 连续在 kind + Ollama 上 `Succeeded`：两次均有 4 次模型决策、3 次 mock 工具决策、48 条事实，回答指出重试循环错误地为每次尝试重置五秒 deadline；worker 清理成功。
- 请求的 `git.write` 被模板上限剔除，实际工具只有 `git.read`；拒绝用例 `investigate-unapproved-project` 无 claim、无 worker、无模型调用。
- CLI 与 `/api/requests/.../evidence` 返回同一个 request；仓库已有的 installed Playwright 测试核对了允许和拒绝两条 UI 路径，预热后 2/2 通过。
- 首次提交在 Ollama 调用两分钟上限处失败并留下失败证据；第二次成功。录屏前须预热模型，并确认 Docker Desktop 状态为 `running`。不要把首次失败改称成功或隐去这一稳定性风险。

## 终期目标，不属于这次录屏

支付开发工程师和 SRE 各自用可信身份提交**完全相同**的故障处理请求；两人都可读完整诊断证据，但组织策略分别允许提交修复 PR 或执行受控回滚。当前参考服务固定 Team A，Policy 只有提交准入规则，worker 只接 mock `git.read`。因此上述两人差异、真实 PR 和回滚都不能由本目录的 YAML 假装实现；须完成身份、团队权限上限及相应受控工具后再录终期演示。
