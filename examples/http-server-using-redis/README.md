# Redis Example

This GoFr example demonstrates the use of Redis as datasource through a simple HTTP server.

### To run the example follow the steps below:

- Run the docker image of Redis
```console
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
```

- Now run the example
```console
go run main.go
```
