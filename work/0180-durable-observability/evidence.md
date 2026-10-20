# #180 实现与验证记录

本分支实现并部署了 SQLite 持久证据、分页历史、故障诊断、OTEL 三类信号，以及由 Platform 管理的独立 Collector / Grafana / Tempo / Loki / Prometheus。部署位于既有 `kind-agenova-k8s-lab` 集群的独立 `agenova-observability-180` 命名空间，原 `agenova-system` 安装未修改。

代码未提交或合并；基础提交、分支、源文件摘要见 [source.json](evidence/source.json) 和 [source-manifest.json](evidence/source-manifest.json)。实际运行镜像的内容标识见 [images.json](evidence/images.json)，不是仅凭浮动标签作出验证声明。

## 已验证结果

| 场景 | 结果与证据 |
| --- | --- |
| 真实 Ollama 正例 | `qwen3:8b`，成功结果和关联调用：[success.json](evidence/success.json) |
| admission 拒绝 | 无 claim、无 ProviderAttempt：[denied.json](evidence/denied.json) |
| 控制面重启 | 终态证据完整且与 CLI/API 一致：[after-restart.json](evidence/after-restart.json) |
| Collector 停机 | Work 仍成功，恢复后新拒绝事件进入专用指标：[collector-outage-work.json](evidence/collector-outage-work.json)、[decisions.json](evidence/decisions.json) |
| 真实三类信号 | [Tempo trace](evidence/tempo-trace.json)、[Prometheus metrics](evidence/prometheus.json)、[Loki logs](evidence/loki.json) |
| Runtime / Provider 两类故障 | 专用 Role 的 sandboxclaims 权限临时移除后恢复；缺失模型通过显式 Platform revision 配置后恢复。均有独立失败原因、持久事实及 CLI/API 一致性：[fault-result.json](evidence/fault-result.json)、[runtime-failed.json](evidence/runtime-failed.json)、[provider-failed.json](evidence/provider-failed.json) |
| 数据保留的清理/重装 | 删除五个观测组件的 Deployment/Service/ConfigMap，保留全部五个 PVC UID，再由 Platform apply 重装；历史 Work 仍可查询：[cleanup-result.json](evidence/cleanup-result.json) |
| 安装负例 | 缺失存储类拒绝 apply；通过既有测试 wrapper 使用真实 Kubernetes impersonation，受限 default ServiceAccount 的 plan 被拒绝，未复制 kubeconfig 凭证：[存储负例](evidence/missing-storage-result.json)、[RBAC 负例](evidence/denied-rbac-result.json) |
| 数据一致性与隐私 | SQLite 事务回滚、提交后进程退出、并发追加、独占写入、未知 schema 拒绝、v1→v2 保留数据迁移、加密游标重启/篡改、查询授权、失败写入阻断 dispatch 均有自动化测试 |
| 全量检查 | 干净源快照执行 `pwsh -NoProfile -File ./scripts/check.ps1 -All` 通过，包含 **54 个浏览器 smoke**：[full-gate.log](evidence/full-gate.log) |
| 并发检查 | `go test -race ./internal/history/... ./internal/facts ./internal/console ./internal/telemetry ./internal/connectedclient` 通过：[race.log](evidence/race.log) |

原工作区的 `-All` 被既有 `tmp/chatterbox-venv/.../empty_template_main.go` 缺少 SPDX 头拦截；没有改动这些无关文件。全量 gate 在 `/private/tmp/agenova180-verify` 的同源干净快照运行，使用自己的临时 Git 基线满足 gate 的依赖文件漂移检查，没有在用户仓库提交代码。

## 界面与复现

- [Grafana 仪表盘截图](evidence/grafana.png)、[从 Portal 实际跳转后的 Trace](evidence/grafana-trace.png)、[跳转关联记录](evidence/trace-navigation.json)
- [Portal 历史截图](evidence/portal-history.png)
- [成功 Work](evidence/portal-success.png)、[Runtime 故障](evidence/portal-runtime-failure.png)、[Provider 故障](evidence/portal-provider-failure.png)
- [最终 Platform 状态](evidence/final-status.json)、[运行资源](evidence/installed-resources.txt)、[授权历史页](evidence/final-history.json)

