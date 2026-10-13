# Agenova Team Quick Guide

Environment details last verified: 13 October 2026.

See also: [team use cases and permission examples (Chinese)](team-use-cases.zh-CN.md).

| Entry point | Purpose | Required access |
| --- | --- | --- |
| Agenova Portal / CLI | Submit agent tasks and inspect results and evidence | Agenova account/project roles |
| AWS / kubectl | Manage cloud resources and deploy or upgrade services on EKS | AWS account and EKS cluster permissions |

Both use your personal Google account, but authorization is separate. Ordinary
Agenova members do not need AWS credentials, kubeconfig, or cluster access.

## 1. Sign in to Agenova

Open <https://app.agenova.app/> and select **Continue with Google**.
If access is pending, send your sign-in email to an administrator. After approval
and project-role assignment, sign in again.

- **Member:** submit Work and read your own Work and evidence.
- **Viewer:** read project Work and evidence.
- **Project Admin:** administer assigned projects and submit/read project Work.
- **Account Admin / Owner:** account-level membership and authorization duties.
- **Governance Admin / Platform Operator:** separately scoped governance or
  deployment duties; these do not automatically grant AWS or Kubernetes access.

## 2. Install and use the Agenova CLI

On macOS with [Homebrew](https://brew.sh/) installed:

```bash
brew install Leo1piece/agenova/agenova
agenova --version
agenova login --server https://app.agenova.app
```

Complete Google login in the browser using your approved account. The CLI
remembers the server after successful login. The public tap currently distributes
a preview release, tested on macOS Apple Silicon.

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
