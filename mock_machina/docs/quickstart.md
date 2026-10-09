# Quickstart

From nothing to an app talking to a mock API in about two minutes.

## Install

```sh
brew install demola234/tap/mockmachina    # or: go install github.com/demola234/tiny-tools/mock_machina/cmd/mockmachina@latest
mockmachina --version
```

[Installing](install.md) covers the install script, Windows and Docker.

## 1. Create a project

In your app's folder:

```text
$ mockmachina init
created .mockmachina/config.yaml
created .mockmachina/routes/health.yaml
next: mockmachina start, then open http://127.0.0.1:4001/health
```

## 2. Add routes

```text
$ mockmachina add GET /users --summary "List users"
created routes/users.yaml with users.list (GET /users)
$ mockmachina add GET /users/{id}
added users.get (GET /users/{id}) to routes/users.yaml
```

Then describe the responses in `.mockmachina/routes/users.yaml`. Each route has **states**, the different answers it can give:

```yaml
list:
  route: GET /users
  summary: List users
  states:
    success:
      body: { users: [{ id: u_1, name: Ada }] }
    empty:
      body: { users: [] }
    unauthorized:
      status: 401
      body: { error: session expired }
    slow:
      latency: 2s
      body: { users: [] }
```

The first state is the default. [The file format](file-format.md) covers everything else you can write.

## 3. Start it

```sh
mockmachina start
```

In a terminal this opens the live screen: your routes and their states on the left, and requests on the right as they arrive. Move with ↑↓, press enter on a state to make it the default, `/` to filter, tab to look at requests, `l` to list problems and `q` to quit.

For a plain log instead, one line per request, add `--plain`. That's also what you get in CI, Docker or a pipe:

```text
$ mockmachina start --plain
serving 3 routes from .mockmachina on http://127.0.0.1:4001 (Ctrl+C to stop)
GET /users 200 users.list:success (active)
```

Leave it running. Every save in `.mockmachina/` applies to the next request. If you save a broken file, it keeps serving the last good version and prints what's wrong.

## 4. Switch states

**For one request**, while testing or from a script:

```sh
curl -H 'X-Mock-State: empty' localhost:4001/users
curl 'localhost:4001/users?__state=unauthorized'      # handy in a browser
```

**For everyone**, while clicking through the app:

```text
$ mockmachina state list users.list
users.list  GET /users
* success       200
  empty         200
  unauthorized  401
  slow          200
$ mockmachina state set users.list empty
users.list: success → empty
```

`state set` changes one line in the route file, so the running server picks it up straight away, and the team sees it in `git status`.

Every response carries `X-Mock-State` and `X-Mock-Route` headers, so your network inspector shows which state you got.

## 5. Check before you push

```text
$ mockmachina lint
routes/health.yaml:2: warning: health.get has no owners; reviews can't be routed
routes/users.yaml:2: warning: users.list has no owners; reviews can't be routed
routes/users.yaml:17: warning: users.get has no summary; add one line saying what it returns
routes/users.yaml:17: warning: users.get has no 4xx or 5xx state; apps can't test errors
routes/users.yaml:17: warning: users.get has no owners; reviews can't be routed
5 problems in 2 files
```

Warnings are suggestions and don't fail the command. Setting `owners` once at the top of each routes file clears most of them. `lint` exits with an error when a file is broken, or on warnings too with `--strict`, so it can run in CI.

## When something doesn't line up

- **A 404 lists the closest routes**, so a typo like `/usrs` points you at `GET /users`.
- **An unknown state gets a 400** listing the valid ones, and a "did you mean".
- **A busy port gets a suggestion:** `port 4002 is free, try --port 4002`.
- **Typing a command wrong gets one too:** `mockmachina strat` asks if you meant `start`.
- **Not sure which command?** Run `mockmachina` on its own to pick one from a list. It asks for what the command needs and shows the full command before running it.

## Next

- [Mobile setup](mobile-setup.md): emulators, simulators, phones and Flutter web
- [HTTPS](https.md): a trusted certificate for every device, with no cleartext exceptions
- [The storefront example](../examples/storefront/): every feature in one project, with `curl` to try each one
- [The file format](file-format.md)
- [Comparing contracts](diff.md): breaking changes in a pull request, or between the contract and your real API
- [Import and export](import-export.md): start from an existing OpenAPI, Swagger or Postman file
- [Schemas and contracts](contracts.md): check bodies and requests against schemas
- [Working as a team](collaboration.md): pull-request reviews with the GitHub Action, and proxying to a half-built backend
- [Using an AI assistant](ai.md): let your assistant suggest and add the states your app is missing
- [The Flutter example](../examples/flutter_shop/): a screen that renders every state, with widget tests against the mock
