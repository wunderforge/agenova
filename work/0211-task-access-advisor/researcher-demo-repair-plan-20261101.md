# Researcher Demo 修复方案：快速回答与逐轮可解释记录

日期：2026-11-01，Australia/Sydney。状态：**本地实现与开发者验收完成，待用户验收**。

本方案覆盖用户最新验收发现，补充此前的本地实现计划；旧测试通过记录保留为历史证据，不代表本方案已通过。用户最新指令授权本轮本地实现、测试、提交并推送当前 feature branch；不创建 PR、不合并、不部署、不操作云资源。

## 1. 用户期待的最终效果

用户只输入一个简短问题，不需要网址、Task type 或 preset。Agent 按当前 Work 权限发现和读取真实公共信息，尽快给出有依据的答案；能结束时直接结束，不进行无意义复核或反复生成 finish。

Agent activity 按真实顺序展示每轮：

```text
Model request → Model response / Thinking（确有返回时）
              → Proposed tool + arguments / final answer
              → Agent action accepted or rejected（附实际原因）
              → Gateway Allow / Deny
              → Tool result / specific error
              → Next action or final result
```

用户能回答：模型返回了什么、为什么选择这个动作（模型确有说明时）、请求了什么、平台允许了什么、工具实际返回什么、为什么失败、为什么重试、最后是否完成。

不将模型自己解释的理由当成可验证的内部思维过程；没有返回／没有采集的 thinking 明确缺失，不事后补造。不要求每轮输出冗长思维链，更不额外调用另一个模型编写解释。

## 2. 已核验问题与基线

本地服务 5225，Researcher v3，真实本地 Llama 3.1；身份、CRM 数据和运行网络生命周期仍为本地合成环境。未重跑任务、未调用云端来得出以下结论。

| 案例 | Work 后缀 | 记录与缺陷 |
| --- | --- | --- |
| Melbourne 今日天气 | c02a24fd… | 42.00s；2 次模型请求共 41.63s，网页调用 0.28s。最终只解释 JS 提示却报告 completed。 |
| 2027 Spring Festival | a1440f44… | 48.75s；3 次模型请求共 48.65s。MCP 返回 public-url-refused，但页面和 Agent 只得到笼统工具失败。 |
| Python 官方网站问题 | 8c087739… | 81.75s；模型请求 21.48/26.27/33.60s，网页调用 0.33s。第二轮提出 finish 后仍进入第三轮，具体退回原因没有记录。 |
| web 不授予后误用 CRM | 5aac495a… | 57.48s；4 次模型请求。web.read Deny 后 crm.read Deny，均无后台执行；权限正确，动作相关性和解释不足。 |
| AI infrastructure 问题 | 19470fbd… | 210.84s、10 次模型请求；两次 invalid-or-large-response，后续一次读取成功。存在格式／质量重试，但完整响应和退回原因缺失。 |
| 早期成功 Python 案例 | 1c3fa883… | 早期读取 fetch-failed，后来一次工具成功，最终完成；成功 Work 中的早期失败没有清晰恢复说明。 |

原因分层：

- `web.read` 仅按 URL 获取静态正文，不是搜索，不执行 JS；当前正文抽取粗糙、截断过早。
- Agent 将几乎所有问题绑到工具证据，相关性保护部分依赖少数英文短语；看见工具不等于应该调用该工具。
- finish 在协议层被接受后，Agent 内部还能因引用／证据检查退回，但这一层没有显式事件。
- MCP `isError` 的具体内容被适配器压成 ErrProvider；控制面再变成 tool-provider-failed。Agent 也失去可操作原因。
- 当前只持久化动作形状和通用说明，逐轮模型响应、工具参数／观察正文和部分判断原因未保留。
- 模型调用本身耗时显著；当前没有加载、排队、输入处理、生成的分项指标。不能断言全部来自冷启动或显卡。

原始运行快照和 MCP 对照位于忽略的 `.tmp/user-run-diagnosis-20261101/` 及本地 MCP receipt；不将原始诊断材料直接提交 Git。

