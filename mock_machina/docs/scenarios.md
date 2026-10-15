# Scenarios

A scenario names a situation across many routes, like "new shopper" or "checkout fails", so you can put the whole app into it at once instead of switching routes one by one. Scenarios live in your repository next to the routes, so the situations your team tests are reviewed and shared like the rest of the contract.

## Write one

Each file in `.mockmachina/scenarios/` is one scenario; its file name is the scenario's name:

```yaml
# .mockmachina/scenarios/checkout-fails.yaml
title: Checkout fails
description: The shop is too busy to take the order; browsing and the cart still work.
states:
  products.list: success
  cart.items: ok
  cart.checkout: busy
```

`states` maps route ids to the state each one answers with. Routes the scenario doesn't name answer as they normally would. `title` and `description` are optional and show up in the dashboard. The [file format](file-format.md#scenarios) has the details, and `scenario.schema.json` gives your editor autocomplete.

`mockmachina lint` and `start` check every route and state, and suggest the closest name when one is mistyped:

```text
scenarios/checkout-fails.yaml:7: route cart.checkout has no state "bussy" (did you mean "busy"?)
```

## Use one

There are three ways, depending on how long the situation should last.

**For one request**, send the `X-Mock-Scenario` header. Nothing on the server changes:

```sh
curl -H 'X-Mock-Scenario: checkout-fails' -X POST localhost:4001/checkout
```

gRPC clients send `x-mock-scenario` as metadata. `X-Mock-State` and `?__state=` still win for a single request.

**For the whole mock**, apply it. Every request, including the ones your app makes on its own, gets the scenario's states until you reset or apply another:

```sh
curl -X POST localhost:4001/__mockmachina/scenarios/checkout-fails/apply
```

Applying removes every pin first, then pins each route the scenario names. To go back, reset:

```sh
curl -X POST localhost:4001/__mockmachina/reset
```

**From the dashboard**, open Scenarios and press Apply. The applied scenario is marked there and on the Overview.

## Which wins

For each request the mock uses the first of these that applies:

1. the `X-Mock-State` header;
2. the `?__state=` query parameter;
3. the `X-Mock-Scenario` header, when the scenario names this route;
4. a pin, including the pins an applied scenario sets;
5. the route's rules, mode and `active` state.

## In tests

Apply a scenario before a test and reset after it. In a Flutter integration test:

```dart
Future<void> scenario(Uri mock, String name) async {
  final client = HttpClient();
  final request = await client.postUrl(mock.resolve('/__mockmachina/scenarios/$name/apply'));
  final response = await request.close();
  await response.drain<void>();
  client.close();
  if (response.statusCode != 200) {
    throw StateError('applying $name: ${response.statusCode}');
  }
}
```

`reset` is in the [control API](control-api.md#in-flutter-tests) guide. A name with no scenario file answers `404` with the closest name, so a typo fails the test instead of quietly testing the wrong thing.

## Try it

The [storefront example](../examples/storefront/README.md) has three: `new-shopper`, `checkout-fails` and `bad-network-day`.
