# Control API

A running mock answers a small HTTP API under `/__mockmachina/` on its own port. Tests and scripts use it to see the routes, switch a route's state, put everything back the way it started, and watch requests as they happen. Nothing extra to start or configure: if `mockmachina start` serves `http://localhost:4001`, the API is at `http://localhost:4001/__mockmachina/`.

| Request | What it does |
| --- | --- |
| `GET /__mockmachina/routes` | Every route: id, method, path, mode, active state, state names, and its pin if it has one |
| `GET /__mockmachina/routes/{id}` | One route |
| `PUT /__mockmachina/routes/{id}/state` | Pin the route to a state: `{"state": "empty"}` |
| `DELETE /__mockmachina/routes/{id}/state` | Remove the pin |
| `POST /__mockmachina/reset` | Remove every pin and the applied scenario, and clear call counts, variables, CRUD collections, the request history and coverage |
| `GET /__mockmachina/requests` | The newest 500 requests, newest first, with how many requests and errors there have been since start or reset |
| `GET /__mockmachina/coverage` | For each route, the states the mock has served since start or reset, and the ones it hasn't |
| `GET /__mockmachina/collections` | Every CRUD collection: its id field, seed file, routes and how many items it holds now |
| `GET /__mockmachina/collections/{name}` | One collection, with its items |
| `DELETE /__mockmachina/collections/{name}/{id}` | Remove one item, as `DELETE` on the CRUD route would |
| `GET /__mockmachina/scenarios` | Every scenario with its states, and which one was applied last |
| `POST /__mockmachina/scenarios/{name}/apply` | Put the whole mock into a scenario |
| `GET /__mockmachina/events` | A server-sent event stream: one `request` event per request the mock answers, and a `reload` event when the project changes |

Route paths under `/__mockmachina/` are reserved; `lint` and `start` report one as a problem.

The [dashboard](dashboard.md) at `/__mockmachina/ui/` does all of this from a browser.

## Switching states

```sh
curl -X PUT localhost:4001/__mockmachina/routes/users.list/state -d '{"state": "empty"}'
```

```json
{"id":"users.list","method":"GET","path":"/users","mode":"active","active":"success","states":["success","empty","unauthorized"],"pinned":"empty"}
```

A pin lives in the running server's memory. It survives hot reloads and lasts until you remove it, call reset or stop the server, and it never touches your files, so `git status` stays clean while a test suite flips states.

For each request, a pin comes after `X-Mock-State`, `?__state=` and `X-Mock-Scenario`, and before rules, modes and `active`. One request can still ask for a different state.

To change the default in the routes file instead, as `mockmachina state set` does, add `"persist": true`:

```sh
curl -X PUT localhost:4001/__mockmachina/routes/users.list/state -d '{"state": "empty", "persist": true}'
```

The file's `active` line changes, the server reloads it, and any pin on that route is removed.

## Errors

Errors are JSON, with the closest match when a name is mistyped:

```json
{"error":"unknown_state","route":"users.list","state":"emty","valid":["success","empty","unauthorized"],"hint":"did you mean \"empty\"?"}
```

| Status | `error` | When |
| --- | --- | --- |
| 400 | `bad_request` | The body isn't `{"state": "<name>"}` |
| 400 | `unknown_state` | The route has no such state |
| 401 | `forbidden` | Another machine sent no token, or the wrong one |
| 403 | `forbidden` | A browser page from another site, or another machine when no token is set |
| 404 | `unknown_route` | No route has that id |
| 404 | `unknown_collection` | No CRUD route uses that collection |
| 404 | `unknown_item` | The collection has no item with that id |
| 404 | `unknown_scenario` | No scenario file has that name |
| 404 | `not_found` | No such endpoint; the body lists the endpoints |
| 409 | `not_saved` | `persist` couldn't write the file, for example because it has problems |

## Live events

```sh
curl -N localhost:4001/__mockmachina/events
```

```text
event: request
data: {"time":"2026-10-14T09:12:03.512Z","method":"GET","path":"/users","status":200,"route":"users.list","state":"empty","reason":"control","duration_ms":3}
```

`duration_ms` is how long the mock took, from receiving the request to finishing the response, including any `latency`. For a WebSocket or SSE stream it's the time to open the stream, and for a webhook it's the time the call to your URL took.

`reason` says why the state was chosen: `header`, `query`, `scenario` (the `X-Mock-Scenario` header), `control` (a pin), `rule` (with `rule`, its number), `sequential`, `random` or `active`. Other fields appear when they apply: `grpc_code`, `problem`, `suggestion`, `fault`, `detail`, `proxied` and `webhook`. HTTP, WebSocket, SSE, GraphQL, gRPC and webhook requests all show up.

When a routes file is saved and the project reloads, the stream sends:

```text
event: reload
data: {}
```

Fetch `routes` again to see the change. Pins survive a reload.

