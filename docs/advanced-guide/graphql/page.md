# GraphQL in GoFr

GoFr provides a **Schema-First** approach to building GraphQL APIs. This means you define your API contract in a standard GraphQL schema file, and GoFr handles the execution, validation, and observability.

## Required Setup

To enable GraphQL, you MUST provide a schema file at the following location:
`./configs/schema.graphqls`

If this file is missing or invalid, GoFr will log a fatal error and the application will fail to start. This fail-fast behavior ensures schema issues are caught at deployment rather than runtime.

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
*   **Playground**: Interactive documentation and testing at `/graphql/ui`. The playground is only registered when `APP_ENV` is **not** set to `production`. In production, the `/graphql/ui` route does not exist and returns `404 Not Found`.

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
            "id":   args.ID,
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
- GoFr leverages the underlying `graphql-go` engine to validate the returned object against your defined schema.
- If the object does not match the schema types, GoFr returns an error in the `errors` array with partial data where applicable.

### 2. HTTP Status Codes

GoFr is opinionated about HTTP status codes in GraphQL responses. Unlike the standard GraphQL-over-HTTP spec (which always returns `200`), GoFr surfaces errors at the HTTP layer to make them visible to standard monitoring and alerting tools without requiring response body inspection.

| Status Code | Condition |
|---|---|
| `200 OK` | Query or mutation succeeded with no errors. |
| `400 Bad Request` | The request body is not valid JSON. |
| `422 Unprocessable Entity` | A resolver returned an error, or GraphQL validation failed (e.g. querying a field not in the schema). |

**Error response body** (for `422`):
```json
{
  "data": null,
  "errors": [
    {
      "message": "your error message here",
      "locations": [{ "line": 1, "column": 3 }],
      "path": ["fieldName"]
    }
  ]
}
```

### 3. Argument Binding
Instead of declarative arguments in the function signature, you use the standard `c.Bind()` method. GoFr automatically maps the GraphQL `args` map to your struct using JSON tags.

### 4. Unsupported Types
Currently, GraphQL `Enum` types are explicitly not supported in resolvers. If an Enum type is defined in the schema and a resolver attempts to map it, GoFr will return an error during schema initialization at startup.

---

## Testing Your GraphQL API

### 1. Interactive Exploration
GoFr automatically hosts a **GraphQL Playground** at `/graphql/ui` in all non-production environments. Set `APP_ENV=production` to disable it. The route is not registered in production — it will return `404 Not Found`.

### 2. Standard POST Requests

The `/graphql` endpoint accepts a JSON body with the following fields:

| Field | Type | Description |
|---|---|---|
| `query` | `string` | **Required.** The GraphQL query or mutation string. |
| `operationName` | `string` | Optional. The name of the operation to execute (used for metrics tagging). |
| `variables` | `object` | Optional. A map of variable values for the query. |

**Simple query:**
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"query": "{ user(id: 1) { name } }"}' \
  http://localhost:9091/graphql
```

**Named operation with variables:**
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"query": "query GetUser($id: Int) { user(id: $id) { name } }", "operationName": "GetUser", "variables": {"id": 1}}' \
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
GoFr exports several GraphQL-specific metrics, all tagged by `operation_name` and `type` (query/mutation):

- **`gofr_graphql_operations_total`**: Total number of GraphQL operations received.
- **`gofr_graphql_error_total`**: Total operations that resulted in an error (resolver error or validation failure). Incremented on any `422` response.
- **`gofr_graphql_request_duration`**: Histogram of the entire request lifecycle in seconds.

> **Note:** The `operation_name` tag is sourced from the `operationName` field in the POST body. For anonymous operations, it defaults to `"unknown"`.

---

## Best Practices

1.  **Keep Schema and Logic in sync**: Since the schema is defined in a separate file, ensure field names in your Go maps/structs match the field names in `schema.graphqls`.
2.  **Use c.Bind()**: Always use `c.Bind()` for accessing arguments to benefit from GoFr's internal mapping and validation.
3.  **Error Handling**: Return errors from your handlers. GoFr will include them in the `errors` array of the GraphQL response and return `422 Unprocessable Entity`.
4.  **Name your operations**: Use `operationName` in your requests so that metrics are tagged meaningfully (e.g., `GetUser` instead of `unknown`).
