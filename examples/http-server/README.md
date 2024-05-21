# HTTP Server Example

This GoFr example demonstrates a simple HTTP server which supports Redis and MySQL as datasources.

### To run the example follow the steps below:

- Run the docker image of Redis
```console
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
```

- Run the docker image of MySQL
```console
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test -p 2001:3306 -d mysql:8.0.30
```

- Now run the example
```console
go run main.go
```
