Draft a MockMachina route for GET /orders/{id} from this real response:

{"id":"o_1","total":12.5}

1. Call list_routes to see how the existing routes are named and which resource this belongs to.
2. Use the response as the first state, named for what it shows, such as success, found or created. Keep its fields and their order, and replace personal data with realistic fake values.
3. Add the error state an app most needs for this route, such as not_found for a path with an {id}, or invalid for a POST.
4. Show the person what you'll add, then call add_route. The route is marked generated: true and status: draft until they review it.