## 3. 责任边界与架构

| 层 | 本轮责任 | 不承担的责任 |
| --- | --- | --- |
| 公共 MCP 工具 | 搜索、读取、明确结果与具体错误 | 不解释／授予 Work 权限，不替 Agent 编造答案 |
| 参考 Researcher image | 相关动作、结果判断、有限恢复、及时结束、可选简短决策说明 | 不修改 Policy，不成为所有 Agent 的强制工作流 |
| Model provider adapter | 保留实际 message/tool-call/可提供的 thinking 和性能元数据 | 不假造隐藏推理，不把模型自述当平台判定 |
| Gateway / 控制面 | 每次独立鉴权、配额和关联证据，转交结构化结果 | 不用天气／春节等关键词充当平台业务裁判 |
| Activity UI | 忠实展示每轮内容、允许／拒绝、错误与恢复、时间成本 | 不从 HTTP 200 推导任务完成，不从没有尝试推导被拦截 |

保留任意兼容 image、可选 Advisor、手动／上游指定权限、旧 API/CLI/历史记录。权限表 requested vs granted 保持当前布局和语义。

### 3.1 逐轮内容与证据

- 以 Work/turn/model invocation/tool invocation 建立关联，保留提供方真实返回的 tool-call ID；模型调用和工具调用保持不同 ID，通过关联字段连接。
- 模型响应记录：公开 content、结构化 action/tool arguments、finish reason、模型身份和 token。提供方确实返回且允许导出的 thinking/reasoning 单独记录，标明来源；缺失即缺失。
- 可为参考 Agent 增加可选、短句的 `decisionSummary`，表达下一步目的；与 completion reason、Gateway reason 分开，不增加专门解释模型调用，不要求生成完整思维链。
- 将 Agent 内部 `action accepted / action rejected` 与拒绝理由记录下来。协议格式合法不等于该候选答案已经被 Agent 接受。
- 工具记录：搜索 query／URL、结果条目或正文、数据时间、截断情况、具体错误。每轮候选 finish 的公开答案也可查看，避免只剩最终答案。
- 结构化元数据进入现有 facts；较大正文通过内容存储接口和不可猜测的逻辑引用关联，不把完整正文塞入每个列表响应或大幅膨胀 journal。
- 本地验收使用有界存储，容量／保留期／截断明确；生产复用或适配现有受控 artifact 存储，持久性另测，不新建数据库 Pod，不默认上传正文到 CloudWatch 等外部平台。
- 内容采集由安装配置显式开启，沿用 Work/project 读取权限，移除凭证、授权头和敏感查询参数；提供方不支持的字段不补造。对任意 image 只展示实际观测到的事件。
- 本轮后续本地验收配置会开启上述内容采集，使用户无需另找开关即可检查自己的测试记录；这不等于默认为生产安装开启全部正文外发。
- 可选协议字段／诊断事件需要版本或协商；不能给旧 Agent 的严格 JSON 解码器强塞字段。

### 3.2 公共工具补齐：找来源与读来源分开

- 新增真实 `web.search(query, limit)`：返回有序 title/URL/snippet、查询时间、实际 provider、是否缓存；默认 3–5 条，不把搜索摘要伪装成完整网页。
- 保留 `web.read(url)`；可读静态 HTML、纯文本及有界 JSON 公共数据，采用正文提取，保留标题、实际 URL、获取／数据时间和截断标记。
- JS 空壳、验证码、登录页、无正文与可用内容区分；不能把 HTTP 200 自动当作获得了答案。当前阶段不为了这一点部署通用浏览器集群。
- 增加可配置原始响应／正文大小预算，分别报告“原始响应过大”和“正文有界截断”；不通过无限扩大上下文解决问题。
- 正常公开重定向可有限支持，每一跳重新验证 URL、DNS/IP 和目标；私网、元数据地址、凭证 URL 和重定向循环仍拒绝。不能为提高成功率放开 SSRF 边界。
- 搜索放在现有 MCP service 内的可替换 provider adapter；本地先验证无 Key 的公开检索候选，不增加必需付费账号、另一 Pod 或云资源。
- 第一候选为 DuckDuckGo 官方 HTML/Lite 搜索界面的薄适配层；它是网页接口，不是具有 SLA 的正式搜索 API。必须先测结果质量、限流／验证码和可用性，再决定是否采用；不绕验证码，不把 Instant Answer 当通用搜索。
- 若候选无法通过真实检索门槛，明确报告 blocker 和备选方案，再决定 provider；不能靠猜 URL、固定答案、伪造搜索结果或用户事先填网址交付。
- 天气若遇 JS 首页，Agent 应另找可访问的预报／公共数据来源，或明确不足；Spring Festival 区分农历日期与另行发布的放假安排。测试不得将固定答案写进平台。

