# Concepts

This page explains the ideas behind Telark: what an application is, how a protection plan works, where change history comes from, how Insights produces its findings, and how access control decides who may do what. For step-by-step use, start with [Getting started](getting-started.md). For the implementation, follow the links at the end of each section.

## Applications

Telark works on applications, not on individual Kubernetes objects. The discovery service watches the cluster and groups workloads into applications by their labels, in this order:

1. `app.kubernetes.io/name`
2. `app.kubernetes.io/part-of`
3. `app.kubernetes.io/instance`, when `app.kubernetes.io/component` is also set
4. `app`

Objects with the same value form one application, across namespaces. When a Deployment, its Service, its ConfigMaps and its autoscaler all carry `app.kubernetes.io/name: checkout`, they make up one `checkout` application, so you protect and inspect `checkout` instead of each object.

Each application is stored as an `Application` resource (`applications.telark.io`) with its health, namespaces, workloads, images and change history. Namespaces excluded in **Settings** (system namespaces are excluded by default) are not discovered. An application whose objects are all gone is removed automatically.

Depth: [discovery service](../services/discovery/README.md).

## Protection plans

A protection plan blocks chosen changes to a scope for a time window. It is Telark's answer to "nobody touches checkout between 22:00 and 02:00 tonight".

### What a plan contains

| Part | Choices |
|---|---|
| Scope | **Applications** (one or more applications, in every namespace they run in) or **Namespaces** |
| Exclusions | Kinds to leave unprotected (any scope), or named resources of the selected applications (applications scope only) |
| Policies | One or more templates, listed below |
| Mode | **Audit**: record what would be blocked, block nothing. **Enforce**: refuse the change at admission |
| Schedule | **Time range** (start and end) or **Permanent** |
| Execution | **Automatic**: deploys when the window starts. **Requires approval**: nothing deploys until an approver confirms |
| Classification | Optional environment and tags, used to filter plans. The Production environment always requires approval |

The nine policy templates:

| Template | Blocks |
|---|---|
| Block Resource Creation | Creating any resource in the scope |
| Block Resource Updates | Updating any resource in the scope |
| Block Resource Deletion | Deleting any resource in the scope |
| Block Image Patterns | Container images matching registry or name patterns you list |
| Block Image Tags | Container images with tags you list |
| Block Replica Scaling | Changing the replica count of Deployments and StatefulSets (applications scope only) |
| Block Storage Changes | Creating, changing or deleting PVCs and workload volume definitions |
| Block ConfigMap and Secret Changes | Updating or deleting ConfigMaps and Secrets |
| Block Workload Config Mount Changes | Changing volume mounts and ConfigMap or Secret sources on workloads |

### How a plan is enforced

While a plan is active, Telark renders each template into a namespaced Kyverno `Policy` in every namespace of the scope, minus the exclusions. Kyverno ships with the chart and checks each request at admission. In audit mode the request goes through and Kyverno records a violation; in enforce mode the request is refused with a message naming the plan.

Telark then checks the live cluster on a short interval (31 seconds by default). A policy that is missing or not ready marks the plan **Degraded**; a policy whose content differs from what the plan expects marks it **Drifted** and is redeployed. The plan's page shows this as its health.

