# GraphQL in GoFr

GoFr provides a **Schema-First** approach to building GraphQL APIs. This means you define your API contract in a standard GraphQL schema file, and GoFr handles the execution, validation, and observability.

## Required Setup

To enable GraphQL, you MUST provide a schema file at the following location:
`./configs/schema.graphqls`

If this file is missing or invalid, GoFr will log an error and return a `500 Internal Server Error` when the `/graphql` endpoint is accessed.

## Core Concepts

### 1. [Query](https://graphql.org/learn/queries/)
Queries are used to fetch data. In GoFr, a Query resolver is a function that takes `*gofr.Context` and returns a data object (or `any`) and an error.

### 2. [Mutation](https://graphql.org/learn/queries/#mutations)
Mutations are used to modify data. They follow the same signature as Queries but are intended for side effects.

### 3. Automatic Health Check
GoFr automatically injects a `gofr` field into your root `Query` type (if not already present). This allows you to check the application health via GraphQL:
```graphql
query {
    gofr {
        status
        name
        version
    }
}
```

---

## The Unified Schema

GoFr aggregates every `GraphQLQuery` and `GraphQLMutation` you register and validates them against your `./configs/schema.graphqls`. The API is served at `/graphql`.

*   **Single Endpoint**: All operations go through `/graphql`.
*   **Playground**: Interactive documentation and testing at `/graphql/ui` (enabled in non-production).

---

## Getting Started

### 1. Define your Schema
Create `configs/schema.graphqls`:
```graphql
type User {
    id: Int
    name: String
}

type Query {
    user(id: Int): User
}
```

### 2. Register Resolvers
In GoFr, resolvers strictly take `*gofr.Context`. You use `c.Bind()` to extract arguments.

```go
func main() {
    app := gofr.New()

    app.GraphQLQuery("user", func(c *gofr.Context) (any, error) {
        // Extract arguments manually
        var args struct {
            ID int `json:"id"`
        }
        _ = c.Bind(&args)

        // Return 'any' - GoFr validates this against the schema at runtime
        return map[string]any{
            "id": args.ID,
            "name": "Antigravity",
        }, nil
    })

    app.Run()
}
```

---

## Schema-First Features

### 1. Returns `any`
Unlike standard HTTP handlers which allow `any` but lose structure, GraphQL handlers in GoFr return `any` while **maintaining the contract** defined in your `.graphqls` file. 
- GoFr validates the returned object against the schema.
- If the object does not match, GoFr returns an error with the partial data.

### 2. Custom Status Codes
GoFr is opinionated about status codes in GraphQL:
- **200 OK**: Request succeeded and data matches the schema.
- **422 Unprocessable Entity**: The data returned by your handler does not match the schema, or there was a validation error.

### 3. Argument Binding
Instead of declarative arguments in the function signature, you use the standard `c.Bind()` method. GoFr automatically maps the GraphQL `args` map to your struct using JSON tags.

---

## Testing Your GraphQL API

### 1. Interactive Exploration
GoFr automatically hosts a **GraphQL Playground** at `/graphql/ui` when running in non-production environments. This is guarded by the `APP_ENV` configuration; the UI will return a `404 Not Found` if `APP_ENV` is set to `production`.

### 2. Standard POST Requests
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"query": "{ user(id: 1) { name } }"}' \
  http://localhost:9091/graphql
```

---

## Observability

GoFr provides production-grade observability for GraphQL out of the box.

### 1. Tracing
GoFr automatically instruments your GraphQL API with OpenTelemetry traces:
- **Root Span**: Every request generates a `graphql-request` span.
- **Resolver Spans**: Each individual resolver call generates a nested span (e.g., `graphql-resolver-user`), allowing you to see the exact time spent in each field's business logic.
- **Attributes**: The `graphql.operation_name` and `graphql.operation_type` (query/mutation) are automatically added to the spans.

### 2. Metrics
GoFr exports several GraphQL-specific metrics:

- **`gofr_graphql_operations_total`**: Total number of GraphQL operations, tagged by `operation_name` and `type` (query/mutation).
- **`gofr_graphql_error_total`**: Total operations that resulted in an error, tagged by `operation_name` and `type`.
- **`gofr_graphql_request_duration`**: Histogram of the entire request lifecycle, tagged by `operation_name` and `type`.

---

## Best Practices

1.  **Keep Schema and Logic in sync**: Since the schema is defined in a separate file, ensure field names in your Go maps/structs match the field names in `schema.graphqls`.
2.  **Use c.Bind()**: Always use `c.Bind()` for accessing arguments to benefit from GoFr's internal mapping and validation.
3.  **Error Handling**: Return errors from your handlers. GoFr will include them in the `errors` array of the GraphQL response.
