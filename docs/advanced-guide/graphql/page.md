# GraphQL in GoFr

GoFr provides a **Schema-First** approach to building GraphQL APIs. This means you define your API contract in a standard GraphQL schema file, and GoFr handles the execution, validation, and observability.

## Required Setup

To enable GraphQL, you MUST provide a schema file at the following location:
`./configs/schema.graphqls`

> **Note:** GoFr uses a single schema file. All Query and Mutation types must be defined in this one file.
> You can register multiple resolvers (one per field) using `GraphQLQuery` and `GraphQLMutation`, but
> they all resolve fields within this single schema.

If this file is missing or invalid, GoFr will log a fatal error and the application will fail to start. This fail-fast behavior ensures schema issues are caught at deployment rather than runtime.

## Core Concepts

### 1. [Query](https://graphql.org/learn/queries/)
Queries are used to fetch data. In GoFr, a Query resolver is a function that takes `*gofr.Context` and returns a data object (or `any`) and an error.

### 2. [Mutation](https://graphql.org/learn/queries/#mutations)
Mutations are used to modify data. They follow the same signature as Queries but are intended for side effects.


## The Unified Schema

GoFr aggregates every `GraphQLQuery` and `GraphQLMutation` you register and validates them against your `./configs/schema.graphqls`. The API is served at `/graphql`.

*   **Single Endpoint**: All operations go through `POST /graphql`.
*   **Playground**: Interactive documentation and testing at `/.well-known/graphql/ui`.

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
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

func main() {
    app := gofr.New()

    app.GraphQLQuery("user", func(c *gofr.Context) (any, error) {
        var args struct {
            ID int `json:"id"`
        }

        if err := c.Bind(&args); err != nil {
            return nil, err
        }

        // Return a struct - GoFr validates this against the schema at runtime
        return User{
            ID:   args.ID,
            Name: "Antigravity",
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

GoFr follows the standard GraphQL-over-HTTP convention by returning `200 OK` for all successfully processed requests, including those with resolver errors. This ensures that the response body is the source of truth for execution results.

| Status Code | Condition |
|---|---|
| `200 OK` | The request was processed (regardless of whether it returned data or errors). |
| `400 Bad Request` | The request body is not valid JSON. |

**Error response body**:

> **Note:** The GraphQL error format follows the [GraphQL specification](https://spec.graphql.org/October2021/#sec-Errors),
> which uses an `errors` array. This differs from GoFr's REST API format which uses a singular `error` object.
> This is intentional — each protocol follows its own standard.

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

### 4. Supported Types
GoFr supports all standard GraphQL types including scalars, objects, enums, and input types. For a complete reference on the GraphQL type system, see the [official GraphQL documentation](https://graphql.org/learn/schema/).

---

## Testing Your GraphQL API

### 1. Interactive Exploration
GoFr automatically hosts a **GraphQL Playground** at `/.well-known/graphql/ui` when GraphQL resolvers are registered.

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
GoFr exports several GraphQL-specific metrics, all tagged by `operation_name`, `type` (query/mutation), and `status` (success/error):

- **`app_graphql_operations_total`**: Total number of GraphQL operations received.
- **`app_graphql_error_total`**: Total operations that resulted in an error (resolver error or validation failure).
- **`app_graphql_request_duration`**: Histogram of the entire request lifecycle in seconds.

> **Note:** The `operation_name` tag is sourced from the `operationName` field in the POST body. For anonymous operations, it defaults to `"unknown"`. GraphQL requests are only recorded by the GraphQL-specific metrics above — they are excluded from `app_http_response` to avoid double-counting.

---

## Monitoring and Health Checks

### 1. Health Checks
Even when building a GraphQL-first application, GoFr's standard **RESTful health check endpoints** remain the primary way to monitor service availability. These are automatically registered and publicly accessible:

- **Aliveness**: `/.well-known/alive` (Returns `200 OK` if the server is running)
- **Health**: `/.well-known/health` (Returns detailed dependency status)

GoFr does **not** inject an automatic `health` query into your GraphQL schema. This avoids redundancy and keeps your GraphQL contract focused on business logic.

### 2. Status Metric Label
While traditional HTTP metrics (`app_http_response`) use numerical status codes (e.g., `200`, `500`) for the `status` label, GraphQL metrics (`app_graphql_*`) use a simplified `success` or `error` value.

- **`success`**: The request was processed and returned no errors in the `errors` array.
- **`error`**: The request was processed but one or more resolvers failed (returning a `200 OK` with an `errors` array), or the request itself was invalid (e.g., `400 Bad Request`).

This distinction is important because GraphQL often returns `200 OK` even when business logic fails. The `success`/`error` label provides immediate visibility into the health of your resolvers.

---

## Design and Limitations

GoFr's GraphQL implementation is designed for simplicity and strict adherence to standards while maintaining the framework's "sane defaults" philosophy.

### 1. Why `GraphQLQuery` / `GraphQLMutation` instead of `app.POST`?
GoFr provides dedicated `GraphQLQuery` and `GraphQLMutation` methods rather than reusing `app.POST("/graphql", ...)` because the framework handles schema validation, resolver dispatch, per-field tracing, and automatic metrics internally. A raw POST handler would require you to implement all of this manually.

### 2. Why POST-only?
Per the [GraphQL-over-HTTP specification](https://github.com/graphql/graphql-over-http), all GraphQL operations (including Queries) should be performed via `POST`.
- **Security**: Preventing Queries over `GET` avoids accidentally exposing sensitive parameters in server logs or browser history.
- **Consistency**: All operations use the same interaction model, simplifying middleware and observability.

### 3. Why only Query and Mutation?
Currently, GoFr supports the two most common operation types:
- **Query**: For read-only data fetching.
- **Mutation**: For operations that cause side effects.

**Subscriptions** (real-time updates) are currently not supported as they require a persistent stateful connection (like WebSockets), which deviates from the stateless, request-response model of GoFr's standard HTTP handlers.

### 4. Single Schema File
GoFr enforces a single `./configs/schema.graphqls` file to ensure a "Single Source of Truth" for your API contract. While you can register many resolvers, they must all belong to this single unified schema. This prevents fragmentation and makes the API easier to document and maintain.

---

## Best Practices

1.  **Keep Schema and Logic in sync**: Since the schema is defined in a separate file, ensure field names in your Go maps/structs match the field names in `schema.graphqls`.
2.  **Use c.Bind()**: Always use `c.Bind()` for accessing arguments to benefit from GoFr's internal mapping and validation.
3.  **Error Handling**: Return errors from your handlers. GoFr will include them in the `errors` array of the GraphQL response while still returning `200 OK`.
4.  **Name your operations**: Use `operationName` in your requests so that metrics are tagged meaningfully (e.g., `GetUser` instead of `unknown`).
