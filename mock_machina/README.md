# MockMachina

MockMachina serves mock APIs from contract files that live in your repository, so frontend and mobile developers can build against an API before it exists and switch between success, empty and error responses without changing app code. The same files are the contract backend and frontend agree on in pull requests.

> **Status: pre-release.** It serves HTTP mocks with switchable states, rules, fake data, faults and hot reload; the first tagged release is v0.1.

## Install

```sh
brew install demola234/tap/mockmachina                                                       # macOS and Linux
scoop bucket add demola234 https://github.com/demola234/scoop-bucket && scoop install mockmachina  # Windows
docker run --rm -p 4001:4001 -v "$PWD/.mockmachina:/mock/.mockmachina" ghcr.io/demola234/mockmachina
```

[Installing](docs/install.md) has the install script, `go install` and more on Docker.

## Try it

```sh
cd examples/storefront
mockmachina                                         # pick a command from a list
mockmachina start                                   # the live screen: routes, states and requests
curl -H 'X-Mock-State: slow' localhost:4001/products # pick a state per request
mockmachina state set products.list server_error    # change the default; the running server picks it up
mockmachina lint                                    # check every file without serving
```

## Build from source

Requires Go 1.25 or newer.

```sh
git clone https://github.com/demola234/tiny-tools.git
cd tiny-tools/mock_machina
go build -o bin/ ./cmd/mockmachina
```

## Documentation

- [Quickstart](docs/quickstart.md): from nothing to an app talking to a mock in two minutes
- [Installing](docs/install.md): Homebrew, Scoop, the install script, Go and Docker
- [Mobile setup](docs/mobile-setup.md): emulators, simulators, phones and Flutter web
- [HTTPS](docs/https.md): a local certificate authority your devices trust, or your own certificate
- [File format](docs/file-format.md): everything you can put in `.mockmachina/`
- [Comparing contracts](docs/diff.md): breaking changes between git versions, or against a running API
- [Schemas and contracts](docs/contracts.md): check bodies and requests against JSON Schemas, and generate bodies
- [Import and export](docs/import-export.md): OpenAPI, Swagger and Postman in and out, and Swagger UI docs
- [Working as a team](docs/collaboration.md): review contract changes in pull requests, check the real backend, and proxy to it while it's half built
- [Using an AI assistant](docs/ai.md): let Claude, Cursor, Copilot or ChatGPT read the contract and add the states your app is missing
- [Storefront example](examples/storefront/): every feature in one project, with `curl` to try each one
- [Flutter example](examples/flutter_shop/): a screen for every state, tested against the mock
- [Engineering standards](docs/engineering.md): how the code is written
- [Decision records](docs/decisions/): why it's built this way
- [Contributing](CONTRIBUTING.md)

## Licence

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
