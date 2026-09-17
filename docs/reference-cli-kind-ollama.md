# 用 CLI 复刻 kind + Ollama 演示（#165）

这份手册只覆盖已验证的参考链路，不需要启动 React 或独立的 `agenova-console`。

## 先准备

- Docker Desktop、kind、kubectl、Go 1.22+ 和 Ollama 可用。
- 现有 kind 集群名为 `agenova-k8s-lab`，上下文为 `kind-agenova-k8s-lab`；已安装 Agent Sandbox v0.4.6。
- 本机 Ollama 已有 `llama3.1:latest`，并可从 kind 容器通过 `host.docker.internal:11434` 访问。
- 已把 `agenova-testworker:kind` 与本分支构建的 `agenova-control-plane:0.1.0` 两个镜像加载进该 kind 集群。镜像构建/加载属于底层测试环境准备，不是以下 Agenova 控制流的隐藏步骤。新环境可在仓库根目录执行：

```powershell
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-testworker:kind .
docker build -f deploy/reference/Dockerfile -t agenova-control-plane:0.1.0 .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
kind load docker-image agenova-control-plane:0.1.0 --name agenova-k8s-lab
```

从空集群安装 Agent Sandbox 的步骤仍属底层 substrate 准备，见 [固定 v0.4.6 的 runbook](../harness/spike/agent-sandbox-substrate/RUNBOOK.md)；本流程不把 kind/上游控制器伪装成 `platform apply` 所安装的能力。

在仓库根目录构建 CLI，并让本次 PowerShell 会话找到它：

```powershell
New-Item -ItemType Directory -Force .tmp | Out-Null
go build -o .tmp/agenova.exe ./cmd/agenova
$env:PATH = "$(Resolve-Path .tmp);$env:PATH"
```

## 七条命令

在仓库根目录依次执行（`platform apply` 首次会询问，输入 `y`）：

```powershell
agenova platform validate -f deploy/reference/platform.kind.yaml
agenova platform plan -f deploy/reference/platform.kind.yaml
agenova platform apply -f deploy/reference/platform.kind.yaml
agenova platform status
agenova policy apply -f deploy/reference/demo/policy.yaml
agenova agent-template apply -f deploy/reference/demo/engineer.yaml
agenova run -f deploy/reference/demo/work.yaml
```

最后一条会等到 Work 结束，再显示请求、决策、worker claim、终态、回答和模型 token 数。成功时会看到 `decision: Allow`、`phase: Succeeded`。不要把 `platform status` 的安装就绪当作 Ollama 或一次任务已经成功；第七条才验证真实执行。

想看完整 JSON，把 `--json` 加到 `run` 命令；同一份 evidence 包含 `RequestResolution`、`AuthorityResolved`、`Runtime`、`ModelDecision`、`ProviderOutcome`、`ToolDecision`、`RunOutcome`。当前参考服务把 Work/evidence 存在进程内，重启 Pod 会失去这批历史；Policy 和 AgentTemplate 注册在 Kubernetes ConfigMap，重启后仍在。

## 治理测试

```powershell
agenova policy apply -f deploy/reference/demo/policy.yaml
agenova agent-template apply -f deploy/reference/demo/engineer.yaml
agenova run -f deploy/reference/demo/denied-work.yaml --json
agenova policy apply -f deploy/reference/demo/policy-conflict.yaml
```

前两条应报告 `already registered`。第三条应以 `Deny` 结束，且没有 Claim、worker、模型或工具调用。第四条故意提交同 ID/版本的不同策略，应报冲突，不能替换已生效策略。测试文件的 request 名称固定；重复运行同一 Work 前应换一个新的 `metadata.name`，或重启参考服务。不要用重启作为正式产品的“清空历史”方法。

## 当前边界

- worker 是真实 Agent Sandbox Pod；模型通过真实 Ollama 调用；示例 `git.read` 内容是明确标记的 mock，不代表已接入真实 Git。
- 可信 Team A 身份由参考服务配置固定提供，不能从 Work YAML 或 CLI 参数伪造。还没有生产身份提供方、用户切换或多租户认证。
- CLI 用当前 Kubernetes 身份/RBAC 注册配置和连接已安装服务；容器内 Work 入口仅监听 loopback。loopback 是传输隔离，不是认证。
- 不要在 Work 运行中执行新的 `platform apply` 或重启 Control Plane；当前参考服务的 Work/evidence 位于单个 Pod 内存中，滚动更新尚无 drain/持久化交接，可能中断执行及清理。这是后续生产升级能力，不属于本次七条命令的完成声明。
- 只验证了这组 Kubernetes/Agent Sandbox/OpenAI-compatible 配置。Memory、Observability、其他 gateway/adapter 和任意 agent 镜像不是此演示的已上线能力。
- 本流程是已验证的参考演示路径，不代表生产身份、多副本历史或通用 adapter 已完成。
