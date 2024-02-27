# HTTP Server Example

This GoFr example demonstrates a simple http server which supports redis and mysql as datasources.

### To run the example follow the steps below:

- Run the docker image of redis
```console
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
```

- Run the docker image of mysql
```console
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -p 2001:3306 -d mysql:8.0.30
```

- Now run the example
```console
go run main.go
```