`web.search` 是新增能力，必须有注册、Template ceiling、Policy、Advisor guide 和 Work 请求支持。新建本地不可变 Template 版本验证，保留 v1/v2/v3 与旧 Work。只有 web.read 的旧 Work 不自动得到 search 权限。

### 3.3 错误从来源一直传到 Agent 与 UI

通用错误模型包含：`category / code / stage / safeMessage / retryable / source / invocationRef / rootCauseKnown`，以及适用时的 HTTP status、URL 校验规则、字节上限和实际大小。记录最具体的已知原因，不声称一定能知道远端服务器内部根因。

| 类别 | 要保留的区分 |
| --- | --- |
| Authority | tool-not-granted、scope-not-granted、过期／错误身份；后台执行数为 0 |
| Input | 非 HTTPS、非法／空 URL、禁止地址、片段／端口等具体规则 |
| Network | DNS、连接、TLS、调用超时；Work 取消与调用超时分开 |
| Remote response | HTTP status、限流、验证码、unsupported-content、redirect-refused／loop |
| Read / parse | response-too-large、body-read-failed、invalid-encoding、parse-failed；拆开原来的合并码 |
| Evidence | JS shell、empty/no-usable-content、truncated；不与权限拒绝混用 |
| Agent / model | schema-invalid、citation-missing、unsupported-finish、model-budget-exhausted；说明是否会再请求模型 |

MCP 使用结构化错误详情，保留 `isError`；适配器映射已知安全码，而不是丢掉内容转成单个 ErrProvider。未知／未结构化错误保留 unknown 和阶段，不把任意服务器文本当可信 code，也不原样透传潜在秘密。

Gateway 仍执行真实判定；被拒调用不进入 MCP，也不从缓存返回数据。相同的安全诊断必须同时提供给 Agent 和 UI。界面不再用 “no fallback was used” 代替原因。

### 3.4 Agent 循环：合理地尽早结束

- 在 Agent 上下文中提供可信的当前日期／时区和工具实际语义，不再靠用户额外填类型。
- 一般知识题不强制先读工具；显式要求当前／网页／官方证据的问题，必须使用实际相关信息，未获得则 partial/incomplete。此为参考 Agent 行为，不是平台对所有 image 的强制语义规则。
- 已获得足够证据时直接输出最终答案；接收并通过最终校验后终止，不能无条件多调用一轮模型做复述或引用修正。
- 原始来源、输入和结果以同一 Work／invocation 的工具记录关联保留；不因自然语言答案没有逐字附上 URL 而多跑一轮或将正确答案判为失败，也不把不同站点视为同一来源。
- 被内部质量校验退回时，同轮记录具体理由与必要修正，再决定是否需要重试；同类格式／质量修正至多一次，不重复执行成功工具调用。
- 短暂网络错误允许有限重试；永久 URL／权限错误不原样重试。只能改用与目标相关且获准的来源，不因为 web 被拒就读 CRM。
- 工具对模型可见和 Work 可执行仍分离。发生不合理请求时保留实际模型输出与 Gateway 拒绝，不能隐藏工具来制造“模型没犯错”。
- 工具失败可以恢复后完成；结论须与目标相关的真实观测一致，包括用于报告可达性失败的受控错误。无实际天气读取不能报告 completed。平台执行状态与 Agent 自报目标完成状态继续分开。
- 正常公共研究通常 2–3 次执行模型请求（搜索、必要读取、回答）；已知来源或 CRM 通常 2 次。搜索结果已足够时不强制再读；确需更多证据可增加，但每轮应有明确进展和记录。
- 保留可配置 maxModelCalls 和 image 安全上限；它们是资源上限，不是要求用满的轮数。穷尽后给出真实 incomplete，不制造答案。

