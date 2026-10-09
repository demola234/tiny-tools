Check whether the API at {{.url}} still matches the MockMachina contract.

1. Call diff_live with url {{.url}}.
2. For each breaking difference, say which side is likely wrong and why: the backend (it changed without the contract) or the contract (it never matched what the backend returns).
3. Then list the routes that weren't checked and why: writes aren't sent, and path parameters need example values.
4. End with the changes to make, to the contract or to the backend, starting with the ones that break apps.
