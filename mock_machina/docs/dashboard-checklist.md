# Dashboard browser checklist

The Go tests check every page the server renders and every action it takes. They can't run htmx or the dashboard's own script in a browser, so this checklist covers that part. Run it before a release that changes anything under `internal/control/ui/`.

Set up with the storefront example, which has every kind of route, a CRUD collection and three scenarios:

```sh
cd examples/storefront
mockmachina start
```

Open `http://localhost:4001/__mockmachina/ui/` and work through the list in each browser. Then do it again in the other theme (the "Toggle theme" button, or your system setting) and at phone width (about 400 pixels wide, or a phone on your Wi-Fi).

| # | Check | Chrome | Safari | Firefox |
| --- | --- | --- | --- | --- |
| 1 | Overview shows 14 routes, the request and error counts, coverage and "none" for the scenario | | | |
| 2 | Routes: typing in the filter narrows the list; the protocol tabs show only CRUD, WebSocket or SSE routes | | | |
| 3 | Clicking a state chip pins it; `curl localhost:4001/checkout -X POST` then answers with that state | ✓ | | |
| 4 | "Remove pin" goes back to the route's own state | | | |
| 5 | "Make … the default" changes the file's `active` line (`git diff`), and the pin disappears | | | |
| 6 | Try it's copy button copies a working curl command |  | | |
| 7 | Traffic: a `curl` shows up at the top within a second, with its duration; each filter shows only its rows |  | | |
| 8 | Pause holds rows back and counts them; resume fills them in; "Clear view" empties the list | ✓ | | |
| 9 | Data lists `cart_items`; deleting an item removes it from `curl localhost:4001/cart/items` | | | |
| 10 | Scenarios: applying `checkout-fails` pins its routes, and Overview names it | | | |
| 11 | Coverage ticks a state after a request reaches it; "Not fully served" hides finished routes | | | |
| 12 | Reset clears pins, the scenario, coverage, counts and data | | | |
| 13 | Saving a routes or scenario file updates the page without reloading it | | | |
| 14 | Stopping the mock shows "disconnected, retrying"; starting it again shows "live" | | | |
| 15 | <kbd>⌘K</kbd> finds a route and <kbd>↵</kbd> opens it; <kbd>g</kbd> <kbd>t</kbd>, <kbd>p</kbd> and <kbd>?</kbd> work | ✓ | | |
| 16 | Everything above works with the keyboard alone | | | |
| 17 | The browser console shows no errors and no Content Security Policy violations | ✓ | | |
| 18 | From another device, with `start --host 0.0.0.0`: the sign-in page refuses a wrong token, accepts the right one, and the dashboard works | ✓ | | |

The ✓ marks are from headless Chrome driven over the DevTools protocol on 2026-10-15.
