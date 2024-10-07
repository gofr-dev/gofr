# GoFr Context

GoFr context is an object injected by the GoFr handler. It contains all the request-specific data, for each
request-response cycle a new context is created. The request can be either an HTTP request, GRPC call or
a message from Pub-Sub.
GoFr Context also embeds the **_container_** which maintains all the dependencies like databases, logger, HTTP service clients,
metrics manager, etc. This reduces the complexity of the application as users don't have to maintain and keep track of
all the dependencies by themselves.

GoFr context is an extension of the go context, providing a wrapper around the request and response providing
user access to dependencies.

# Usage

## Reading HTTP requests

`ctx.Request` can be used to access the underlying request which provides the following methods to access different
parts of the request.

- `Context()` - to access the context associated with the incoming request
  ```go
  ctx.Request.Context()
  ```
- `Param(string)` - to access the query parameters present in the request, it returns the value of the key provided
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
  
- `Binding multipart-form data / urlencoded form data ` 
  - To bind multipart-form data or url-encoded form, you can use the Bind method similarly. The struct fields should be tagged appropriately 
    to map the form fields to the struct fields. The supported content types are `multipart/form-data` and `application/x-www-form-urlencoded`
    
    ```go
    type Data struct {
    Name string `form:"name"`

    Compressed file.Zip `file:"upload"`

    FileHeader *multipart.FileHeader `file:"file_upload"`
    }
    ```

  - The `form` tag is used to bind non-file fields.
  - The `file` tag is used to bind file fields. If the tag is not present, the field name is used as the key.


- `HostName()` - to access the host name for the incoming request
  ```go
    // for example if request is made from xyz.com
    host := ctx.Request.HostName()
    // the host would be http://xyz.com
    // Note: the protocol if not provided in the headers will be set to http by default
  ```
- `Params(string)` - to access all query parameters for a given key returning slice of strings.
  ```go 
    // Example: Request is /search?category=books,electronics&category=tech
    values := ctx.Request.Params("category")
    // values = []string{"books", "electronics", "tech"}
  ```

## Accessing dependencies

GoFr context embeds the container object which provides access to
all the injected dependencies by the users. Users can access the fields and methods provided
by the **_container_**.