构建 CLI 到 `.tmp/agenova-180`，按 [参考安装与验证说明](../../harness/observability/README.md) 构建并加载镜像。三个主验证命令按顺序运行，不要并行改变同一测试安装：

```sh
node harness/observability/verify.mjs --context kind-agenova-k8s-lab --model qwen3:8b
node harness/observability/faults.mjs --context kind-agenova-k8s-lab
node harness/observability/render.mjs --context kind-agenova-k8s-lab
```

可另用 `cleanup-verify.mjs --context kind-agenova-k8s-lab` 重现观测栈清理/重装。它保守地要求当前历史页没有活动 Work 且没有未检查的下一页；它不会删除命名空间或 PVC。

CLI 下一页可用 `agenova work history --json -- <cursor>`；`--state-dir` 等选项放在分隔符 `--` 之前，避免不透明游标被识别成命令行选项。

本地 Grafana 入口：

```sh
kubectl --context kind-agenova-k8s-lab -n agenova-observability-180 port-forward service/agenova-grafana 3000:3000 --address 127.0.0.1
```

打开 `http://127.0.0.1:3000/d/agenova/agenova-work-diagnostics`。管理员登录信息由该专用命名空间的 `grafana-admin` Secret 管理，报告和截图不保存密码。Portal 的 trace 链接默认指向这个 loopback 入口。

## 明确限制与剩余依赖

- **#180 未关闭**：E14 的真实用户身份和 read-policy 联验尚未交付。本次提供默认拒绝的可信 viewer/authorizer 接口及隔离测试；实际参考安装仍是明确标注的固定身份、loopback 模式，不是 OIDC/多租户验证。
- 单控制面、SQLite 本地 PVC；参考服务仍有每进程 32 个请求的容量限制。历史跨重启保留；生产 HA、安全活动 Work rollout、自动恢复/重放不在本次交付内。
- 崩溃后的旧 claim 不恢复调用权限；未完成 attempt 保持未知，cleanup 未知就明确显示未知。进程崩溃/WAL 用真实 SQLite 子进程测试，不把静态清单当作真实 provider 恢复证据。
- 最终重复验证中出现过一次真实 Provider 失败，之后配置核对及正例复验通过。失败记录也保留：[intermittent-provider-failure.json](evidence/intermittent-provider-failure.json)。没有用 mock 或自动重试改写结果。
- 早先 PRD/公共说明修改被自动审批拒绝；用户随后明确重申授权（“不会缺少授权的 你看下”）。现已同步 PRD 的 #180 承诺范围，并由 README 链接本分支验证报告；未将本分支证据写成已合并状态，E14 验收依赖仍保留。

## 推送前复验

暂存源码的干净快照通过 `pwsh -NoProfile -File ./scripts/check.ps1 -Profile PR`，包含完整 Go race 和 54 项浏览器测试：[日志](evidence/pr-gate.log)。原沙箱禁止测试监听本地端口，使用测试所需权限重跑后通过。

刷新后的远端 main 比本分支基线多 135 个提交，已包含 E14 #207。本报告中 E14 未就绪的描述针对原测试基线；尚未整合和验证最新主分支，因此发布为草稿 PR，保留真实身份联验及主分支整合要求。

## PR #235 冲突整合验证

已整合主分支 `3c3556321`，保留 E14 的身份、会话撤销、项目权限、工作负载身份、PostgreSQL 持久化及真实工具后端。SQLite 仅用于参考模式；成员模式沿用 PostgreSQL，不以固定身份覆盖成员身份。

成员历史分页在数据库权限过滤后执行，复用单条证据的权限规则；每页重新授权。PostgreSQL 分页游标在 writer 重启后失效，需从首页重新查询，历史证据不会删除。私有 admission 快照不进入公开 evidence JSON。