## 4. Agent activity 展示设计

沿用 June 风格和现有权限区域。每轮左侧概览、右侧检查器：默认显示动作／工具、状态、耗时和短摘要；展开才看正文。

1. **Model response**：实际返回内容；若有，单列 `Thinking · model-provided`；可选 `Decision summary · model-reported`。没有 thinking 不留大块空白或假文字。
2. **Agent action**：所选工具和参数，或候选最终答案；明确 Accepted / Rejected。Rejected 必须给出导致下一轮的理由。
3. **Gateway**：独立 Allow / Deny、Work 实际授权及原因，链接请求／授予对照。
4. **Tool result**：实际结果、出处和时间；失败显示具体类别及原因，技术详情可展开。
5. **Next step**：实际恢复／重试／结束事件。某次调用失败后后续成功，则概览为“完成，含已恢复的调用失败”，保留所有早期失败，不能整条标成从未失败。

模型解释与平台事实使用来源标签，不用多套装饰颜色。保留已确认的简洁权限配色；真实拒绝继续有顶部简短 warning。不要把内容采集开关或缺字段塞成大量产品解释文案。

## 5. 性能目标与测量

目标是“用户感觉快”，不只是请求真实。以下为本轮验收目标；当前测量和准确边界见[本地验收记录](repair-local-acceptance-20261101.md)：

- 提交后 1 秒内出现真实阶段状态；不伪造百分比、token 或 thinking。
- 热模型下 10 次混合 Demo 任务，至少覆盖下面三个公共问题、内部 CRM 和缺权限：端到端 P50 ≤15s、P95 ≤25s，公开报告样本与每次结果；不是生产 SLA。
- 端到端从点击开始计，包括 Automatic Advisor、队列、Worker、模型、工具和最终展示；分别展示各段，不能把 Advisor 从总耗时中剔除。
- Python 正例不应再次出现无说明的第三次模型调用；普通正例以 2–3 次为主，超过时必须能解释新增证据需求。
- 冷启动／模型切换单独报告，不与热模型数据混算；缓存命中也单独标记，不用缓存固定答案冒充实时检索。

先基准测量既有本地模型与硬件，记录加载、排队、输入处理、输出生成、token/s、上下文大小和逐轮 token；当前 20–34s/轮已证明，仅减少一轮不足以保证达标。

优先在已安装模型中选质量与速度均通过的组合；Advisor 与执行模型依然独立配置，可选择同模型以减少切换，但不建立新的发布耦合。不自动下载模型、购买 API、改变全局 Ollama 配置或启动云端。若现有硬件／模型达不到门槛，给出明确实测瓶颈与需选择的方案，不能降低标准后声称通过。

压缩提示和观察为相关证据，保留引用与必要原文入口，不让多轮重复堆积上下文。流式显示可后续改善等待感，但不当作总时延优化；未完成、未验证的流式 JSON 不得触发工具执行。

## 6. 开发顺序与出口门禁（已实施，开发方最终验收中）

