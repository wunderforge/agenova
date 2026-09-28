# E13 AWS / Bedrock 交接

更新：2026-09-28 Sydney。先读 AGENTS → PRD → [当前 E13 任务](../0175-shared-eks-bedrock/task.md)。

## 当前结果

- 分支 `codex/e13-aws-demo-bootstrap`，本地目录 `/Users/neoliu/projects/agenova`。
- AWS账户931228356546，Sydney ap-southeast-2，集群agenova-demo：ACTIVE，单个t3.medium节点Ready。
- Terraform最初23资源，保留期限更新成功，随后Pod Identity/Bedrock服务角色4资源成功创建。State在infra/demo/aws中，已忽略且必须保留至清理。
- Agent Sandbox v0.4.6控制器、Pod Identity agent、Agenova控制面均运行。
- 两个独立Work真实执行成功，经过Agenova Gateway调用Bedrock Nova Micro；未授权项目请求Deny、无claim，调用计数4→4。
- 实际第二个Work的Worker快照：default SA、automount false、无环境变量和凭证挂载；控制面独有Pod Identity。
- Platform重复apply changed:false；Policy/模板重复注册already registered。
- 全量gate已通过（52/52 UI smoke），真实安装CLI/API/Portal E2E 2/2通过。旧键盘测试已修复为跨平台type-ahead，未改产品UI行为。
- 详细原始输出、非密钥配置、截图：[证据](../0175-shared-eks-bedrock/evidence/summary.md)。

## 用户范围与剩余工作

用户已分别批准基础设施包和完整E13包；随后明确“暂时不用管E14，尽量把E13跑完”。因此当前交付为E13云端主链路。仍固定reference Team A，通过本地Kubernetes隧道访问，不对公网开放API。不能宣称两个真实用户、组织权限收窄、完整共享API或关闭E13/#146。

E15真实出网限制/直连绕过验证未完成；不要声称网络隔离已证实。工具仍是synthetic git.read。kind/Ollama真实回归未运行，因为本机没有Ollama可执行程序或服务；现有Go/fixture回归已通过。Work历史仅进程内存，重启控制面会丢失，磁盘证据已保存。

先前演示身份草稿已移出活动代码，仅在/tmp/agenova-e14-draft.patch与/tmp/agenova-e14-shared.go，未经完整验证，不直接套用。未创建新的bootstrap IAM策略；废弃的未验证示例已删除。

## 身份与工具

- CLI profile `agenova-demo`，STS已核实为`arn:aws:iam::931228356546:user/agenova-demo-operator`，不是root。
- 用户自行配置admin组；不要重复创建IAM用户/policy。root MFA已设置，operator登录由用户完成。
- 临时`aws login`，无长期access key。失效时让用户完成浏览器登录；不要读取/打印login cache、密码下载文件或会话中的旧密码。
- TLS使用已有系统可信Zscaler根拼接的`/Users/neoliu/.aws/agenova-ca-bundle.pem`，仅profile配置，未关闭TLS。Docker构建通过可选BuildKit CA secret；CA不进入runtime镜像。
- Terraform1.16.4、AWS provider6.66.0、CLI2.37.4。Node24在/opt/homebrew/opt/node@24/bin，全局Node26未改。Docker已启动。
- kubeconfig独立文件`/Users/neoliu/.kube/agenova-demo`。AWS_PROFILE和KUBECONFIG必须显式设置。

## 查看与复现

当前本地Portal：http://127.0.0.1:5177/?mode=connected#/work/investigate-payment-retries

后台进程：`.tmp/e13/agenova api connect --state-dir .tmp/e13/state`（端口8088）和`npm --prefix ui run dev -- --host 127.0.0.1 --port 5177 --strictPort`。进程退出后可按原命令重启，不必重建AWS资源。

CLI二进制`.tmp/e13/agenova`；隔离state `.tmp/e13/state`；实际Platform和模板 `.tmp/e13/platform.yaml`、`.tmp/e13/engineer.yaml`。无密钥副本已保存于证据目录。可执行wrapper `.tmp/e13/agenova-live`带上正确profile/kubeconfig/state供E2E使用。

构建/配置：[EKS runbook](../../deploy/demo/eks/README.md)。ECR仓库agenova-demo/control-plane和agenova-demo/worker，镜像固定digest，清单[demo-image-inventory.json](demo-image-inventory.json)。

## 清理：必须执行

**悉尼2026-09-29 01:00 = UTC2026-09-28T15:00:00Z**。用户要求至少保留十小时，取代原四小时。权威本地记录[cleanup-deadline.txt](cleanup-deadline.txt)，Terraform ExpiresAt已同步。

Codex heartbeat ID `agenova-eks`已调度，但需要本机可运行和登录有效；不是AWS端硬停费保证。如果转给Claude CLI，接手者必须确认到期清理有人执行。账户预算AUD100/月，USD50告警非硬上限。

1. 到期验证STS operator/账户，确认没有并发apply。
2. 保留证据，检查Kubernetes额外创建的LB/PV（本次未创建）。
3. 审查Terraform destroy plan仅涉及本stack。
4. 对照镜像清单，仅删除本任务确切digest（先manifest list后子manifest）；未知镜像/持久数据先报告，不force_delete仓库。
5. 执行destroy，验证EKS、EC2、EBS、网络、ECR和日志残留。保留operator/admin组/默认VPC/本地代码和证据。预算告警也会随stack删除。
6. 完成后暂停自动化；登录过期则通知用户重新aws login，不切root。

不要删除本地Terraform state或旧stash。`stash@{0}`为旧分支Before syncing main 2026-09-28，与本任务无关，不能pop/drop。

## 给Claude CLI的接手提示

```text
阅读work/0175-aws-demo-bootstrap/HANDOFF.md及E13任务包/证据。EKS+Bedrock主链路已跑通，E14已按用户要求暂停，不能声称完整Epic完成。保留云端到悉尼9月29日01:00并按清单清理，仅用agenova-demo operator。保留state、证据和旧stash。先检查git和进程现状，再继续剩余验证或代码审查，不读取或输出密码/token。
```
