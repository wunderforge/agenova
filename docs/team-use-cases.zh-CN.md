# 团队内部使用场景

后台入口：[Administration](https://app.agenova.app/admin)。普通用户从 Portal 或 CLI 提交任务。

## 三种角色

| 角色 | 可以做什么 | 不包含什么 |
| --- | --- | --- |
| Platform Admin | 管理普通用户、在已分配项目提交任务/查看证据/管理治理配置、部署当前安装 | 不自动获得 AWS IAM 或 Kubernetes 权限 |
| Platform Operator | 对已授权安装部署、升级、诊断 | 不能管理用户、治理配置、提交任务或查看业务证据 |
| Member | 在已分配项目使用批准模板提交任务、查看自己的结果 | 不管理平台 |

Owner 是受保护的所有者身份，不是第四个可分配角色。Owner 和 Platform Admin 均可调整其他用户的三种角色；不能修改 Owner 或自己的角色。
Platform Admin 已包含部署授权，不需要再勾选 Operator。切换角色会原子撤销不兼容的旧权限。

## 常见场景

| 场景 | Agenova 权限 | 额外条件 |
| --- | --- | --- |
| 新同事提交任务 | Member + 指定项目 | 先 Google 登录，再由管理员审批 |
| 查看自己运行的结果 | Member | 使用自己的登录会话 |
| 查看项目所有任务证据 | Platform Admin + 指定项目 | 仍受账户/项目边界限制 |
| 审批新成员、停用普通账号 | Platform Admin | 只能管理授权范围内的普通成员；不能任命管理员 |
| 任命管理员 | Owner / Platform Admin | 在后台选择 Platform Admin |
| 升级控制面镜像 | Platform Operator 或 Platform Admin | 自己的 AWS SSO + EKS 权限；审核升级配置 |
| 注册 Policy / AgentTemplate | Platform Admin + 指定项目 | 不变注册：同身份不同内容冲突；不是任意 CRUD |
| 修改模型配置 | Platform Admin 是应用管理角色 | 当前 member-upgrade 只改镜像；需另外审核部署配置，不代表该命令支持模型修改 |
| 撤销运维权限 | Owner 或有权管理员切换角色 | 切换 Member 会撤销应用部署/治理权限；AWS 权限需在 Identity Center 单独撤销 |

## CLI

```bash
agenova login --server https://app.agenova.app
agenova run -f work.yaml
agenova work list
agenova work show <request-ref>
```

部署人员另外配置 AWS SSO 和 kubeconfig：

```bash
aws sso login --profile agenova-sso
export AWS_PROFILE=agenova-sso
export KUBECONFIG="$HOME/.kube/agenova-sso"
agenova platform plan -f platform.yaml --server https://app.agenova.app --installation demo-eks-bedrock
agenova platform apply -f platform.yaml --server https://app.agenova.app --installation demo-eks-bedrock
agenova platform status --server https://app.agenova.app --installation demo-eks-bedrock
```

`platform.yaml` 必须使用当前 Deployment UID、审核过的模板摘要和兼容镜像；不能直接复制过期绑定。
安装状态使用个人默认目录，不需要共享 `state-dir`。应用角色不代替 AWS / Kubernetes 授权。

## 本次迁移

按 Owner 的明确要求，仅把当前账户现有用户升级为 Platform Admin，保留 Owner 和禁用状态。
新注册用户仍待审批，默认 Member，绝不自动成为管理员。
数据库底层保留 account/project/scoped grants 以兼容已有会话和审计；后台只提供三种规范角色。
历史自定义组合显示为需审核的旧权限，不会伪称已经迁移。迁移记录单独保存前后状态和审计。

## 在后台编辑治理配置

打开 `/admin#governance`，选择项目、Policy 或 AgentTemplate，点击
**Load current configuration**，修改 JSON 后 **Save configuration**。
Policy 修改内容时必须使用新的 `metadata.version`，保存后对新 Work 生效。
Template 修改时使用新的 `metadata.name`，随后在 Policy 中允许该名称，并在新
Work 中选择它。旧记录不覆盖；正在运行的任务不会被改写。服务端仍检查项目权限、
文档格式及兼容 Worker 镜像/入口；拒绝或版本冲突会明确显示。
