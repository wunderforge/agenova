# 中期演示：从提交任务到真实结果

这个 checkpoint 把已经完成的模块串成一条用户可操作的链路：

**提交任务 → 检查身份和权限 → 创建 claim → kind worker 执行 → Model Gateway 调用真实模型 → 查看结果和证据 → 释放环境。**

## 现在可以看什么

- **Work**：创建任务，查看运行进度、真实回答和模型用量。
- **Access**：比较请求与最终权限；例如 45 分钟请求被 template 上限收窄为 30 分钟。
- **Activity**：查看同一个请求、claim、worker、调用关联起来的执行证据。
- **Platform**：区分已配置、实际使用过、尚未连接；不是另一套厂商监控控制台。
- **Identity / Policy / Agents**：查看服务端固定身份、组织策略和可用 template，不在 UI 中编辑或自行切换身份。

Demo 使用示例数据；Connected 只使用本地服务的真实数据。连接失败不会切回示例。

## 本地启动

前提：Docker Desktop、已有 `kind-agenova-k8s-lab`（Agent Sandbox CRD/controller 已安装）、本机 Ollama 与已有 `llama3.1:latest`。没有模型时先配置合适的本地模型，不自动下载。

在仓库根目录构建并加载测试 worker：

```powershell
docker build -t agenova-testworker:kind -f harness/integration/agentsandbox/testworker/Dockerfile .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
```

选一个新的、属于本次演示的 namespace，再启动固定 Team A 身份的本地服务：

```powershell
kubectl --context kind-agenova-k8s-lab create namespace agenova-demo-local
go run ./cmd/agenova-console -kube-context kind-agenova-k8s-lab -namespace agenova-demo-local -principal team-a
```

另一个终端启动 UI，打开输出地址并切到 Connected：

```powershell
npm --prefix ui ci
npm --prefix ui run dev -- --port 5175 --strictPort
```

在 New work 输入一个公开/合成问题，例如 “Explain in one sentence why retries need a time limit.”。当前极小 agent 会通过 Model Gateway 请求回答；它还不是修改 Git 仓库的 coding agent。

服务仅监听本机，使用 operator 固定的可信身份。不是上线后的登录或多用户部署方案。要演示拒绝场景，可以在独立端口/namespace 启动 `-principal team-b`；浏览器不能通过提交字段更改身份。

## 直接查看 evidence API

从 Work 页面复制 request reference 或展开的 claim ID，替换下列占位值：

```powershell
curl.exe --noproxy "*" http://127.0.0.1:8088/api/requests/REQUEST_REFERENCE/evidence
curl.exe --noproxy "*" http://127.0.0.1:8088/api/claims/CLAIM_ID/evidence
```

两种查询返回相同的 `agenova.evidence/v0` View：`requestRef`、原始 `request`、可信 `state`、有序 `facts`、实际 `outcome`。成功的 `outcome` 包含 `text` 和模型调用/用量；运行中尚无最终 outcome；拒绝可以按 request 查询，但没有 claim。未知引用返回 404，非法引用返回 400。

可复验的 HTTP 用例使用真实 handler，不依赖浏览器 mock：

```powershell
go test -count=1 -v ./internal/console -run TestHTTP
```

## 可复验的 gate

```powershell
./scripts/check.ps1 -All
node harness/e2e/ui-kind-checkpoint.mjs --live-model --base-url http://127.0.0.1:5175
```

后一个命令会真正提交任务并调用本地模型；成功后保存结果、权限和 Platform 截图及 evidence 到 `.tmp/ui-kind-checkpoint/`。默认 CI 不运行真实推理。

## 还没接上的部分

- Tool 和 Memory 接口尚未连接；不能把界面的 granted 权限当成已经调用过这些服务。
- 当前记录只保存在服务进程内，不是持久历史。
- 还没有真实 vendor coding agent、代码修改/PR、SSO 或生产环境防绕过保证。
- 当前模型走本机 Ollama，无 API 费用；模型 endpoint/provider 细节留在 adapter，公共接口保持通用。