本次新增实际 PostgreSQL 的分页/撤权/篡改负例及 HTTPS 历史查询凭据和重定向隔离测试。此前 kind/Ollama 镜像和截图对应原源码记录，不作为整合后镜像的部署证明；真实外部身份联验仍未宣称通过。

整合后的完整 `-Profile PR` 在锁文件依赖安装后的干净源码快照上通过：[完整日志](evidence/merge-pr-gate.log)。包含全量 Go 测试、全量 race、63 项浏览器测试、契约/文档/生产构建；数据库测试使用真实临时 PostgreSQL 16.6：[focused race 日志](evidence/merge-postgres-race.log)。[整合源码校验记录](evidence/merge-source.json)记录主分支基线及实际验证的 61 个实现/测试文件哈希。

整合后 UI 的注入证据用例已补充截图并目视检查：[历史分页](evidence/merge-fixture-history.png)、[中断 Work 与失效权限](evidence/merge-fixture-interrupted.png)。它们仅证明 UI 对受控证据的渲染，不充当真实身份或后端部署证明。

## Existing Google session / Grafana extension

The user authorized Grafana Google access and reuse of existing system accounts.
Read-only inspection through the existing Google login confirmed one Owner and
four Platform Admins; no emails, Google credentials or roster are copied into
source. The bridge consumes E14's current session and installation diagnostics
permission on every request and gives Grafana an opaque Viewer identity only.

- [Full PR gate](evidence/google-pr-gate.log): Go/race, contracts/build and 63 browser cases passed in a clean snapshot.
- [Real Grafana protocol](evidence/google-real-grafana.log): actual pinned Grafana 11.4 backend accepted the trusted opaque identity as Viewer; revoked access was rejected. Identity/E14 are controlled test fixtures, not a real Google-to-EKS login claim.
- [Read-only EKS preflight](evidence/google-eks-preflight.json): no installed Grafana, no EBS CSI driver for the advertised gp2 class; existing EFS CSI is not silently substituted for the planned Grafana PVC.
- [Deployment/trust boundary](../../deploy/production/grafana-google.md): private ingress, credential isolation, image-only member rollout boundary and persistent install prerequisites.

No EKS deployment, IAM modification, new Google client or new public endpoint was applied by this extension. Real Google positive/negative login, private-network denial and persistent restart evidence remain deployment gates.

Standalone Linux/amd64 bridge image build passed: [build log](evidence/google-image-build.log). The local manifest list is `sha256:388d25b82bbbd339ef61f4772d4c50dffc592485f7583569b66af0c304c65e3d`; this is a local build, not an ECR publication or EKS rollout. The optional [NGINX route fragment](../../deploy/production/grafana-location.conf) is prepared separately and has not been applied.

## Administrator dashboard editing (20 October 2026)

Owner authorized mapping administrators to Grafana Editor and pushing to #235.
Live E14 membership grants Owner/Platform Admin Editor, installation operators
Viewer, and denies ordinary or disabled members. Only dashboard create/save is
added to the proxy write allowlist with exact same-origin checking. Role-scoped
opaque Grafana identities avoid stale permission caches during role changes.

Actual disposable Grafana 11.4 verified Editor dashboard save/read and immediate
Viewer write denial, session/role revocation and credential isolation. Actual
PostgreSQL verified membership, installation scope and revoked administrator
checks with race detection; see [focused output](evidence/editor-focused.log).
The member authorization endpoint must be upgraded before the standalone bridge.
Provisioned dashboards remain source-managed; customize using Save as copy.
No EKS rollout, storage/IAM change or live Google-to-Grafana acceptance is claimed.

Final `check.ps1 -All` passed in the clean source snapshot, including all 63
browser smoke cases; see [full gate](evidence/editor-all.log). Final packet/docs
checks and PR delivery contract passed. Original workspace recursive checks
retain the previously recorded unrelated `tmp/chatterbox-venv` SPDX blocker.
