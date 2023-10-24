## Postgres Example

#### To run the example follow the steps below:

## Postgres Setup
Run the docker image of Postgres

1. `docker run --name gofr-pgsql -e POSTGRES_DB=customers -e POSTGRES_PASSWORD=root123 -p 2006:5432 -d postgres:15.1`


## Run
Now run the example on path `/zopsmart/gofr/examples/using-potgres` by

1. `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-postgres:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-postgres:$(date +%s) .`