| 顺序 | 交付 | 出口门禁 |
| --- | --- | --- |
| S0 | 保存现有 5225 记录；建立复现语料和时间拆分；最小无 Key 搜索可行性／现有模型基准 | 不覆盖旧进程／记录；确认真实搜索而非固定来源；明确模型时延下限，失败即记录 provider／硬件阻碍 |
| S1 | 通用错误分类＋MCP→adapter→Agent/UI 传递；逐轮内容／校验事件与关联存储 | 同一个 invocation 的已知原因前后不丢；实际模型输出可查；无敏感值；旧协议和历史缺字段可读 |
| S2 | web.search provider 插拔＋web.read 提取/公开 JSON/重定向/正文预算；新本地 Template 和 guide | 短问题不含网址也能找到真实来源；新能力独立授权；JS/无用正文不冒充答案；不加新云资源 |
| S3 | 参考 Agent 提示／格式、相关性、证据引用、结束和有限恢复 | 两任务正常完成；否定例不乱切 CRM；有效 finish 不被无理由退回；失败后能恢复且过程完整 |
| S4 | Activity 内容检查器、具体失败／恢复说明、逐轮时间 | 每轮 Model→Action→Gateway→Tool→Next 可追；保持权限布局；桌面/移动端、键盘/焦点、字体按钮视觉检查 |
| S5 | 端到端质量和性能调优、完整回归、本地用户验收入口 | 满足语料、权限、性能和兼容门槛；失败就报告，不部署；再按后续授权提交推送 feature branch |

S1 优先保证后续调试不再盲目；S0 的轻量可行性与速度基准应在投入完整工具／UI 开发前暴露硬阻碍。用户后续指令已授权本地实施、测试和 feature branch 备份；本轮仍不创建 PR、不合并、不部署。

## 7. 联合验收矩阵

| 场景 | 输入／条件 | 必须观察到 |
| --- | --- | --- |
| 今日天气 | “What's the weather in Melbourne today?”；不提供 URL | 真实检索／数据读取；城市、适用日期/时间、温度/天气、来源；不是 JS 提示；不能答到其他城市或旧日期 |
| 2027 春节 | “When is Spring Festival 2027?” | 有可核对的日历来源和具体日期；区分节日当天与放假安排，不要求用户输入 URL |
| Python | 原官方网页问题 | 真实官方资料、简短用途回答和来源；无多余 finish 复写 |
| 内部客户 | 三个使用量下降客户及跟进建议 | 真实 CRM 调用，synthetic 明示、客户 ID 和正确字段语义；不为完成它乱搜网页 |
| web 能力全部撤销 | 同一公共任务，search/read 均不授予，但 Template 描述仍可见 | 真正尝试被 Gateway 拒绝且后台 0 次；不乱用 CRM；不能伪造拦截或让 Pod 绕公网 |
| 只撤销 search | read 仍授予 | search 不执行；Agent 若知道合适 URL，获准 read 可合法执行；不能把撤销搜索说成全面禁止公共读取 |
| 恢复成功 | 已知首次超时/503，再换有效相关来源 | 早期错误原因、实际后续调用、恢复关联、最终有证据；不丢早期 Failed |
| URL/编码/大小/内容 | 非 HTTPS、片段、私网/重定向、无效编码、超限、JS shell | 精确到实际失败步骤；已知原因不压平，未知原因不猜；永久错误不无限重试 |
| 模型/循环 | 非法格式、缺引用、同 URL 斜杠差异、第一次候选 finish、配额耗尽 | 每次退回理由可见；引用合理关联；不无谓重生成；配额与结论真实 |
| thinking/content | 有 thinking 的 provider fixture、无 thinking 的真实模型、截断／关闭采集 | 原样来源可辨；无字段不补造；不为解释增加模型调用；旧记录不伪造历史 |
| 保密与权限 | 外部输入含授权头／敏感值、跨 Work/project 内容引用、缓存命中 | 不泄露／跨范围读取；缓存之前仍鉴权；Agent 自述不能改变权限 |
| 兼容 | Manual/Suggest/Automatic、上游显式权限、旧 image/API/CLI、copy/retry | 行为保留；新增 search 不自动授权旧 Work；当前 requested/granted 规则不变 |

使用确定性 fixtures 验证错误和协议边界，使用真实本地模型与真实公共来源验收答案／性能，两者不混算。天气的动态答案以当时来源为准；日历固定答案只进入测试参考，不进入运行逻辑。所有结果保留成功率、调用次数、总时延和失败样本；不挑一条成功截图代替通过。

