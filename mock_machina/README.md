# MockMachina

MockMachina serves mock APIs from contract files that live in your repository, so frontend and mobile developers can build against an API before it exists and switch between success, empty and error responses without changing app code. The same files are the contract backend and frontend agree on in pull requests.

> **Status: pre-release.** Phase 0 (foundations) is in progress. Nothing is ready to use yet; the first usable release is v0.1.

## Build from source

Requires Go 1.25 or newer.

```sh
git clone https://github.com/demola234/tiny-tools.git
cd tiny-tools/mock_machina
go build -o bin/ ./cmd/mockmachina
```

## Documentation

- [Engineering standards](docs/engineering.md): how the code is written
- [Decision records](docs/decisions/): why it's built this way
- [Contributing](CONTRIBUTING.md)

## Licence

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
