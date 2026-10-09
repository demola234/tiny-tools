Look at the route users.get in this MockMachina contract and suggest the states apps still need to handle.

1. Call get_route with id users.get. Call lint too.
2. For each route, think about what a real app has to handle: the errors the API can return (401 when a session expires, 403, 404 for an unknown id, 422 for invalid input, 500), an empty list, and a slow response. Skip states the route already has.
3. Propose each missing state with a name, a status and a realistic body in the style of the route's existing bodies. Say in one line why an app needs it.
4. Ask the person which ones to add, then call add_state for each. New states are marked generated: true until the person reviews them.
