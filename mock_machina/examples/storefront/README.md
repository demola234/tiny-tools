# Storefront: every MockMachina feature in one project

A small Nigerian grocery API: sign-in, a catalog, a cart, checkout and order tracking. Each route shows a different MockMachina feature, and `internal/cli/example_storefront_test.go` serves this folder and checks every response below, so the example stays correct.

```sh
cd examples/storefront
mockmachina                # pick a command from the launcher
mockmachina start          # or serve it directly on http://127.0.0.1:4001
mockmachina tui            # or serve it with the live screen
```

## What's inside

```text
.mockmachina/
├── config.yaml        seed 42 and Nigerian fake data (locale: en_NG)
├── schemas/           Credentials, Session, Me, Product, ProductList, NewOrder, Order, OrderList, Tracking, Error
├── data/
│   └── cart_items.json   the cart's starting contents
└── routes/
    ├── health.yaml    sequential mode
    ├── session.yaml   request validation, rules, variables, templates
    ├── products.yaml  body files, query rules, latency, random mode
    ├── cart.yaml      CRUD, call-count rules, faults
    └── orders.yaml    header rules, generated bodies, stage-by-stage tracking
```

| Route | Feature it shows |
| --- | --- |
| `GET /health` | `mode: sequential`: the first call gets 503 while the API "starts", then 200 |
| `POST /session` | A request schema (a bad email gets a 400), a rule on the body (the password is `secret`), and `set` to remember who signed in |
| `GET /me` | A rule on a variable (401 until you sign in) and `fake.*` data from the `en_NG` locale |
| `DELETE /session` | Clearing variables with `null` |
| `GET /products` | A body file, a query schema (`?page=two` is a 400), a rule on `query.page`, and a `slow` state with latency and jitter |
| `GET /products/{id}` | A rule on the path (`p_404`), a templated body, and a cache header |
| `GET /deals/today` | `mode: random`: a seeded pick each call, including a 404 |
| `/cart/items` | `CRUD`: list, add, update and remove, starting from `data/cart_items.json` |
| `POST /checkout` | `call` rules (the first call drops the connection, the second is busy, the third works), a response schema, and a `truncated` fault with a rate |
| `GET /orders` | A header rule: no `Authorization` header gives a 401 |
| `GET /orders/{id}` | `body: generate`, built from the `Order` schema |
| `GET /orders/{id}/tracking` | `call` rules that move the order on one stage per call |

Every file has `owners` and a contract `status` (draft, agreed or implemented), so `diff` can route reviews.

## Try it

With `mockmachina start` running, in another terminal:

```sh
curl localhost:4001/health                     # 503 starting
curl localhost:4001/health                     # 200 up

curl localhost:4001/me                         # 401: sign in first
curl -X POST localhost:4001/session -d '{"email":"ada@","password":"secret"}'
                                               # 400: "ada@" isn't a valid email
curl -X POST localhost:4001/session -d '{"email":"ada@example.com","password":"secret"}'
curl localhost:4001/me                         # 200: ada@example.com, with a Nigerian name, phone and city

curl localhost:4001/products                   # the catalog from routes/products/products.json
curl 'localhost:4001/products?page=3'          # an empty last page, picked by a rule
curl localhost:4001/products/p_404             # 404

curl localhost:4001/cart/items
curl -X POST localhost:4001/cart/items -d '{"sku":"p_garri","qty":1}'
curl -X PATCH localhost:4001/cart/items/c_1 -d '{"qty":3}'

curl -X POST localhost:4001/checkout -d '{"items":[{"sku":"p_zobo","qty":3}]}'
                                               # 1st: connection reset, 2nd: 503, 3rd: 201

curl -H 'Authorization: Bearer t_1' localhost:4001/orders/o_1001/tracking
                                               # packed, in_transit, out_for_delivery, then delivered
```

The full set of requests and answers is in `testdata/golden/examples/storefront.txt`.

## Switch states

Any request can ask for a state:

```sh
curl -H 'X-Mock-State: slow' localhost:4001/products      # waits about 2 seconds
curl 'localhost:4001/cart/items?__state=down'             # 503 cart unavailable
```

To change what everyone gets, set the route's default. The file changes and the running server picks it up:

```sh
mockmachina state set products.list server_error
```

Or press ⏎ on a state in `mockmachina tui`, or choose **state set** in the launcher.

## Replay a run

`config.yaml` sets `seed: 42`, so names, ids, random picks and fault rates are the same on every start. Use `mockmachina start --seed 7` for a different run that you can replay with the same seed.

## Use it from an app

- `mockmachina start --host 0.0.0.0` lets a phone on your Wi-Fi connect; see `docs/mobile-setup.md`.
- `mockmachina start --https` serves trusted HTTPS for iOS and Android; run `mockmachina cert --install` once.
- `mockmachina export -o openapi.yaml` writes the contract as OpenAPI 3.1, and `mockmachina docs --serve` opens it in Swagger UI.
- `mockmachina mcp --print-config claude-code` lets an AI assistant read and edit these routes.
