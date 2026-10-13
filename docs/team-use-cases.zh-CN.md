# 团队内部使用场景与权限

更新：2026-10-13。入口：[Agenova Portal](https://app.agenova.app/)。
安装、Google 登录、AWS SSO 和 kubectl 配置见 [Team Quick Guide](team-quick-guide.md)。

这里的“提交工单”指提交 Agenova Work；创建 Jira/GitHub 工单是另一种工具集成，
不能因为能提交 Work 就认为已能创建外部工单。

## 先区分三类授权

- **账户角色**：Owner、Account Admin、Account Member，管理人员和账户访问。
- **项目角色**：Project Admin、Member、Viewer，管理指定项目、运行任务或阅读证据。
- **专项职责**：Governance Admin、Platform Operator，管理获授权范围的治理或部署。

同一个人可以兼任这些角色。Account Admin 不自动成为每个项目的 Admin；
Owner 不自动获得 Governance Admin、Platform Operator 或 AWS 权限。
普通成员只用 Google 登录 Agenova。基础设施操作还需要自己的 AWS SSO 和 Kubernetes 权限。

## 常用场景

| 场景 | 建议授予的权限 | 操作入口 | 关键限制 |
| --- | --- | --- | --- |
| 新同事首次加入 | Account Member + 指定项目 Member | Google 登录后，管理员在 Members 审批 | 注册成功不等于能访问项目；审批后重新登录 |
| 提交一次问题分析/开发任务 | 项目 Member | Portal New work 或 `agenova run -f work.yaml` | 必须使用获准模板、模型 Profile 和工具范围；角色不绕过 Policy |
| 查看自己的任务、回答和调用证据 | 项目 Member | `agenova work list`、`agenova work show <request-ref>` | Member 不能查看其他成员的 Work |
| 测试/审计同事查看项目内全部结果 | 项目 Viewer | Portal、`agenova work list/show` | 只读，不能提交任务或修改成员；不需要 AWS 权限 |
| 项目负责人管理项目成员并提交任务 | Project Admin | Members + Work CLI | 只管理其项目的 Member/Viewer；不能任命管理员或扩大账户权限 |
| 管理新成员待审批请求 | Account Admin + 对应项目 Project Admin | Members → Pending access requests | 给普通用户分配 Member/Viewer；任命管理员由 Owner 操作 |
| 任命管理员或授予专项职责 | Account Owner | Members | 授予治理/部署职责仍需明确范围；不自动分配云权限 |
| 注册新的治理 Policy 版本 | 项目范围 Governance Admin | `agenova policy apply` | 版本不可原地改写；新版本激活影响后续 Work，不扩大已有 claim |
| 注册兼容的 AgentTemplate | 项目范围 Governance Admin | `agenova agent-template apply` | 不是任意 CRUD；已有同身份不同内容会冲突，且不能任意选择 Worker 镜像 |
| 升级当前 SSO 控制面镜像 | 安装范围 Platform Operator + 自己的 AWS/EKS 部署权限 | `agenova platform plan/apply/status` | 使用 member-upgrade 配置；会短暂中断；不修改数据库结构或登录配置 |
| 排查 Pod、读取容器日志 | AWS SSO 身份 + 对应命名空间的 Kubernetes 读取权限 | `kubectl get`、`kubectl logs` | 应用角色不会授予日志权限；日志可能含敏感运维信息 |
| 扩缩节点、修改 EKS 网络或 IAM | 指定基础设施运维人员的 AWS 权限 | Terraform / AWS CLI | 不属于 Agenova Work 或应用角色授权，不能仅授予 Platform Operator 就完成 |
| 更换 Bedrock 模型或新增 Model Profile | 指定平台运维人员；通过 Agenova 部署路径时还需 Platform Operator | 平台配置变更流程 | 本次 member-upgrade **只支持镜像升级**，会拒绝模型/运行时变化；当前 SSO 配置更新尚未在这条 CLI 路径完成验收 |
| 移除离职成员 | 相应成员管理权限；管理员/专项职责由 Owner 处理 | Members + AWS Identity Center | 必须分别撤销 Agenova 和 AWS 权限；停用 Agenova 不等于撤销 AWS SSO |
| Agent 自动创建 GitHub/Jira 工单 | 项目提交权限 + 已实现的工具 adapter + Policy/Template 获准工具和资源范围 | Work 的受治理工具调用 | 目前这份验收没有证明真实工单工具可用；不能将模拟工具结果当作真实创建 |

## 例子一：普通成员提交 Work

给同事 `payments` 项目的 Member 即可，不需要 Account Admin、Platform Operator、
AWS Access Key 或 kubeconfig。

```bash
agenova login --server https://app.agenova.app
agenova run -f work.yaml
agenova work list
agenova work show <实际返回的request-ref>
```

使用 [现成示例](examples/member-work.yaml)，修改 `metadata.name` 和任务目标。
是否可运行由当前项目权限、模板和 Policy 决定。当前并非每个 Work 都先走人工审批；
获准请求执行，不获准请求拒绝。成员入组审批与每次任务的授权检查是两回事。

## 例子二：Fisher 升级控制面

本次真实验证的授权组合：

- Agenova：`Platform Operator`，installation 为 `demo-eks-bedrock`。
- AWS：Fisher 自己的 `AgenovaAdministrator` SSO 会话。
- EKS：该 SSO IAM Role 的集群访问授权。当前权限较宽，后续可独立收窄。

```bash
aws sso login --profile agenova-sso
export AWS_PROFILE=agenova-sso
export KUBECONFIG="$HOME/.kube/agenova-sso"
agenova login --server https://app.agenova.app
agenova platform plan -f platform.yaml --server https://app.agenova.app --installation demo-eks-bedrock --state-dir ./operator-state
agenova platform apply -f platform.yaml --server https://app.agenova.app --installation demo-eks-bedrock --state-dir ./operator-state
agenova platform status --server https://app.agenova.app --installation demo-eks-bedrock --state-dir ./operator-state
```

需要包含新升级适配器的 CLI；最初的公开 Homebrew preview 尚不包含此模式。
配置必须绑定当前 SSO Deployment，不能拿旧 reference 平台安装文件覆盖线上服务。
具体配置与回退限制见 [member service 运维说明](../deploy/production/member-service.md#upgrading-an-existing-member-deployment-with-the-cli)。
`operator-state` 是部署者本地记录，不是成员共享的权限文件，也不包含用户登录令牌。

## 例子三：治理同事更新 Policy 或 Template

Owner 给此人授予 `Governance Admin`，范围限定在项目 `payments`。
该职责本身不授予 Work 提交或跨项目证据读取权限；需要测试时再加项目 Member。

```bash
agenova policy apply -f policy.yaml --server https://app.agenova.app --project payments
agenova agent-template apply -f engineer.yaml --server https://app.agenova.app --project payments
```

这些是版本化注册接口，不是任意编辑/删除配置。它们有代码和自动化测试支持；
不应把本次 Fisher 镜像升级验收同时描述为这两个命令的真实用户验收。

## 建议团队分配

| 团队职责 | Agenova 配置 | 是否需要 AWS |
| --- | --- | --- |
| 一般开发者 | Account Member + 项目 Member | 否 |
| QA / 只读审查 | Account Member + 项目 Viewer | 否 |
| 项目负责人 | Account Member + Project Admin | 通常不需要 |
| 团队成员管理者 | Account Admin + 负责项目的 Project Admin | 通常不需要 |
| 治理负责人 | 项目范围 Governance Admin，按需叠加 Member/Viewer | 通常不需要 |
| 部署值班人员 | 安装范围 Platform Operator，按需叠加项目角色 | 是 |
| 账户负责人 | Owner，其他职责按实际需要单独授予 | 只有兼任基础设施运维时需要 |

不要为了让同事运行一个 Work 就给 AWS AdministratorAccess。
反过来，具有 AWS/Kubernetes 管理权限的人可以直接操作集群；应用角色不能限制其
独立持有的云权限，限制必须在 IAM 和 Kubernetes RBAC 层落实。
