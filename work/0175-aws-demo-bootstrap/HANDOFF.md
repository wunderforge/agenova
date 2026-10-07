# E13 AWS / Bedrock 交接

## 最新覆盖记录：2026-10-05

以下旧章节保留历史背景；本节和 `cleanup-deadline.txt` 覆盖旧访问方式、profile 和期限。

- 10月2日已销毁旧演示栈；10月5日按用户要求重新创建 27 个 EKS 基础资源和 9 个 ALB 入口资源。
- 当前使用 `AWS_PROFILE=agenova-demo-ak`，STS 已核验为同一 operator；专用 kubeconfig 也绑定该 profile。不要使用旧的过期网页登录会话。
- 入口 https://demo.agenova.app/ 已恢复，HTML 共享演示登录 → ALB → EKS；不依赖本机端口转发或 Cloudflare。根路径自动跳转登录页。
- Nova Micro 真实 Work 成功；拒绝前后 provider 计数 4→4；21 项 HTTPS 检查通过。新证据：[恢复记录](../0175-shared-eks-bedrock/evidence/restore-20261005/summary.md)。
- 清理不早于 UTC 2026-10-05T17:35:32Z，即悉尼 2026-10-06 04:35 AEDT；保留至少十小时。最新用户期限优先。
- 清理顺序：先审查并 destroy `infra/demo/edge`，删除 Kubernetes 公网入口，再按更新后的精确镜像清单清理 ECR，最后审查并 destroy `infra/demo/aws`。保留 `infra/domain`、注册域名、hosted zone、证书与验证记录、IAM 用户/admin 组和默认 VPC。
- 自动化 `agenova-eks` 已恢复为每小时检查，使用 AK profile；仍依赖本机在线。域名和证书不能随演示清理。
- PR #196 对应交付子任务 #200；E14 #197 仅有规划文档，无实现 PR 可用。不得宣称完整 E13 已完成。

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

## Git 发布状态

实现与证据已提交为 `a742b20`，工作树干净。首次push/PR失败：当前gh账号yliu0661_syduni是Enterprise Managed User，推送403、createPullRequest被企业账号限制拒绝。尚未成功发布远端分支或创建PR。已询问用户是否改用本机已登录的Leo1piece，未自动切换账号。拟好的PR正文在/tmp/agenova-e13-pr.md。等待用户选择；云端运行与到期清理不受影响。

## Authenticated public HTTPS overlay

Owner subsequently authorized a provider-issued HTTPS address with minimum access
control. The separate `agenova-demo-public` Deployment runs nginx, cloudflared and
a Pod-scoped kubectl forwarder inside EKS. See `deploy/demo/eks/public/README.md`.
Current URL is saved in `.tmp/e13/public/url.txt`; shared demo credentials are in
`.tmp/e13/public/credentials.json` (0600, never commit/print). No new AWS resources
or ECR images. Existing control plane was not restarted. This is shared reference
Team A, not E14 identity. Remove the overlay Deployment, ServiceAccount, Role,
RoleBinding, Secret and two ConfigMaps before Terraform destroy. Deadline unchanged:
2026-09-28T15:00:00Z. The public endpoint does not depend on the laptop, but scheduled
Terraform cleanup still requires the local automation host and valid AWS login.

## Owner's permanent domain — preserve during demo teardown

Owner purchased `agenova.app` and explicitly requested reuse of existing public
hosted zone `Z00685831HNHNRLEZ9VYK`. Do not delete this zone, domain registration,
or its NS/SOA records during EKS teardown.

Prepared ACM DNS-validated non-exportable certificate for `demo.agenova.app` in
us-east-1: `arn:aws:acm:us-east-1:931228356546:certificate/b3491085-3fd7-4649-a2c8-40a12128de7f`.
Created only its verification CNAME in that existing zone:
`_2072119b633521d8f4e042dc1dbab723.demo.agenova.app.` →
`_18d2ca087adc2a953661695f7accc2f5.wzccmgtwzk.acm-validations.aws.`
Preserve the certificate and verification record with the permanent domain.
At preparation, registrar operation d84a7da2-9313-4073-af43-ac02e9881d9b still reported
IN_PROGRESS and public NS resolution was empty; certificate PENDING_VALIDATION.
No CloudFront distribution or demo traffic DNS record created yet. Do not point
this hostname directly at the temporary trycloudflare hostname: that does not
provision the required custom-hostname TLS/routing. Finish the authenticated
custom-domain ingress after public delegation/certificate validation succeeds.

### Terraform ownership supersedes CLI-only setup

Both permanent-domain resources are now imported into `infra/domain` local state:
`aws_acm_certificate.demo` and
`aws_route53_record.validation["demo.agenova.app"]`. The existing zone is a data
source only. Apply added only the ManagedBy=terraform tag (0 add, 1 change,
0 destroy); no certificate was recreated. Certificate remains visible in ACM
us-east-1, not Sydney. Preserve this root/state during demo teardown. Follow
`infra/domain/README.md`; no CloudFront/traffic alias has yet been provisioned.

### Apex + wildcard certificate supersedes demo-only certificate

Owner requested Chrome console setup for `agenova.app` plus `*.agenova.app`.
New certificate `146c07de-d961-43f3-a291-d27bd552c410` in us-east-1 is ISSUED,
both names validated successfully. Owner deleted old b3491085 certificate.
Terraform `infra/domain` now imports the new certificate at the existing demo
resource address; the old deleted certificate state was removed, not recreated.
One shared validation CNAME `_c2bcda06481a5cdcdce022a8459d61c1.agenova.app.` points to
`_dee5d55d6ce64231f3ff6904ca2b3a93.wzccmgtwzk.acm-validations.aws.`.
Old demo verification CNAME is retained under `aws_route53_record.legacy_validation`.
Preserve all permanent domain resources during EKS cleanup. Issuance alone does
not publish the website: CloudFront/traffic DNS is still outstanding.

## ALB direct ingress supersedes both CDNs

Latest Owner instruction: Route 53 → Sydney ALB → EKS, without CloudFront or
Cloudflare. `infra/demo/edge` owns disposable ALB, listener, target group, managed
node ASG attachment, restricted SG rules and demo traffic alias. Destroy edge
BEFORE EKS. `infra/domain` additionally owns Sydney apex/wildcard ACM certificate
05298630-af13-42c2-8ffa-4bed2401d2ea; preserve it and all permanent domain resources.
Kubernetes overlay includes NodePort Service 31089; delete that Service with the
other overlay objects. HTTPS password stays in existing ignored credentials file.
Check final evidence for cutover/CloudFront removal status; never assume a
partially completed transition removed the old distribution E326A8GTQMZBIR.

Final ALB verification: CloudFront E326A8GTQMZBIR was deleted (AWS returns
NoSuchDistribution); both domain and edge refreshed Terraform plans report no
changes. Normal Mac DNS now works. All 11 HTTPS perimeter assertions and the
rendered real Work pass without a DNS override; see evidence/alb/https-checks.json
and evidence/alb-portal.png in work/0175-shared-eks-bedrock.

The public perimeter now uses HTML /login and /logout with an eight-hour
server-side session, replacing Basic Auth. Existing demo credentials are unchanged.
Deploy with AGENOVA_SESSION_IMAGE=931228356546.dkr.ecr.ap-southeast-2.amazonaws.com/agenova-demo/control-plane@sha256:394646864093320ca64fe8ca4006ed6a3ba8d7ba81c70f4a53a975663037b467.
Session images share the existing control-plane ECR repository; refreshed inventory
includes their digests for cleanup. No new AWS infrastructure or cleanup timing.
