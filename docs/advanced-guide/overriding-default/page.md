# Overriding Default

GoFr allows to override default behavior of its features.

## Raw response format

GoFr by default wraps a handler's return value and assigns it to the "data" field in a response.

### Example

```go
package main

import "gofr.dev/pkg/gofr"

type user struct {
  ID   int    `json:"id"`
  Name string `json:"name"`
}

func main() {
  app := gofr.New()

  app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {

    users := []user{{ID: 1, Name: "Daria"}, {ID: 2, Name: "Ihor"}}

    return users, nil
  })

  app.Run()
}
```

Response example:
```json
{
  "data": [
    {
      "id": 1,
      "name": "Daria"
    },
    {
      "id": 2,
      "name": "Ihor"
    }
  ]
}
```

If you want to have a raw response structure - wrap it in `response.Raw`:
```go
app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {

    users := []user{{ID: 1, Name: "Daria"}, {ID: 2, Name: "Ihor"}}

    return response.Raw{Data: users}, nil	
})
```

Response example:
```json
[
  {
    "id": 1,
    "name": "Daria"
  },
  {
    "id": 2,
    "name": "Ihor"
  }
]
```

## Favicon.ico

By default GoFr load it's own `favicon.ico` present in root directory for an application. To override `favicon.ico` user 
can place it's custom icon in the **static** directory of it's application.

> NOTE: The custom favicon should also be named as `favicon.ico` in the static directory of application.