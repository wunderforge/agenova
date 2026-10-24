# Draft: Record structured rejection evidence for installed worker operations

建议标签：`source:e15-security`, `area:authority`, `area:gateway`。未发布。

Parent: #177；发现于 #198；矩阵 AC-1、AC-5、ES-1。

在隔离 kind 参考安装上，worker 发送其他 claim ID、未授权 profile、工具或 scope 时，调用被阻止；每个场景的 Ollama 增量仅为预设的一次合法对照调用，目标 claim facts 没有新增。CLI 最终只显示 `execution-failed`，没有说明被拒的操作、原因类别或拒绝所在层。

这是拒绝证据缺口；本次没有观察到越权放行。工具/scope 场景可能先触发模型动作对应校验，不能宣称已验证 Gateway 的资源策略。请由治理/证据负责 Epic 补充可查询、归属当前 worker claim 的拒绝证据；不要把事实写到被冒用的 claim 下。

验收建议：跨 claim、profile、tool、scope、格式错误和不支持操作分别有明确安全的原因码；CLI/API 能区分传输层拒绝、Gateway Deny 和运行失败；Provider 无额外调用，目标 claim facts 不变。E15 在修复后复测。

另需在 #198 说明：当前串行控制面无法提供同一服务下两个 Running claim；两实例测试只作补充。终态调用及模型注入到 Gateway 的验收仍未完成，不能关闭 #198。

## Supplementary finding — terminal rejection records

补充公开接口测试确认：Succeeded、Failed、Expired 后各发 model/tool 请求（使用新 context），六次均被拒；Provider 无额外调用，Work facts 完全不变。缺少的是后续拒绝的记录。对应 AC-4、ES-1；这是内存运行时的代码级证据，不是已安装 worker 通道证据。测试输出见 boundary-probe.log。

并发运行属于另一个前置能力缺口，不应混成安全放行问题；请按 blocker-analysis.md 单独指派负责团队。
