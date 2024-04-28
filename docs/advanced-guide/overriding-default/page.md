# Overriding default

GoFr allows to override default behavior of its features.

## Raw response format

GoFr by default wraps a handler's return value and assigns it to the "data" field in a response.

### Example

```go
package main

import "gofr.dev/pkg/gofr"

type user struct {
  Id   int    `json:"id"`
  Name string `json:"name"`
}

func main() {
  app := gofr.New()

  app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {

    users := []user{{Id: 1, Name: "Daria"}, {Id: 2, Name: "Ihor"}}

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

If you want to have raw a response structure - wrap it in `response.Raw`:
```go
app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {

    users := []user{{Id: 1, Name: "Daria"}, {Id: 2, Name: "Ihor"}}

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