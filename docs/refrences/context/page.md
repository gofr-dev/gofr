# GoFr Context
Gofr context is an object injected by the gofr handler. It contains all the request-specific data, so for each
request-response cycle a new context is created. The request can be of any type be it an HTTP request, GRPC call or
a message from Pub-Sub.
GoFr Context also embeds the **_container_** which maintains all the dependencies like databases, logger, http service clients,
, metrics manager, etc. This reduces the complexity of the application as users don't have to maintain and keep track of
all the dependecies by themselves.

Gofr context is an extension of the default go context, providing a wrapper around the request and response providing the
user access to the dependecies on the go. 

# Usage
## Reading HTTP requests
`ctx.Request` can be used to access the underlying request which provides the following methods to access different
parts of the request.
- `Context()` - to access the context associated with the incoming request
    ```go
    ctx.Request.Context()
    ```
- `Param(string)` - to access the query parameters present in the request, it retruns the value of the key provided
  ```go
  // Example: Request is /configs?key1=value1&key2=value2
  value := ctx.Request.Param("key1")
  // value = "value1"
  ```
- `PathParam(string)` - to retrieve the path parameters
  ```go
  // Consider the path to be /employee/{id}
  id := ctx.Request.PathParam("id")
  ```
- `Bind(interface{})` - to access a decoded format of the request body, the body is mapped to the interface provided 
  ```go
  // incoming request body is 
  // {
  //    "name" : "trident",
  //    "category" : "snacks"
  // }
  
  type product struct{
      Name string `json:"name"`
      Category string `json:"category"`
  }

  var p product 
  ctx.Bind(&p)
  // the Bind() method will map the incoming request to variable p
  ```
- `HostName()` - to access the host name for the incoming request
  ```go
  // for example if request is made from xyz.com
  host := ctx.Request.HostName()
  // the host would be http://xyz.com
  // Note: the protocol if not provided in the headers will be set to http by default
  ``` 
  
## Accessing dependecies
As mentioned earlier, gofr context embeds the container object which provides the access to 
all the injected dependencies by the users. Users can access the fields and methods provided 
by the **_container_**.