By default Kyverno fails open: if its admission webhook is down, requests are admitted even for enforcing plans. You can make enforcement fail closed; see [Policy engine fail-open](INSTALL.md#policy-engine-fail-open).

### Lifecycle

| Phase | Meaning |
|---|---|
| `pending_approval` | Waiting for an approver. The person who requested it cannot approve it |
| `scheduled` | Approved or automatic, waiting for the window to start |
| `active` | Policies deployed and checked |
| `terminated` | The window ended; policies removed |
| `canceled` | Canceled by a user, or rejected by an approver; policies removed |
| `failed` | Activation failed (scope, render or deploy error) |
| `draft` | Defined in the API, not used today |

A canceled, terminated or failed plan can be reactivated, and any plan can be duplicated.

### Violations and reports

Violations come from the Kubernetes Events Kyverno writes, which the API server keeps for about an hour. Telark copies them into a per-plan ledger every 15 minutes, so a report covers the whole window. When a plan ends or is canceled, Telark captures a final report automatically; you can also generate one on demand. Reports are available as HTML, Markdown, JSON and CSV. Once a plan has ended, its page shows no live violations: its history is in the report.

Depth: [protection plans](architecture/protection-plans.md).

## Change history and rollback

When an application changes, discovery records the change field by field (an image, a replica count, an environment variable, a deleted object) and stores a sanitized snapshot of the manifests as they were before the change. The history and snapshots appear on the application's page.

A rollback applies a chosen snapshot back to the cluster: missing objects are created and changed ones replaced. A rollback is recorded as a change of its own, so it can be rolled back too, and an in-progress rollback can be aborted.

History shows who made a change for most kinds, taken from annotations a chart-installed Kyverno policy adds. Secrets are not annotated, so a Secret change shows no author. See [Last-modified annotations](INSTALL.md#last-modified-annotations).

Depth: [discovery rollback](../services/discovery/README.md#rollback).

## Insights

Insights helps you understand why an application is unhealthy and what to improve in its setup. It is decision support: it suggests causes and fixes, and it never changes the cluster.

### Incident cards

When an application degrades, the analyzer reads its overview, recent change history, warning events and the status of the affected workloads, then writes one card per affected workload. Each card names the likely cause: out of memory, image pull failure, crash loop, scheduling, failing probes, resource pressure, a stuck rollout, or a regression that started right after a config or resource change. It lists the evidence it used and the change it followed, and it resolves on its own when the workload recovers.

Analysis runs when you select **Analyze**, or automatically on incidents and recoveries when you turn that on in **Settings**.

### Setup review

The analyzer also reviews each application against 60 rules in 10 families: reliability, resources, scaling, security, images, config, networking, change risk, protection and consistency. It lists what to fix as recommendation cards, such as a single replica with no disruption budget in production. Reviews run on their own, after every analysis and every 2 hours by default, and never use the model.

An application counts as production when a namespace, or the environment of a protection plan covering it, matches a naming pattern (by default names containing `prod`, `production` or `prd`). Production raises the severity of some rules.

### How findings are produced

Deterministic rules decide every finding: its cause, its evidence and its severity. A small open-weight model (by default `granite4:350m`) then rewrites the title and summary of incident cards to read more naturally. Telark discards any rewrite that drops a fact from the rule's text or adds one, and keeps the rule's text instead. Recommendation cards are never rewritten.

The model runs in your cluster through Ollama, which ships with the chart. No data leaves the cluster, no API key is needed, and it works air-gapped once the model is loaded.

### Limits

- A card is a starting point. It can be wrong or incomplete, so check the evidence it cites.
- A run writes at most three incident cards.
- On the default CPU sizing, the model's rewrite takes several seconds per card. If the model is missing or slow, the cards keep the rule text.
- Usage rules need metrics-server data collected over at least 12 hours.
- Insights is on by default and optional. Turn it off in **Settings** and the rest of Telark works as before.

Depth: [how the analyzer works](../services/analyzer/ARCHITECTURE.md), [analyzer service](../services/analyzer/README.md).

## Access control

### Signing in

People sign in with a passkey or with Google SSO. There are no passwords. Passkey self-registration is off by default: the bootstrap admin enrolls with a one-time token from the break-glass command. Google users are created as ReadOnly on their first sign-in once the bootstrap admin turns SSO on in **Settings**, an admin can also create an account on **Members** and send its owner a one-time enrollment link, and an admin then grants roles, including Admin to at least two regular users. See [First admin](INSTALL.md#2-first-admin) and [Login and SSO](../services/auth/OIDC.md).

### Roles, levels and deny rules

Permissions are managed under **Administration**, with users, groups and roles.

A role grants a level on one or more areas, such as `applications`, `protectionplans`, `insights`, `users` or `settings`, or on `ALL` areas. The levels, from lowest to highest:

| Level | Typical use |
|---|---|
| ReadOnly | View |
| Contributor | Create and edit, for example create plans or run an analysis |
| Owner | Manage, for example approve plans, enforce a plan on whole namespaces, manage users |
| Admin | Everything, including sign-in settings |

A user's permissions combine their own roles and the roles of their groups. Only active users and active, unexpired roles count.

A role can also carry deny rules for single actions. A deny rule wins over any level, so you can grant Owner on protection plans and still deny approving them. Nobody can grant a role above their own level or change their own roles.

Depth: [security model](security/README.md).
