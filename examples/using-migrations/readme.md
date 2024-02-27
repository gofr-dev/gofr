# Migrations Example

This example in GoFr demonstrates the use of `migrations` through a simple http server using mysql and redis.

### To run the example follow the below steps:
- Run the docker image of mysql and redis

```console
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -p 2001:3306 -d mysql:8.0.30
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
```

- Now run the example using below command :

```console
go run main.go
```