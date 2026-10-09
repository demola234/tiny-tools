# Comparing contracts

`mockmachina diff` shows how a contract changed and whether that change breaks apps. It compares two things:

- **Two versions of the contract in git:** what a branch or pull request changes.
- **The contract and a running API:** whether the backend actually returns what the contract promises.

Both give the same kind of report, in the same formats, and both can fail a CI job.

## Between git versions

```text
$ mockmachina diff main
4 changes between main and the working tree: 2 breaking, 1 warning, 1 safe
breaking  users.get: now answers GET /members/{id}, was GET /users/{id}
breaking  users.list: state "server_error" removed
warning   users.create: state "created" now returns 200, was 201
safe      users.export: route added (GET /users/export)
```

| You run | It compares |
| --- | --- |
| `mockmachina diff` | `HEAD` with your uncommitted changes |
| `mockmachina diff main` | `main` with your working tree |
| `mockmachina diff main..HEAD` | Two commits, branches or tags |

A ref from before the contract existed counts as an empty contract, so everything shows as added.

### What counts as breaking

| Change | Severity |
| --- | --- |
| Route removed, or its method or path changed | breaking |
| State removed | breaking |
| A state's status changes class, like 200 to 404 | breaking |
| A state's content type changes, or its body is removed | breaking |
| A state's status changes within a class, like 201 to 200 | warning |
| Route renamed (same method and path, new name) | warning |
| Default state, owners, status or summary changed | info |
| Route or state added | safe |

Renaming a path parameter (`{id}` to `{userId}`) isn't a change: apps can't tell.

## Against a running API

```text
$ mockmachina diff --live https://staging.example.com --header "Authorization: Bearer $TOKEN"
6 differences between the contract and https://staging.example.com: 3 breaking, 3 info
breaking  users.get: live API returned 410; no state returns it (states return 200, 404)
breaking  users.list: state "success": field "nextPage" is missing live
breaking  users.list: state "success": field "users[].id" is a number live, a string in the contract
info      users.create: not checked: POST isn't sent to a live API without --include-writes
info      users.delete: not checked: DELETE isn't sent to a live API without --include-writes
info      users.list: state "success": live response has extra field "users[].avatar"
```

For each route it calls the API, finds the state with the same status (preferring the active one), and checks:

- **The status:** a status no state returns is breaking.
- **The content type.**
- **The headers the state declares.**
- **The shape of the JSON body,** using the state's example body as the expected shape:
  - a field the contract has but the live response lacks is **breaking**, since apps read it;
  - a field whose type changed is **breaking**;
  - a field that's `null` live is a **warning**;
  - extra fields in the live response are **info**;
  - in lists, the first item of each is compared.

### Staying safe

- **Only `GET`, `HEAD` and `OPTIONS` are sent by default.** Routes with other methods are listed as "not checked". Add `--include-writes` only for environments where creating or deleting data is fine.
- **Requests go to the URL you give,** with the headers you pass. Nothing else is sent: no `X-Mock-State`, no cookies.

### Path parameters

Routes like `GET /users/{id}` need a real value. Put one in the route:

```yaml
get:
  route: GET /users/{id}
  examples: { id: u_1 }
  states:
    found:
      body: { id: u_1, name: Ada }
```

Or pass it on the command line, which overrides `examples`: `--param id=u_1`. Routes with a parameter that has no value are listed as "not checked".

### Other options

| Flag | What it does |
| --- | --- |
| `--header "Name: value"` | Header to send, such as auth. Repeatable |
| `--param name=value` | Path parameter value. Repeatable |
| `--include-writes` | Also send `POST`, `PUT`, `PATCH` and `DELETE` |
| `--timeout 5s` | Time limit per request (default 10s) |

## Output formats

| `--format` | For |
| --- | --- |
| `text` (default) | People in a terminal |
| `json` | Scripts: `{base, head, changes: [{severity, route, state, message, file, line}]}` |
| `markdown` | A pull-request comment: a short summary and a table |
| `github` | GitHub Actions: each change becomes an annotation on the route's file and line in the pull request |

## In CI

`--fail-on breaking` exits 1 when there's a breaking change; `--fail-on warning` also fails on warnings.

```yaml
- run: mockmachina diff origin/${{ github.base_ref }}..HEAD --format github --fail-on breaking
- run: mockmachina diff --live https://staging.example.com --header "Authorization: Bearer ${{ secrets.STAGING_TOKEN }}" --fail-on breaking
```

The first catches pull requests that break the contract. The second catches the backend drifting away from it. For pull requests, the [GitHub Action](collaboration.md#review-changes-in-pull-requests) does the first, and adds a comment that mentions each route's owners.

With `--format markdown` and `--format json`, each change also lists its route's owners.
