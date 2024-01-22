# Interservice HTTP Calls

GoFr supports inter service http calls which provide the following benefits :

1. Access to the following method from container - GET, PUT, POST, PATCH, DELETE.
2. Info logs and traces for the request.

## Usage

### Registering HTTP Service

```go
func main() {
	// Create a new application
	a := gofr.New()

	a.AddHTTPService("anotherService", "http://localhost:9000")

    a.GET("/redis", RedisHandler)
	
	// Run the application
	a.Run()
}
```

### Accessing HTTP Service in handler

```go
func RedisHandler(ctx *gofr.Context) (interface{}, error) {
	//Call Another service
	resp, err := ctx.GetHTTPService("anotherService").Get(c, "redis", nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
```