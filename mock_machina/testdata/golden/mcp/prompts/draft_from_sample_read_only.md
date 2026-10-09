Draft a MockMachina route for GET /orders/{id} from this real response:

{"id":"o_1"}

1. Call list_routes to see how the existing routes are named and which resource this belongs to.
2. Use the response as the first state, named for what it shows, such as success, found or created. Keep its fields and their order, and replace personal data with realistic fake values.
3. Add the error state an app most needs for this route, such as not_found for a path with an {id}, or invalid for a POST.
4. This session can't change the contract. Show the person the YAML for the route instead.
