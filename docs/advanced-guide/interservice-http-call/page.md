# Interservice HTTP Calls

GoFr supports inter-service http calls which provide the following benefits :

1. Access to the following method from container - GET, PUT, POST, PATCH, DELETE.
2. Logs and traces for the request.

## Usage

### Registering HTTP Service

```go
func main() {
	// Create a new application
	a := gofr.New()

	a.AddHTTPService("anotherService", "http://localhost:9000")
	
	a.GET("/customer", Customer)
	a.GET("/user", User)
	
	// Run the application
	a.Run()
}
```

### Accessing HTTP Service in handler

```go
func Customer(ctx *gofr.Context) (interface{}, error) {
    //Get & Call Another service
    resp, err := ctx.GetHTTPService("anotherService").Get(ctx, "user", nil)
    if err != nil {
        return nil, err
    }
	
	defer resp.Body.Close()
	
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    return string(body), nil
}

func User(_ *gofr.Context) (interface{}, error) {
    return "GoFr", nil
}
```