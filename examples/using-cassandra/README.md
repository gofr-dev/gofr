#### To run the example follow the steps below:
## Cassandra Setup
Run the docker image of cassandra

1. `sh init.sh`

## Run
Now run the example on path `/zopsmart/gofr/examples/using-cassandra` by

1. `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-cassandra:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-cassandra:$(date +%s) .`

   