先跑相关 Go/前端/协议/适配器测试，再跑完整仓库检查和浏览器回归；既有 Windows Owner-bootstrap 问题若仍存在，单独说明，不能宣称全套 green。部署／K8s packet enforcement 不在本轮本地验收内。

## 8. 当前 TODO 与未决点

- [x] 真实搜索候选的本地可用性与限制验证。
- [x] 已安装模型的时延／质量基准及 Demo 模型选择。
- [x] 精确错误模型、MCP 传递及已知原因拆分。
- [x] 逐轮模型内容、可提供的 thinking、工具输入／结果及 Agent 校验反馈。
- [x] 真实搜索、可用正文／公开数据读取与独立授权接入。
- [x] finish／引用／有限重试／CRM 相关性和 completion 修复。
- [x] Activity 可读性、恢复展示、逐段耗时及全部受影响 UI 视觉检查。
- [x] 最终联合验收：10 次真实模型混合案例全部通过，P50 13.95 秒、P95 17.31 秒；额外网页 404、墨尔本天气与浏览器点击验收通过。185 个前端单测、95 个通用浏览器案例和 4 个当前候选版浏览器案例通过。
- [ ] 用户本地 walkthrough／验收。完整 Go 检查仅原有 Windows Owner-bootstrap 测试仍失败，不能声称全套 green。

本轮代码与验收记录交付到 `codex/task-access-advisor`，备份以本地和远端 branch 的相同提交为准；不创建 PR、不合并、不部署。

实测选择：可替换的 Bing RSS 搜索、显式启用的有界内存内容存储、本机已有 Qwen3 4B Instruct 2507。Ollama 7B/8B 的迭代失败与耗时仍保留在忽略的诊断目录，不能当作本组合的验收结果。不得因为无 Key 候选不稳定而偷偷启用付费 provider，或放开 Agent Pod 的公网出口。

## 9. 方法依据

- [OpenTelemetry GenAI 内容采集](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-spans.md#capturing-instructions-inputs-and-outputs)：内容显式启用，较大内容可独立存储并记录引用。该约定仍在 Development；本方案借鉴结构，不宣称已通过标准兼容认证。
- [Ollama usage](https://docs.ollama.com/api/usage)：区分加载、输入计算与输出生成时间，用于替代“模型可能慢”的猜测。
- [DuckDuckGo 非 JS 搜索](https://duckduckgo.com/duckduckgo-help-pages/features/non-javascript)：官方存在 HTML/Lite 搜索页面；不意味着提供自动化 API 的稳定性保证，所以候选必须先最小验证。
- 当前仓库职责边界与真实运行记录：`cmd/agenova-demo-worker`、`internal/modelprovider`、`internal/workerprotocol`、`internal/console/tool_provider.go`、`internal/adapters/bundled/mcp_client.go`、`harness/integration/mcpfixture`、`ui/src/WorkerActivity.tsx`。


## 10. 实施中的明确决定

- 新 `research-discovery-v3` 显式启用补充反馈／内容；旧 v1/v2 schema 和普通
  image 的执行契约保留。工具可见性来自 Template，Gateway 每次按 Work 检查。
- 当前天气的参考 Agent 指引使用公开 JSON，日历／当前事实及明确要求网页的
  问题先取相关来源；这些是 image 内的研究策略，不是平台按关键词分配权限。
- 普通知识／计算无需无关工具；真实工具拒绝、源站失败、格式／质量退回保持
  分层。合成客户来源必须明确标记；后续建议是建议，不宣称已经执行。
- 内容默认关闭；本地自愿采集的数据不进 fact 列表正文，用当前 Work 读取权限
  获取随机引用。字段／容量／时限均有界；旧记录不补造 body/thinking。
- 依赖仅在独立 MCP module 更新至兼容 Go 1.25 的 x/net v0.57.0，不安装新 Go
  或模型；较大 Ollama 仍可配置，Advisor 与执行的 loopback endpoint 独立。
- 缺少可观察 queue/cache 命中字段时不伪造数据。基准使用热模型及 llama.cpp
  正常前缀 KV 缓存，没有固定答案缓存；冷模型启动另记，不冒充热模型时延。
