# 本地产品验收：从 Agent 到逐任务权限

2026-10-23 · 分支 `codex/final-demo-product-local` · 基于 main `58d951c`。

本轮未提交、推送、合并或部署，没有调用 AWS、变更共享集群、DNS、真实 OAuth 或凭证。

## 先打开这里

[进入本地验收](http://127.0.0.1:5210/login)，选择 **owner → Enter local workspace**。

这不是静态 mock：页面使用实际 Go API、授权引擎、Tool/Model Gateway、PostgreSQL 和真实 Go MCP 服务，管理修改及 Work 证据会保存。**登录身份、运行实例、模型回答和客户数据是合成实现；公开资料经 MCP 实时 HTTPS 获取。** 这不是 Google、真实模型或包级网络隔离验收。页面有持续标识；生产登录逻辑没有增加此测试入口。

## 本轮优先验收：Researcher + 两种业务任务

### 最新验收：详情页与缺权限失败

- 本轮新增：失败且有真实拒绝记录的任务，在标题下、Permissions 上方显示浅琥珀色 `<tool> blocked` 横条，右侧只有 `Evidence`，无箭头／附加解释。成功任务的拒绝记录仍在权限表下；没有拒绝事实的普通失败不生成横条。刷新你当前的失败任务即可验收，点击 Evidence 应打开对应拒绝记录。
- 刷新任意实际 Work 详情：先看 Permissions。只列本次 requested 的工具、资源、模型、网络／Memory（若申请）、Runtime 和 Time limit；同样的值统一黑色、正常字重，只有申请但没获批的 Granted 项为灰色。未申请的能力不列出。
- Details 集中显示 Agent、调用人、Team／Project、历史 Policy、实际出现的 Gateway、Backend、网络证据等级和清理状态。Result 保留原始内容；Activity & evidence 可展开查看原有运行进度、调用、完整记录及访问详情。没有把当前配置填到历史任务中。
- 从 researcher-v2 创建 Market research，目标填写 `Read https://jitsi.org/jitsi-meet/ and summarize its meeting features for a consulting team.`。勾选 web.read、资源 web:public-pages 应成功。
- 再复制任务，取消 web.read、勾选 crm.read，但保留 web:public-pages；提交后应 Failed、显示 `Blocked · web.read`，有 View evidence 入口，且没有 Result。也覆盖 web.read + 错误 CRM scope、无工具／无 scope 两种失败。这里顶层各项获批不代表所有工具与任意 scope 的组合都可调用。
- 拒绝来自实际本地 Gateway，不是 UI 伪造。执行器不再因空工具目录直接调用合成模型返回默认成功；低层独立 Provider 计数测试验证拒绝时为零，实际 API 证据也无 ProviderAttempt。普通无工具任务仍可完成；Researcher 的确定性任务解释不作用于旧 coding Agent。
- 之前的 `work-4185e2ddb40fef18c92221eba01ef127df4858a05519e348e76542c16a0f15a5` 是修复前历史记录，仍保持原有成功状态及结果，不能作为修复后的新试验。

本轮仍只在本地：真实 Go API／授权／PostgreSQL／MCP，合成身份／Runtime／模型。没有合并、部署或调用 AWS。

[查看新成功任务](http://127.0.0.1:5210/?mode=connected&project=research&template=researcher-v2#/work/work-7cdc690bc36020a0fb7d31846ab3ffb511371289a1928f87731dbc3ba53f3c46) · [查看缺少网页权限的失败任务](http://127.0.0.1:5210/?mode=connected&project=research&template=researcher-v2#/work/work-f4cb7374da4a2a1b2fea63a878162b277ae79e561ca3337d91c129029c1cbf4a)。这些是同一 research 项目中新创建的独立任务，均使用实际本地 API。

本次通过 161 条 UI 单测、80 条浏览器 smoke、最终版本 9 条实际 API／SQL／MCP 联合旅程。原始 Windows 总门禁仍受既有 owner bootstrap 目录 Sync 测试影响；没有修改该安全写入逻辑来绕过。详细失败／复测记录见任务 acceptance。

### 最新修正：无 key 的公开网页读取

[直接创建新版 Researcher 任务](http://127.0.0.1:5210/?mode=connected&project=research&template=researcher-v2#/work/new)。已有页面请刷新；旧 `researcher` 注册不可变，因此保留旧模板并新增 `researcher-v2`，不覆盖旧任务或 Policy 快照。

- 选择 **Task preset → Market research**，只申请 `web.read` / `web:public-pages`。任务描述初始为空，示例只是 placeholder；填入例如 `Read https://jitsi.org/jitsi-meet/ and summarize its meeting features for a small consulting team.`。
- 这是可信 MCP 实际读取指定的公开 HTTPS URL，**不是关键词搜索**，不需要 Tavily、账号或 API key。页面内容不可信；内网、云元数据、非 HTTPS、URL 账号密码、重定向被拒绝，响应与时间受限。
- 另一单选择 **Customer insights**，填写自己的客户分析目标，仅申请 `crm.read`。对比页应显示同人同模板但不同工具权限；两单均无 Agent Pod 直连公网权限。
- 演示真实 Gateway 越权拒绝可在网页任务描述末尾加 `Local acceptance probe: cross-scope CRM access.`。本地执行器是合成的；它只提取描述中第一个 HTTPS URL，不是模型理解任务或真实 LLM 答案质量验收。
- 预设切换不覆盖已输入的描述／名称；Custom 不会偷偷填业务任务。下拉框鼠标选择后不保留紫色光环，键盘 Tab 仍有可见焦点。

最新回归：156 条 UI 单测、79 条浏览器 smoke、6 条实际 API／PostgreSQL 联合旅程通过；Go 回归及 vet 通过，Go 仅显式排除下文记录的既有 Windows 目录 Sync 测试。网页读取及跨工具拒绝截图已实际检查。

下面保留的 `researcher` / `web.fetch` 步骤是旧固定资料源回归路径；本次重新验收优先使用上述 v2。

1. 用 owner 登录，打开 **Agents**。目录是可搜索表格，Project／Tool 是 checkbox facets，不再强制用 Project 下拉导航。保留 engineer、incident-reader，新增 researcher；同名旧模板按项目注册分行显示，不误合并不同配置。
2. 输入 `project:research`，只应看到 researcher；打开详情，检查镜像／入口／ceiling，再点 Run。payments、billing 不能运行 researcher；浏览器伪造请求也会被服务器拒绝。
3. **Public product research** 预设：只申请 `web.fetch` + `web:product-comparison`。MCP 实际读取 GitHub Issues、GitLab Issues 的固定官方文档；结果明确标注“收集资料，不是真实 LLM 比较”。若要同时看越权拒绝，在 objective 加 `cross-scope`；本地执行器会确定性尝试 crm.read，由真实 Gateway 拒绝。
4. 再从同一 Agent 创建 **Customer retention review**：只申请 `crm.read` + `crm:customer-retention`。读取 MCP 服务端只读文件中的合成客户数据，显示 Northstar 等优先跟进记录；无需新 DB Pod／凭证。
5. **Compare two Work**：同 owner、research 项目、researcher、Policy；两边的工具和资源授权不同，两者都没有直连公网权限。预设只填写申请，不是 LLM 自动判权。
6. Administration 默认 Users 表格。搜索用户 → Edit → 单用户侧栏，分别保存 research／payments／billing 授权，不改其他项目；平台职责整体替换仍单独警告。组织显示名可编辑，可信 team ID 不可随意改名。
7. 用 viewer 登录：仍能发现 research 和点 Run，但真正提交收到拒绝。Owner 撤掉该用户的 research 查看权后，新目录响应不再包含 Researcher／该项目 facet；页面筛选本身不授予权限。
8. Platform 是六张等权配置卡：Model／Tool／Policy／Runtime／Observability／Memory。Operator 可下钻安全配置摘要；普通成员不获得安装地址或全平台拓扑。未配置、未验证要明确显示，不能当作服务健康。

**仍未通过的安全项：**现有本地 kind 只有 kindnet／kube-proxy，没有 NetworkPolicy 执行插件；本轮没有替换 CNI 或修改共享集群。因此已经证明“Researcher 无公网授权、伪造 public-web 申请被拒绝、真实 Gateway 阻止交叉工具调用”，但没有把原生 Pod 公网超时当作 infra 拦截证明。需在另行批准的、执行 NetworkPolicy 的隔离环境补成功对照和绕过测试。旧模板的显式 public-web 能力没有全局删除，以免破坏已支持场景；本轮 Researcher 路径强制 Gateway-only。

## 建议的 10 分钟验收顺序

| 顺序 | 操作 | 应看到什么 |
|---|---|---|
| 1 | 退出登录，看首页，再重新进入 | June 风格；问题与产品说明；开源可用、托管/catalog/marketplace 明确 Planned；Sign in 正常进入本地测试登录 |
| 2 | Agents，筛选 payments，打开 `engineer` / `incident-reader` 详情 | 原模板保留；镜像引用、入口、能力上限来自配置，不伪造 digest 或兼容性认证 |
| 3 | New work，选 payments / engineer / Investigation | 真正的 Project 下拉；默认无工具、无资源、无公网；右侧显示实际申请摘要 |
| 4 | 创建 A，勾选 `git.read`，scope 填 `repo:acme/payments` | Granted 只含 payments；有结果、调用记录和清理记录 |
| 5 | 复制 A 为新 Work，将 scope 改为 `repo:acme/billing` 后提交 B | 新标识、新授权；不是重放 A 或复制已签发权限 |
| 6 | Work → Compare two Work，选 A/B | 同调用者、模板、Policy，Requested/Granted 的资源不同，并排高亮 |
| 7 | 再建一个 payments Work，任务描述加 `cross-scope` | 测试执行器确定性尝试 billing；真实 Gateway 拒绝越权调用，但允许的任务仍完成。首屏摘要 → Open evidence |
| 8 | Administration，修改组织显示名，逐项目编辑成员权限 | `team-a` 不变；修改 billing 不覆盖 payments；平台职责整体替换操作另有明确警告 |
| 9 | Administration → Policy & Templates，执行下方 Policy 更新实验 | 发布前显示 Before/After 与完整候选；新 Work 用新 Policy；历史 Work 不变；可重新激活旧版本 |
| 10 | Platform → Observability；再以 viewer 登录 | 当前观测 Not configured，不生成虚假 Grafana 链接；viewer 可检查授权可见配置，但不能提交 Work 或修改组织 |

A 的任务可写：`Investigate the payment retry incident using only the requested evidence.`；B 保持同人同 Agent，只改任务资料范围即可。这里的 `repo:` 是现有工具契约，不代表所有任务都必须是 coding；任务类型已不再固定为 repository-change。

### 已准备的直达记录

这些是本地真实 API 产生的合成环境记录，不是线上客户数据。先按上面登录，再点击：

- [A：payments + 越权拒绝](http://127.0.0.1:5210/?mode=connected&project=payments#/work/work-5d413a993a264a32e686adf9a2d2478ead99fe8c45d6b7bc7e6ab72471d47782)
- [B：billing](http://127.0.0.1:5210/?mode=connected&project=payments#/work/work-acd049e2192b13725e2489b400e643b4a8354925ba7185df8995756d0b981dab)
- [A/B 并排对比](http://127.0.0.1:5210/?mode=connected&project=payments#/compare?left=work-5d413a993a264a32e686adf9a2d2478ead99fe8c45d6b7bc7e6ab72471d47782&right=work-acd049e2192b13725e2489b400e643b4a8354925ba7185df8995756d0b981dab)
- [同一业务请求：Policy 允许版](http://127.0.0.1:5210/?mode=connected&project=payments#/work/work-13ed93ab1d4201b66a61b6ba9ab682ea927b44a49f292127b214b8117808898e)
- [同一业务请求：Policy 拒绝版](http://127.0.0.1:5210/?mode=connected&project=payments#/work/work-dbe8b3cce5f3a9055417bfe41c4c63332938e689622ce99fbdb6bae81d0c69fa)
- [回滚后再次允许](http://127.0.0.1:5210/?mode=connected&project=payments#/work/work-6dcc6bf395de5c370577ae9f38e1232f476a7f6fd1f2aff01727c18cc7c49e4b)

## Policy 热更新怎么试

1. 在 payments 项目先完成一个允许任务，保存链接。
2. Administration → Policy & Templates：选 payments / Policy → Load current configuration。
3. 保存原 JSON 以便比较；把 `metadata.version` 改成一个新值，比如 `owner-deny-1`，把 `spec.rules` 改为 `[]`。这是显式 deny-all 实验，只改本地测试项目。
4. Save configuration → 查看差异 → Publish reviewed configuration。保存本身不是未经预览直接上线。
5. 复制原任务并直接提交，不改业务输入或申请权限。应准入拒绝；不是模型自己拒绝，也不是连接故障。
6. 回到 Administration，重新加载当前配置，从 Immutable versions 选择 `local-work-boundary / 1`，预览并重新激活。
7. 再复制相同请求，应重新允许。打开之前两份 Work，旧 Policy 版本和授权快照不变。

版本内容不可覆盖。两个管理员基于同一旧版本发布，后到者应收到冲突并重新加载；当前 UI 使用这一保护，旧 CLI 的兼容调用仍可不传 precondition。Policy 更新仅影响新 Work，不宣称立刻撤销已有 Work。

## 异常恢复怎么看

提交超时／断网／收到不完整响应时，界面标明“结果未确认”，保留原请求标识，先 **Check existing request**；同页可 **Retry identical request**，不会换名生成新任务。页面重新打开只恢复标识，不保存业务输入或 token。明确选择另一任务时会提醒：之前的 Work 并未取消，可能继续执行。

真实控制台故障不自动切换示例模式。连接或权限无法重新确认时隐藏业务记录，保留路由与恢复入口；**没有为了展示连续性把旧敏感数据长期缓存给可能已被撤权的用户**。示例工作区是独立、明确标识的入口。

## 安全边界与观测的验收口径

- Policy、Gateway、Infrastructure 分栏；配置存在、执行事实、独立验收证据分开。
- 本地合成运行实例会显示 **Simulated network lifecycle**，不是 NetworkPolicy 拦截证明。
- `cross-scope` 检验本地实际 Gateway 授权，不是已在 AWS Pod 中绕过 SDK 的网络攻击。
- 超时只说明访问失败；不自动归因到 infra。AWS 公网放行成功对照、重建 Pod 等仍需在另行批准的实际环境复验；未把 #247 网络分支混入这次 UI 工作。
- Platform 只读展示启用状态、OTLP receiver、导出失败计数及目的地验证状态；安装 revision 在同页。未配置则 Not configured，不能用以前 CloudWatch 验证冒充当前连接。
- 下游接收无法从应用单独判定；不会自动宣称三类信号均 Connected。不部署 Grafana/Collector，不收费连接 CloudWatch。
- 可配置 `AGENOVA_DIAGNOSTICS_URL` 作为导航，允许安全 HTTPS、开发用 loopback HTTP 或现有 `/grafana` 鉴权代理；禁止 URL 用户名密码、查询凭证或片段。该变量不配置 exporter、IAM 或下游资源。

<details>
<summary>本地重启、测试命令与资源</summary>

在 checkout 根目录运行：

```powershell
.\scripts\local-product-preview.ps1
```

前提：Docker Desktop 运行，已缓存 `postgres:17-bookworm`，已有锁定 npm 依赖及 Go/Node。脚本不自动下载安装 Docker/镜像，不读取真实凭证。首次或源码更新时构建 UI；只准备可加 `-PrepareOnly`，已构建可加 `-SkipBuild`。只能运行一个预览服务，已有 5210 服务时不要再开第二个。

- 服务：仅 `127.0.0.1:5210`，固定 Host 与同源写校验。
- 专用 PostgreSQL：`agenova-final-local-pg-20261023`，标签 `agenova.owner=final-local-20261023`，仅 `127.0.0.1:55439`，**trust 认证，只能放合成数据**。
- 数据库运行在本地容器可写层，重启保留；删除该专用容器会丢弃这些测试记录。
- 没有创建 volume、云资源或改既有 kind 集群；不使用真实 OAuth 或模型 key。
- Ctrl+C 停服务；验收后可仅停止上面具名本地容器。删除前先确认标签，不清理其他容器、磁盘或共享集群。

测试：

```powershell
npm --prefix ui run check
$env:AGENOVA_TEST_POSTGRES='postgres://postgres@127.0.0.1:55439/agenova_local?sslmode=disable'
go test ./internal/authentication/postgres ./internal/console ./internal/management -count=1
node ui/node_modules/@playwright/test/cli.js test --config ui/playwright.product.config.ts
```

输出在 `.tmp/product-acceptance/` 和 `.tmp/ui-smoke/`；页面截图包含合成数据标识。PostgreSQL 单测使用独立临时 schema，不改验收工作区记录。

</details>

<details>
<summary>已执行的检查与上线前门禁</summary>

- UI 类型检查、canonical contract 检查与生产构建通过；154 条 Vitest、77 条 Playwright smoke 通过。
- 实际 Go API + PostgreSQL 的 5 条浏览器旅程通过：旧两任务范围／Gateway 拒绝／对比，独立项目修改／同请求 Policy 更新与回滚，匿名及 viewer 边界，Researcher 目录／撤权／伪造申请拒绝，同人同模板两个真实 MCP 路径。组合筛选、移动端与实际截图已检查。
- DB 单测证明并发 Policy 激活仅一个成功、一个冲突；跨项目与停用成员拒绝；结构化激活历史不会通过斜线项目名泄漏。真实 API 重试恢复证明同标识只有一次 Provider 调用。
- Windows `scripts/check.ps1 -All` 的文档、架构、格式、依赖整理及 vet 通过；全量 Go 检查遇到原有 `TestOwnerBootstrapDeliveryGuards` 的目录 `Sync` 平台失败。未修改该敏感文件写入逻辑以绕过；同一测试的 Linux 二进制在离线本地容器通过。全量 Windows Go 回归单独显式排除此项后，其余包全部通过（包含实际 PostgreSQL）。原始 Windows 一键门禁不能称为全绿。
- 曾有一轮既有登录会话容量测试瞬时失败；隔离重复 3 次通过，后续完整 SQL/Go 回归通过。不会把这一轮失败当作从未发生。
- 真实 Go MCP 的旧 read_file、crm_read 和 web_fetch 已本地验证，两个固定公共资料源实际获取通过；真实 Google、真实模型、AWS 网络隔离和当前共享环境未在此本地产品验收中重新证明。
- 本地服务不进入生产镜像。生产更新需 owner 批准、PR/CI、实际登录和黄金链路复验；不直接把合成验收当发布批准。

### 数据库与回滚特别要求

新增 migration 010（组织显示元数据）和 011（结构化 Policy 激活审计）；没有改写旧 migration。上线前必须由受控 operator 执行迁移，再给已有 runtime 角色最小新表权限：

| 对象 | 新增 runtime 权限 |
|---|---|
| `agenova_organization_metadata` | SELECT / INSERT / UPDATE |
| `agenova_policy_activations` | SELECT / INSERT |
| `agenova_policy_activations_event_id_seq` | USAGE |

不授予 DELETE、DDL、数据库管理员或迁移凭证给 runtime；现有项目隔离仍由可信服务授权。Schema 011 会被旧二进制拒绝，因此需要单 writer 协调切换和 **兼容 schema 011 的回滚镜像**，不能只回滚到旧镜像或删除审计表／migration 记录。迁移前备份与真实 runtime grant 验证属于上线门禁；本地 superuser 测试不能证明生产 SQL grants 已安装。010/011 之前的激活事件不凭空补造。

</details>
