#### To run the example follow the steps below:

## CLICKHOUSE Setup

### Docker command
```c
docker run --rm -e CLICKHOUSE_DB=users -e CLICKHOUSE_USER=root -e CLICKHOUSE_PASSWORD=password -e CLICKHOUSE_HTTP_PORT=8123 -p 9001:9000/tcp  -p 8080:8123/tcp clickhouse/clickhouse-server
```

## Run

Now run the example on path `/zopsmart/gofr/examples/using-clickhouse` by `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-clickhouse:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-clickhouse:$(date +%s) .`

   