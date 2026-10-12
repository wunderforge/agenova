# Agenova 新人上手

## 1. 注册并获得权限

打开 <https://app.agenova.app/login>，点击 **Continue with Google**，使用自己的 Google 账号登录，不需要另设 Agenova 密码。

首次登录若显示等待授权，请联系管理员。管理员在 **Members → Pending access requests** 中批准账号，并分配项目角色：

- 日常提交任务：Account Member + 对应项目的 Member。
- 只查看任务：对应项目的 Viewer。
- 项目管理：Project Admin，由 Owner 授予。

批准后重新登录。普通成员不需要 AWS 账号、AK/SK、kubectl 或集群权限。

## 2. 安装 CLI（macOS）

先安装 [Homebrew](https://brew.sh/)，再运行：

```bash
brew install Leo1piece/agenova/agenova
agenova --version
```

[公开 tap](https://github.com/Leo1piece/homebrew-agenova) 当前提供 `0.1.0-preview.20261012.1` 预览版，安装时自动编译；已实测 macOS Apple Silicon。

更新：`brew update && brew upgrade Leo1piece/agenova/agenova`。

## 3. 登录 CLI

```bash
agenova login --server https://app.agenova.app
```

在浏览器选择刚才获批的 Google 账号，终端显示 `login succeeded` 即完成。CLI 会记住服务器。

macOS 若询问钥匙串访问，确认是自己安装的 Agenova 后可选择 **Always Allow**，输入 Mac 登录密码。不要把密码发给管理员。程序升级或钥匙串锁定后仍可能再次询问。

## 4. 提交第一个任务

将以下内容保存为当前目录下的 `work.yaml`（也可使用[示例文件](examples/member-work.yaml)）（示例需要 `payments` 项目权限）：

```yaml
apiVersion: agenova.io/v1alpha1
kind: ClaimRequest
metadata:
  name: newcomer-payment-analysis-20261013
spec:
  templateRef: engineer
  projectRef: payments
  task:
    type: investigation
    input:
      objective: Explain why resetting a five-second timeout on every payment retry can exceed an overall deadline, and suggest a fix. Use only this description; do not access repositories or tools.
  requestedAccess:
    tools: []
    resourceScopes: []
    modelProfile: coding-standard
  runtime:
    profileRef: standard-isolated
    timeout: 5m
```

```bash
agenova run -f work.yaml
agenova work list
agenova work show <上一步返回的request-ref>
```

`Succeeded` 表示执行流程成功，仍需检查回答是否满足任务要求。也可打开网页的 Work 页面查看。
每个新任务使用新的 `metadata.name`。本例只调用获准的模型，不访问仓库。
账号授权由管理员批准一次；任务通常自动检查 Policy/Template，不逐次人工批准。角色不能绕过模型、工具和项目限制。

退出 CLI：`agenova logout`。网页退出使用 **Log out**，两者是独立登录会话。

## 已使用旧的本地 tap

只有之前安装过 `agenova/preview` 的用户需要切换；新用户跳过：

```bash
brew uninstall agenova/preview/agenova
brew untap agenova/preview
brew install Leo1piece/agenova/agenova
```

切换后若出现钥匙串提示，按第 3 步处理。
