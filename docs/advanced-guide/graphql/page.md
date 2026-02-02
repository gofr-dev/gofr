# GraphQL in GoFr

GoFr provides a native, code-first approach to building GraphQL APIs. It eliminates the need for schema files or code generation building the GraphQL schema directly from your Go functions and structs.

## Core Concepts

### 1. [Query](https://graphql.org/learn/queries/)
Queries are used to fetch data. In GoFr, a Query resolver is a function that takes `*gofr.Context` and returns a data object (or slice) and an error.

**Best Practice:** Keep Queries idempotent. They should never modify the state of the database or application.

### 2. [Mutation](https://graphql.org/learn/queries/#mutations)
Mutations are used to modify data (Create, Update, Delete). They follow the same signature as Queries but are intended for side effects.

**Best Practice:** Mutations should return the object that was modified. This allows clients to update their local cache immediately.

---

## The Unified Schema

Unlike REST, where every resource has its own URL path (e.g., `/users`, `/products`), GraphQL operates on the principle of a **Single Endpoint**.

### Why Single Endpoint?
GoFr aggregates every `GraphQLQuery` and `GraphQLMutation` you register into a **Unified Schema** served at `/graphql`. This is the industry standard for several reasons:
*   **Request Batching**: Clients can fetch multiple unrelated data sets (e.g., user profile and product list) in a single HTTP round-trip.
*   **Discovery**: Tools like the integrated Playground can introspect the entire API through this single entry point to provide autocomplete.
*   **Consistency**: The frontend only needs to manage one base URL for the entire application.

---

## Getting Started

### Registration

Use `app.GraphQLQuery` and `app.GraphQLMutation` to register your resolvers with **declarative arguments**.

```go
func main() {
    app := gofr.New()

    // Query without arguments
    app.GraphQLQuery("hello", func(c *gofr.Context) (string, error) {
        return "Hello, GraphQL!", nil
    })

    // Query with arguments (declarative style)
    type GetUserArgs struct {
        ID int `json:"id"`
    }
    
    app.GraphQLQuery("user", func(c *gofr.Context, args GetUserArgs) (User, error) {
        // GoFr automatically maps GraphQL arguments to the args struct
        return fetchUser(c, args.ID)
    })

    // Mutation with arguments
    type CreateUserArgs struct {
        Name string `json:"name"`
        Role string `json:"role"`
    }
    
    app.GraphQLMutation("createUser", func(c *gofr.Context, args CreateUserArgs) (User, error) {
        return createUser(c, args.Name, args.Role)
    })

    app.Run()
}
```

### Writing Resolvers

GraphQL resolvers in GoFr use **declarative arguments** - define your argument types as structs in the function signature, and GoFr will automatically:
- Generate the GraphQL schema with proper argument types
- Validate incoming arguments
- Map them to your struct

**Key Points:**
- First parameter is always `*gofr.Context`
- Second parameter (optional) is your arguments struct
- Return type must be a **specific struct or slice**, not `any`

---

## GraphQL vs REST (HTTP) Handlers

In standard REST/HTTP handlers, GoFr allows returning `any` because the response is simply serialized to JSON at runtime. However, GraphQL works differently:

| Feature | REST (HTTP) | GraphQL |
| :--- | :--- | :--- |
| **Return Type** | `any` (Interface) | **Specific Structs/Slices** |
| **Arguments** | Via `c.Param()` or `c.Bind()` | **Declarative in function signature** |
| **Contract** | No Schema | **Auto-Generated Schema** |
| **Why?** | REST only cares about the final JSON blob. | GoFr must "see" the struct fields and arguments to build the schema at startup. |

**Pro Tip:** Always use specific struct types for both return values and arguments. This enables full GraphQL introspection and auto-documentation.

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Role string `json:"role"`
}

type GetUserArgs struct {
    ID int `json:"id"`
}

// ✅ Correct: Specific return type and declarative arguments
func(c *gofr.Context, args GetUserArgs) (User, error) {
    return User{ID: args.ID, Name: "Alice", Role: "Admin"}, nil
}

// ❌ Discouraged: Returns 'any', which prevents sub-field discovery
func(c *gofr.Context, args GetUserArgs) (any, error) {
    return map[string]any{"id": args.ID, "name": "Alice"}, nil
}
```

---

## Testing Your GraphQL API

### 1. Interactive Exploration
GoFr automatically hosts a **GraphQL Playground** at `/graphql/ui` when running in non-production environments. You can use it to browse the auto-generated documentation and test queries with real-time autocomplete.

### 2. Using Postman
To test your API in Postman:
1.  **Method**: Set the request method to `POST`.
2.  **URL**: Enter `http://localhost:9091/graphql`.
3.  **Body**:
    *   Select the **Body** tab.
    *   Choose the **GraphQL** radio button.
    *   Enter your query:
        ```graphql
        query {
          user(id: 1) {
            name
            role
          }
        }
        ```
4.  **Variables**: (Optional) Use the "Query Variables" section in Postman for dynamic values:
    ```json
    {
      "id": 1
    }
    ```

### 3. Using cURL
You can also test your API via terminal using standard JSON POST requests:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"query": "{ user(id: 1) { name } }"}' \
  http://localhost:9091/graphql
```

---

## Middlewares in GraphQL

GoFr's GraphQL engine is built on top of the standard HTTP stack, which means **all GoFr HTTP middlewares are fully reusable** with GraphQL.

### Built-in Middlewares
When you enable GraphQL, GoFr automatically applies these key middlewares to the `/graphql` endpoint:
*   **Logging & Metrics**: Every GraphQL request is logged with its execution time and status code.
*   **Tracer**: Automatically tracks spans for the request and individual resolvers.
*   **Recover**: Ensures a resolver panic doesn't crash your server.
*   **CORS & Auth**: Works exactly the same as in REST. If a user is unauthorized by the Auth middleware, they will receive a `401 Unauthorized` before the GraphQL engine even starts.

### Adding Custom Middlewares
You can add your own logic using the standard `UseMiddleware` method:

```go
app := gofr.New()

// This middleware applies to both REST and GraphQL routes
app.UseMiddleware(func(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Custom logic (e.g. checking headers)
        h.ServeHTTP(w, r)
    })
})

app.Run()
```

---

## Best Practices in GoFr

1.  **Use JSON Tags**: GoFr uses `json` tags on your structs to determine the GraphQL field names. If no tag is provided, it defaults to the PascalCase field name.
2.  **Naming Conventions**:
    *   Queries: Use nouns (e.g., `user`, `products`).
    *   Mutations: Use verbs (e.g., `createUser`, `deleteOrder`).
3.  **Error Handling**: Return clear errors. GoFr automatically logs these errors with Trace IDs and includes them in the GraphQL `errors` response array.
4.  **Observability**: You don't need to add spans manually. GoFr automatically creates spans for the request and each individual resolver.
5.  **Health Checks**: Use the built-in `gofr` field to monitor your app health via GraphQL:
    ```graphql
    query {
      gofr {
        status
        version
      }
    }
    ```

---

## Interactive Exploration

GoFr automatically hosts a **GraphQL Playground** at `/graphql/ui` when running in non-production environments. You can use it to:
*   Browse the auto-generated documentation.
*   Test queries and mutations with real-time autocomplete.
*   View headers and response timings.
