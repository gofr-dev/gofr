## Solr Example
#### To run the example follow the steps below:

## Solr Setup
Run the docker image of Solr

1. `docker run --name gofr-solr -p 2020:8983 solr:8 -DzkRun`

Now run the following command to load the schema

1. `docker exec -i gofr-solr sh < ../../.github/setups/solrSchema.sh;`

## Run
Now run the example on path `/zopsmart/gofr/examples/using-solr` by

1. `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-solr:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-solr:$(date +%s) .`