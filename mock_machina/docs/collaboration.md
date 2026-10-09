# Working on a contract as a team

The contract in `.mockmachina/` is shared between the people who build the app and the people who build the API. This guide covers how teams agree on it, review changes to it, and keep the real backend in line with it.

## The lifecycle of a route

Each route has a `status` that says how settled it is:

| Status | Means | Usually set by |
| --- | --- | --- |
| `draft` | Proposed; may still change. The default | Whoever adds the route, often the app team |
| `agreed` | Both sides have reviewed it; the backend builds to it | The backend owner, in review |
| `implemented` | The real API returns this | The backend owner, once it ships |
| `deprecated` | Still served, but on its way out | The backend owner |

A typical flow:

1. **The app team drafts a route** with `mockmachina add`, describes the states the screen needs, and builds against the mock straight away.
2. **They open a pull request.** The [GitHub Action](#review-changes-in-pull-requests) comments with the contract changes and mentions each route's owners.
3. **The backend owners review it** like any code. When it's settled, they change `status: draft` to `status: agreed` in the same pull request.
4. **The backend ships it,** and the route becomes `status: implemented`. From then on, [`diff --live`](#keep-the-backend-honest) checks that the real API still matches.

Set `owners` once at the top of each routes file, and override it per route where needed:

```yaml
owners: { backend: [ademola], frontend: [ada] }
status: agreed

list:
  route: GET /users
  states: { ... }
```

Owners are GitHub handles. They're mentioned in pull-request comments, so the right people get asked.

## Review changes in pull requests

Add the action to a workflow in your repository:

```yaml
name: API contract
on:
  pull_request:
    types: [opened, synchronize, reopened, labeled, unlabeled]

permissions:
  contents: read
  pull-requests: write

jobs:
  contract:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: demola234/tiny-tools/mock_machina@mock_machina/v0.3.0
        with:
          dir: .mockmachina
```

On every pull request it:

- **Runs `lint`.** A broken file fails the job.
- **Annotates the changed lines.** Each change shows on the route's line in the "Files changed" tab: breaking changes as errors, warnings as warnings, the rest as notes.
- **Comments once and keeps the comment current.** You get a table of the changes, with owners and a list of whom to ask. It's updated on every push rather than posting again.
- **Fails on breaking changes.** These are things that make apps built on the old contract fail: a removed route or state, a changed path, a status that moves from 2xx to 4xx, a changed content type. See [what counts as breaking](diff.md#what-counts-as-breaking).

**When a breaking change is intended,** add the `breaking-change` label to the pull request. The job passes, and the comment still lists the change. The `labeled` trigger above makes the job re-run when you add the label.

| Input | Default | What it does |
| --- | --- | --- |
| `dir` | `.mockmachina` | The project folder, relative to the repository root |
| `override-label` | `breaking-change` | The label that lets breaking changes through |
| `comment` | `true` | Set to `false` to rely on annotations and the job summary only |
| `github-token` | the job's token | Needs `pull-requests: write` to comment |

`fetch-depth: 0` is needed so the action can find where the pull request's branch started. Pull requests from forks can't be commented on with the default token. The action warns instead of failing, and the report is still in the job summary.

## Keep the backend honest

Once routes are `implemented`, check the real API against them on a schedule:

```yaml
on:
  schedule:
    - cron: "0 6 * * 1-5"
jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: stable }
      - run: go install github.com/demola234/tiny-tools/mock_machina/cmd/mockmachina@latest
      - run: mockmachina diff --live https://staging.example.com --header "Authorization: Bearer ${{ secrets.STAGING_TOKEN }}" --fail-on breaking
```

It only sends `GET`, `HEAD` and `OPTIONS`, so it's safe against a shared environment. See [comparing against a running API](diff.md#against-a-running-api).

## When the backend is half built

Point the mock at the real backend, and it fills in whatever the contract doesn't cover:

```sh
mockmachina start --proxy http://localhost:8080
```

| Request | Goes to |
| --- | --- |
| Matches a route | The mock, as usual |
| Matches no route | The real backend |
| Matches a route with `serve: proxy` | The real backend, unless the request asks for a state with `X-Mock-State` or `?__state=` |

This lets the app run against real endpoints as they ship, route by route: mark a route `serve: proxy` when the backend has it. App tests can still force error screens on those routes by asking for a state. Requests sent on to the backend don't carry `X-Mock-State`, and if the backend is down, the mock answers `502` saying so.

Put `proxy: http://localhost:8080` in `config.yaml` to make it the default for everyone. `--proxy` overrides it.

## With an AI assistant

Assistants connected through [`mockmachina mcp`](ai.md) can draft states and explain diffs. Anything they write is marked `generated: true` and, for new routes, `status: draft`, so it goes through the same review as everything else.
