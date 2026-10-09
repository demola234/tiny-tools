# Flutter Shop example

A Users screen that renders every state a real app has to handle, fed by MockMachina:

| State | Screen |
| --- | --- |
| loading | a spinner |
| `success` | the list of users |
| `empty` | "No users yet" |
| `unauthorized` (401) | "Your session expired", with "Sign in again" |
| `server_error` (500) | "Couldn't load users", with "Retry" |
| `slow` (2 s latency) | the spinner, for long enough to see it |

Debug builds show a `mock: <state>` badge in the app bar, read from the `X-Mock-State` response header.

It also covers flows that need memory, from `.mockmachina/routes/session.yaml` and `cart.yaml`:

| Route | Behaviour |
| --- | --- |
| `POST /session` | A rule refuses any password but `secret` with a 401; signing in sets `signed_in` and returns a UUID token |
| `CRUD /cart/items` | A real in-memory cart, starting from `.mockmachina/data/items.json`: add, update and remove items and see them in the next list |
| `POST /checkout` | Drops the connection (`fault: reset`), so the app's "connection lost" handling gets tested. `X-Mock-State: placed` places the order |

## Run it

From this folder, in one terminal:

```sh
mockmachina start
```

In another:

```sh
flutter run                                                    # iOS Simulator, desktop or Chrome
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:4001   # Android Emulator
```

Then switch screens without touching the code: run `mockmachina state set users.list empty` (or `unauthorized`, `server_error`, `slow`), then tap Retry on an error screen, or press `R` in the `flutter run` terminal for a hot restart. See [mobile setup](../../docs/mobile-setup.md) for phones and other devices. This example already has the Android and iOS settings it needs.

## Test it

The widget tests start the real `mockmachina` binary on a free port with this folder's `.mockmachina/`. They check that each state renders the right screen, and that signing in, the cart and a dropped checkout behave as above:

```sh
cd ../.. && go build -o bin/ ./cmd/mockmachina && cd examples/flutter_shop
flutter test
```

Set `MOCKMACHINA` to use a binary elsewhere. From the repository root, `just example` builds and tests in one step, and CI runs it on every change.

Two details make real network calls work inside Flutter widget tests:

- The fetch runs inside `tester.runAsync` and `HttpOverrides.runWithHttpOverrides(..., HttpOverrides())`. Widget tests use simulated time and a fake HTTP client by default.
- The test keeps reading the server's output. If it stopped, the server would be stopped (SIGPIPE) the next time it wrote a log line.
