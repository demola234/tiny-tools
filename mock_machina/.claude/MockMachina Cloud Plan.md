# MockMachina Cloud plan (mockmachina.dev)

Status: proposal, 2026-10-15. Nothing here is built. It turns the hosted half of [the dashboard mockup](MockMechina%20Dashboard.html) into a plan, and lists the decisions to make before any code.

## What it's for

The local mock works on one machine and its Wi-Fi. MockMachina Cloud runs the same contract, from git, at a public HTTPS address, so that:

- real phones on mobile data can reach it, with no LAN addresses or cleartext exceptions;
- CI and end-to-end tests (Playwright, Maestro, XCTest) point at a URL that's always up;
- designers, PMs and QA use the app against it with nothing installed;
- every pull request gets its own URL, so a contract change can be tried in the app before anyone agrees to it.

Cloud is optional. The contract stays in git, the CLI keeps working offline, and nothing local depends on an account.

## What the mockup asks for

| Area | Mockup pieces |
| --- | --- |
| Sign-in and organisations | Log in, organisation and project switcher, user menu, people and teams, invites, roles, GitHub team sync, access, approvals, audit log |
| Projects | "Connect your first project", "New project from a GitHub repo", "Add the contract folder", `cloud.yaml` |
| Environments | `https://shop-app.mockmachina.dev`, `pr-44--shop-app` previews, custom domain, deploys, versions, `mockmachina deploy` |
| Hosted dashboard | The Phase 9 dashboard against a cloud environment, plus notifications and "Since you last looked" across the team |
| Reviews and specs | Reviews on a pull request, sharing a spec outside the repo |
| Billing | Plans, compare plans, change plan, payment method, invoices, Enterprise contact, delete organisation |

## Shape of the system

```text
GitHub ──webhook──▶ control plane (mockmachina.dev) ──deploys──▶ environment runtimes
  ▲                    │  accounts, orgs, projects,               (one per environment,
  │                    │  deploys, billing, audit                 the mockmachina binary)
  └──GitHub App────────┘                                                 │
                       browser ◀── dashboard (same htmx UI) ◀────────────┘
                       CLI ──── mockmachina login / deploy ──▶ control plane
```

- **Control plane:** a web app and API at `mockmachina.dev`. It owns accounts, organisations, projects, environments, deploy history, billing and the audit log. It stores metadata only; the contract itself is never edited or stored as the source of truth here.
- **Environment runtimes:** each environment (main, each pull request, each custom domain) runs the existing `mockmachina` server on the contract at one commit. Pins, data, history and coverage stay per environment, as they are locally.
- **Git is the source of truth:** a GitHub App receives pushes and pull-request events, and the control plane deploys the commit. Edits made in a hosted dashboard become commits or pull requests, never silent database changes.

## Decisions to make first

| # | Question | Options | Recommendation |
| --- | --- | --- | --- |
| C-1 | How environments run | One container per environment that scales to zero (for example Fly.io Machines or Cloud Run) / one multi-tenant process | Container per environment: real isolation, reuses the binary as is, and an idle preview costs nothing. Revisit multi-tenant only if cost demands it |
| C-2 | How people sign in | GitHub OAuth / email magic link / SSO (SAML, OIDC) | GitHub first, since deploys need repository access anyway; email links for people without GitHub; SSO on the Enterprise plan |
| C-3 | How the repo connects | GitHub App / personal tokens | A GitHub App with read access to contents and write access to checks and pull-request comments only |
| C-4 | Control-plane stack | Go with server-rendered htmx (like the dashboard) / a separate SPA | Go and htmx: one language, one UI approach, and the hosted dashboard can reuse Phase 9's templates |
| C-5 | Data store | Postgres / SQLite per region | Postgres for the control plane; environments keep their runtime state in memory, as locally |
| C-6 | Who can reach a hosted mock | Public / unguessable URL / token / signed-in members only | Per environment: private by default (token or signed-in members), with public as an explicit switch. The control API and dashboard always need a member or an API key |
| C-7 | Abuse | Public mock URLs can be used to host phishing or malware | Only serve what the contract describes; block HTML bodies on public environments unless the organisation is verified; rate limits; a report-abuse address |
| C-8 | Traffic logs | Keep none / keep N days | Keep recent traffic per environment for a short, plan-based window; bodies off by default; delete on request |
| C-9 | Billing | Stripe Billing / hand-rolled | Stripe Billing and the Customer Portal; plans by environments and seats |
| C-10 | Licence and code | Cloud in this monorepo / a separate private repo | Keep the CLI open source (ADR 005); the control plane in a separate repository, with the runtime image built from the public binary |
| C-11 | Domain and TLS | `mockmachina.dev` wildcard / custom domains | A wildcard certificate for `*.mockmachina.dev`; ACME (Let's Encrypt) per custom domain after a DNS check |

## Phases

| Phase | Delivers | Depends on |
| --- | --- | --- |
| C1: Accounts | Sign in with GitHub and email; organisations; members and roles (owner, admin, member); `mockmachina login` | C-2, C-4, C-5 |
| C2: Projects and environments | GitHub App; connect a repository; `cloud.yaml`; deploy main to `<project>.mockmachina.dev` on push; deploy history and rollback; `mockmachina deploy` | C1, C-1, C-3, C-11 |
| C3: Hosted dashboard | The Phase 9 dashboard against an environment, with members' sessions instead of the control token; per-environment API keys for CI | C2, C-6 |
| C4: Pull-request previews | `pr-<n>--<project>.mockmachina.dev` for each pull request, its URL in the PR comment, torn down on merge or close | C2 |
| C5: Teams and governance | GitHub team sync, route-group owners from `team.yaml`, approvals, audit log, access settings | C1, local Phase 13 |
| C6: Billing | Plans, Stripe checkout, invoices, payment method, plan limits, Enterprise contact, delete organisation | C1, C-9 |
| C7: Custom domains and Enterprise | Custom domains with automatic TLS, SSO, longer traffic retention | C2, C6 |
| C8: Shared reviews and specs | Reviews across a pull request's contract changes, sharing a spec outside the repo, notifications | C5, local Phase 13 |

## Before any code

1. Decide C-1 to C-11 (some can be "later" on purpose).
2. Register `mockmachina.dev` and its DNS, if not done already.
3. Write a threat model for C-6 to C-8: hosted mocks take traffic from the internet.
4. Agree on the free plan's limits, so C2 can enforce them from the start.
5. Finish local Phases 10 and 13 first where the cloud reuses them: the hosted dashboard is the local one, and teams come from `team.yaml`.
