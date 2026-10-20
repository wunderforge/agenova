# Agenova Team Quick Guide

Role guide updated: 19 October 2026. Cloud access and installation availability are environment-specific.

See also: [team use cases and permission examples (Chinese)](team-use-cases.zh-CN.md).

| Entry point | Purpose | Required access |
| --- | --- | --- |
| Agenova Portal / CLI | Submit agent tasks and inspect results and evidence | Agenova account/project roles |
| AWS / kubectl | Manage cloud resources and deploy or upgrade services on EKS | AWS account and EKS cluster permissions |

Both use your personal Google account, but authorization is separate. Ordinary
Agenova members do not need AWS credentials, kubeconfig, or cluster access.

## 1. Sign in to Agenova

Open <https://app.agenova.app/>, select **Sign in**, then **Continue with Google**.
If access is pending, send your sign-in email to an administrator. After approval
and project-role assignment, sign in again.

The homepage is public; the workspace is not. After signing in, open **Work** to
see your accessible requests and their activity. Select a request to inspect its
result, requested/granted permissions and recorded evidence. The activity chart
summarizes the loaded list, not all organization traffic.

Administrators use **[Administration](https://app.agenova.app/admin)** to manage users and approve access requests. The previous `/members` URL redirects there.

| Role | Application permissions |
| --- | --- |
| Platform Admin | Manage other users and their operational roles, run Work/read project evidence, manage governance in assigned projects, deploy the serving installation |
| Platform Operator | Deploy/upgrade/diagnose the assigned installation; no Work, business evidence, user or governance management |
| Member | Submit approved Work and read your own results in assigned projects |

Owner is protected and is not an assignable everyday role. Owner and Platform Admins can assign operational roles. Owner and self-role changes remain protected. Platform Admin includes deployment authorization, so it
does not need a second Operator role. Cloud IAM/Kubernetes permissions are separate.
Role changes atomically replace incompatible grants; they do not modify Work policy.
Model configuration changes are not part of the existing image-only upgrade command.

## 2. Install and use the Agenova CLI

See [platform installation instructions](cli-installation.md) for macOS, Linux, WSL2 and Windows.

The same `agenova` commands work on macOS and Windows; only installation differs.

### macOS (Homebrew)

With [Homebrew](https://brew.sh/) installed:

```bash
brew install Leo1piece/agenova/agenova
agenova --version
```

### Windows (PowerShell)

Download [install-windows.ps1](https://github.com/Leo1piece/homebrew-agenova/releases/download/v0.1.0-preview.20261013.1/install-windows.ps1), review it, then run from your Downloads directory:

```powershell
.\install-windows.ps1
agenova --version
```

No WSL, Homebrew or administrator shell is needed. If your execution policy blocks
scripts, use the verified ZIP method in the installation guide instead. Windows
interactive login still needs a real Windows acceptance check.

### All platforms: login and run

```bash
agenova login --server https://app.agenova.app
```

Complete Google login in the browser using your approved account. The CLI
remembers the server after successful login. The public tap currently distributes
a preview release. See the installation guide for platform prerequisites and
verification limits.

```bash
agenova work list
agenova work show <request-ref>
agenova run -f work.yaml
```

Replace `<request-ref>` with an actual returned reference. Save the
[example Work YAML](examples/member-work.yaml) as `work.yaml` before submitting;
it requires access to the `payments` project and its approved template/model
profile. Use a new `metadata.name` for each new task.

Submission still goes through server-side role, project, Policy and Template
checks. Requested access does not grant authority. `Succeeded` describes execution
status; inspect the answer separately for quality.

If macOS prompts for Keychain access, verify it is your installed Agenova CLI,
then enter your Mac login password and optionally choose **Always Allow**.
Upgrades or a locked Keychain may prompt again. Never share the password.

Use `agenova logout` to sign out of the CLI. Portal **Log out** ends the separate
browser session.

## 3. Sign in to the team AWS account

This section is for authorized infrastructure operators. Ask an AWS administrator
to provision your Google email in IAM Identity Center and assign account access.

Open <https://d-97678162bb.awsapps.com/start>, select **Continue with Google**, then:

- Account: **agenova** (`931228356546`).
- Role: **AgenovaAdministrator**.

Only explicitly assigned users can select this role. It currently grants AWS
account administrator access and administrator access to the `agenova-demo` EKS
cluster. It is not the same as an Agenova application administrator role.

### Identity Center, permission sets, and IAM roles

IAM Identity Center manages team sign-in and account assignments. A permission
set defines the permissions assigned to members; Identity Center provisions an
IAM role in the target AWS account to implement those permissions.

| Location | Name in this environment |
| --- | --- |
| Identity Center permission set / portal role choice | `AgenovaAdministrator` |
| Automatically provisioned IAM role | `AWSReservedSSO_AgenovaAdministrator_43738c55e42c3fae` |

The flow is **Google → Auth0 → IAM Identity Center → assigned IAM role → AWS/EKS**.
Search IAM **Roles** for `AWSReservedSSO_AgenovaAdministrator` to find the generated
role. Manage membership and permission assignments in Identity Center rather than
editing this generated role directly. Its suffix can change if the role is
deleted and recreated; EKS access entries must then reference the new role ARN.

## 4. Configure local AWS SSO

```bash
brew install awscli kubectl
aws configure sso --profile agenova-sso
```

Enter these values when prompted:

| Setting | Value |
| --- | --- |
| SSO session name | `agenova-team` |
| SSO start URL | `https://d-97678162bb.awsapps.com/start` |
| SSO region | `ap-southeast-2` |
| Registration scopes | Press Enter for `sso:account:access` |
| AWS account | `931228356546` |
| Role | `AgenovaAdministrator` |
| Default region | `ap-southeast-2` |
| Output format | `json` |

Complete browser authorization, then verify your identity:

```bash
aws sts get-caller-identity --profile agenova-sso
```

The returned account should be `931228356546`, and the assumed-role identity
should identify your assigned SSO role and email. No long-lived access keys are
needed.

## 5. Connect kubectl to EKS

Generate a separate kubeconfig once:

```bash
aws eks update-kubeconfig \
  --name agenova-demo \
  --region ap-southeast-2 \
  --profile agenova-sso \
  --kubeconfig "$HOME/.kube/agenova-sso" \
  --alias agenova-sso

export KUBECONFIG="$HOME/.kube/agenova-sso"
kubectl get pods -n agenova-system
```

In subsequent terminal sessions:

```bash
export KUBECONFIG="$HOME/.kube/agenova-sso"
# Renew the SSO session when needed.
aws sso login --profile agenova-sso
kubectl get pods -n agenova-system
```

As of the verification date, the cluster temporarily accepts all public IPv4
source addresses, so no VPN or individual IP allowlist entry is required.
AWS authentication and EKS authorization remain required. Confirm the current
network policy with an operator if connectivity changes.

## Troubleshooting

| Symptom | Action |
| --- | --- |
| Agenova access is pending | Ask an Agenova administrator to approve your account and assign a project role. |
| No AWS account or role appears | Ask an AWS administrator to assign Identity Center account access. |
| AWS session expired | Run `aws sso login --profile agenova-sso`. |
| kubectl returns Forbidden | Check the selected kubeconfig and the role's EKS access entry. |
| kubectl connection times out | Check network connectivity and the cluster endpoint IP allowlist. |
| `work.yaml` not found | Save the example first or specify the actual file path. |

Use your own Google account. Do not share credentials, SSO caches or another
member's CLI login data.

## Edit Policy and AgentTemplate in the browser

Platform Admins can open [Administration](https://app.agenova.app/admin#governance),
choose a project and document type, then **Load current configuration**. Edit the
JSON and **Save configuration**. For changed Policy content, increment
`metadata.version`; saving activates that version for new Work. For changed
Template content, use a new `metadata.name`, allow it in the project Policy, and
select it when submitting Work. Existing registrations and running Work are not
overwritten. The compatible worker image/entrypoint restrictions still apply.
A conflict or denied request is shown explicitly; configuration does not grant
cloud access. The editor is JSON; the existing CLI also accepts YAML.
