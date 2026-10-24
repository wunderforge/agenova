# #198 comment draft — 未发布

第一轮 kind 实测和补充代码测试已完成，当前不能关闭 #198。

- 正常对照成功；冒用 claim、profile/tool/scope 及错误协议操作未产生额外 Provider 调用。两实例测试中 B 保持 Running，facts 不变。
- 同一服务的两条 Running claim：控制面 executeMu 和 RunService runMu 都串行执行；新增测试在有两个空闲 worker 的情况下仍确认 A Running 时 B Pending。换提交入口或装第二套环境不能满足这条验收。
- 模型注入到 Gateway Deny：参考 worker 的 profile/scope 由系统固定，伪造值会被 exec 协议提前拦截。公开 Executor 测试接口下能得到含 policy/reason 的真实 Gateway Deny，但这不算安装路径的模型注入证据。
- 终态调用：补充测试在确认 Succeeded/Failed/Expired 后，用未取消的新 context 分别发送 model/tool 操作，全部被拒、零额外调用；但没有新增拒绝记录。安装路径的会话已结束，尚无支持的入口验证完整过程。

E15 按 #177 约定只报告、不修复。请维护者确认：并发执行前置能力和拒绝记录分别交给哪个 Epic，以及模型注入这条采用什么受支持的安装路径复测。原验收保持不变，第二套安装和代码测试仅作补充。

证据、源码位置和运行输出已整理在本地 docs/evidence/198/，待 PR 发布后补链接；没有观察到越权放行。已核对开放 PR #217，它处理 Gateway 内部的早期拒绝通知，不覆盖 exec 协议前置拒绝、终态 guard 或并发执行；建议与 #180 协调记录归属，避免重复工作。

辅助证据已补齐：拒绝提交期间连续监测零分配事件（正常提交可捕捉创建事件）；canary 拒绝后在指定对象、环境及当前日志未残留（扫描阳性对照通过）；真实成功/失败/拒绝 Work 的 CLI/API/UI 一致且截图已保存。Policy 约束当前 schema 不支持，CLI 拒绝 constraints 且活动 Policy 不变；该能力已有 E14 #204 跟踪。模板收窄已验证，不能当成 Policy 收窄。
