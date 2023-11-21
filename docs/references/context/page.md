# Context

Gofr Context is an object which is injected in any gofr handler. Context used in gofr handlers is a holder of all request specific concerns. So, each request-response cycle gets a unique context. Depending on [configurations](/docs/v1/references/configs), gofr initializes and injects different dependencies like [logger](/docs/v1/references/logs), [datastores](/docs/v1/references/datastore), etc. which Handlers can use for a variety of purposes.

All handlers in gofr have same singature `func (*gofr.Context) (interface{}, error)`.

One can also think of the `gofr.Context` as a wrapper around the request, responses, and all dependencies. So, if a handler needs any information from the request or needs to get any connection for a database, context will provide it.

## Getting information from HTTP Request

### Retrieving the underlying request

`ctx.Request` is used to access the incoming request. It returns the `*http.Request`.

### Identifying all the query parameters

`ctx.Params` is used to access all the parameters present in the request. For example, consider the following request `/config?key1=value1&key2=value2&key2=value3`. The `?` indicates that anything after it is query parameters. The query parameters can be accessed through `c.Params()`. `c.Params` returns a `map[string]string`. If the same key has multiple values then the value in c.Params will be comma-separated.

### Getting a query parameter

`c.Param(queryParam)` will provide value from a http URL Param in context of a server. So, if there is a request like `/config?key=value` then `c.Param("key")` will return `value`.

### Getting a value from Path

One can attach a handler to a path with named parameters using `k.GET("/product/{id}", productHandler)`. Once a route is attached with a named parameter using `{}`, handler can request the value using `c.PathParam(paramName)`. In this current example, if for the URL `/product/123456`, `c.PathParam("id")` will return `123456`

### Getting Request Body

GoFr abstracts away the actual request body but provides a way to map the body of a request to a variable. `c.Bind(interface{}) error`.

> For example, if one is making a post request with the body as following:

```json
{
  "something": "value",
  "something-else": "value"
}
```

To read this inside the handler as a map, a map would have to be initialized and the body will have to be bound to it as shown below:

```go
data := make(map[string]string)
if err := ctx.Bind(&data); err != nil {
	return nil, err
}
```

Similarly, structured data can be bound to a struct. For example, a POST request with a JSON body is made as follows:

```json
{
  "name": "Loreal Shampoo",
  "category": "Hair Care"
}
```

It can be bound as shown below:

```go
type product struct{
 Name string `json:"name"`
 Category string `json:"category"`
}

var p product ctx.Bind(&p)
fmt.Printf("Product Name is: %s, Category is: %s, p.Name, p.Category)
```

Handling `XML` body:

```xml
<Name>
	<FirstName>Hello</FirstName>
</Name>
```

```go
type Name struct {
 FirstName string `xml:"FirstName"`
}
var n Name
ctx.Bind(&n)
```

_The address of the variable needs to be passed when utilizing `ctx.Bind`, otherwise, the binding will not be successful._

### BindStrict for request-Body

GoFr support the BindStrict for request-body where it checks the request-body contains fields that are present or not in provided struct. `ctx.BindStrict(interface{}) error`.The error will not be thrown if the provided struct has more field than request-body. The error will be thrown only when all request body's fields are not present in the struct fields.

> For example, if one is making a post request with the body as following:

```json
{
  "something": "value",
  "something-else": "value"
}
```

To read this inside the handler as a map, a map would have to be initialized and the body will have to be bound to it as shown below:

```go
data := make(map[string]string)
if err := ctx.BindStrict(&data); err != nil {
  return nil, err
}
```

Similarly, structured data can be bound to a struct. For example, a POST request with a JSON body is made as follows:

```go
{
	"name": "Loreal Shampoo",
	"category": "Hair Care"
}
```

It can be bound as shown below:

```go
type product struct{
 Name string `json:"name"`
 Category string `json:"category"`
}
var p product
c.BindStrict(&p)
return p
```

Handling `XML` body:

```xml
<Name>
	<FirstName>Hello</FirstName>
</Name>
```

```go
type Name struct {
 FirstName string `xml:"FirstName"`
}
var n Name
c.BindStrict(&n)
```

7. `Note`: The address of the variable needs to be passed when utilizing `c.BindStrict`, otherwise the Strict binding will not be successful.

## Headers

The value of a header for a HTTP request can be checked by `c.Header(<header_name>)`. e.g, if we wanted to check the header `content-type` of a http request which returns a json body, one can use `c.Header("Content-Type")` and it would return `application/json`.

## Log

```go
 Log(key string, value interface{})
```

`Log` logs the key-value pair into the logs. Whenever a key-value pair is added using `Log`, that key-value pair is going to logged in every log line for a given request.

For more clarity see this example : [Adding data to the log](https://docs.zopsmart.com/doc/43-logging-ruq0y3KJqu)

## SetPathParams

```go
SetPathParams(pathParams map[string]string)
```

`SetPathParams` sets the URL path variables to the given value. It can be accessed through `c.PathParam(key)`.

`SetPathParams` should not be used in the application, but should only be used while testing our Handler.

## Using different Databases