The stream never slows the mock down. A listener that falls more than 64 events behind misses events until it catches up.

## Request history

```sh
curl localhost:4001/__mockmachina/requests
```

```json
{"total":42,"errors":3,"requests":[{"time":"2026-10-14T09:12:03.512Z","method":"GET","path":"/users","status":200,"route":"users.list","state":"empty","reason":"control","duration_ms":3}]}
```

`requests` holds the newest 500, newest first, in the same shape as the events. `total` and `errors` count every request since the mock started or was reset, including ones that have dropped out of the list. A request counts as an error when its status is 400 or above, its gRPC code isn't `OK`, a fault was injected, or the mock reported a problem with it, such as no matching route or a client that left early.

## Coverage

```sh
curl localhost:4001/__mockmachina/coverage
```

```json
{"served":4,"total":12,"percent":33,"routes":[{"id":"users.list","method":"GET","path":"/users","served":["success","empty"],"unserved":["unauthorized"]}]}
```

Coverage answers "which screens haven't we seen yet?" A state counts once the mock has answered a request with it, whatever picked it: a header, a pin, a rule, a mode or `active`. It doesn't count when the request named a state the route doesn't have, failed validation, or left before the response. Webhooks and proxied requests don't count. `percent` is rounded down.

Coverage measures the mock's answers, not your app's code. Reset starts it again.

## CRUD collections

```sh
curl localhost:4001/__mockmachina/collections/cart
```

```json
{"name":"cart","id_field":"id","seed":"data/cart.json","routes":["CRUD /cart"],"count":2,"items":[{"id":"c_1","sku":"TEE"},{"id":"c_2","sku":"SCARF"}]}
```

These show what the running mock holds, including whatever your app has added, changed or removed. `seed` appears when the collection started from a `data/` file. Deleting an item answers `204`, or `404` with `unknown_item`. A collection's name comes from its routes, so you can see it before the app makes its first request. Reset puts every collection back to its seed.

## Scenarios

```sh
curl -X POST localhost:4001/__mockmachina/scenarios/checkout-fails/apply
```

```json
{"applied":"checkout-fails","scenarios":[{"name":"checkout-fails","title":"Checkout fails","states":[{"route":"payments.create","state":"declined"},{"route":"orders.list","state":"success"}]}]}
```

Applying a [scenario](file-format.md#scenarios) removes every pin, then pins each route the scenario names to its state. Routes it doesn't name answer as they normally would. `applied` names the last scenario applied until reset; pinning or unpinning a route afterwards doesn't clear it. An unknown name answers `404` with `unknown_scenario` and the closest name.

To apply a scenario to one request only, without changing the server, send the `X-Mock-Scenario` header instead.

## Who can use it

Requests from the same machine to `localhost`, `127.0.0.1` or `[::1]` need nothing. That covers tests, scripts, the iOS Simulator, and the Android emulator through `adb reverse`.

The address matters as well as the machine. A website can point its own name at `127.0.0.1` and send requests that come from your machine, so a request that names any other host needs the token too.

Requests from other machines need `Authorization: Bearer <token>`. When `--host` makes the mock reachable from the network, `start` prints the token:

```text
control API on http://0.0.0.0:4001/__mockmachina/; other machines send "Authorization: Bearer 6QXK2G…"
```

Set `MOCKMACHINA_CONTROL_TOKEN` to choose the token yourself; `start` then prints the variable's name instead of its value. In Docker, requests from the host come from the container network, not loopback, so they need the token too:

```sh
docker run -e MOCKMACHINA_CONTROL_TOKEN=secret -p 4001:4001 -v "$PWD/.mockmachina:/mock/.mockmachina" ghcr.io/demola234/mockmachina
curl -H 'Authorization: Bearer secret' localhost:4001/__mockmachina/routes
```

A browser page from another site can't use the API, even on your machine. Any request whose `Origin` doesn't match the mock's own address is refused.

## In Flutter tests

Pin a state before a test and reset after it:

```dart
Future<void> pin(Uri mock, String route, String state) async {
  final client = HttpClient();
  final request = await client.putUrl(mock.resolve('/__mockmachina/routes/$route/state'));
  request.write(jsonEncode({'state': state}));
  final response = await request.close();
  await response.drain<void>();
  client.close();
  if (response.statusCode != 200) {
    throw StateError('pinning $route to $state: ${response.statusCode}');
  }
}

Future<void> reset(Uri mock) async {
  final client = HttpClient();
  await (await client.postUrl(mock.resolve('/__mockmachina/reset'))).close();
  client.close();
}

void main() {
  tearDown(() => reset(baseUrl));

  testWidgets('shows the empty message', (tester) async {
    await pin(baseUrl, 'users.list', 'empty');
    // pump the screen and expect the empty state
  });
}
```

A pin applies to every request, including ones the app makes on its own, so this works for flows where you can't add `X-Mock-State` to each request.
