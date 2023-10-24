## ElasticSearch Example

#### To run the example follow the steps below:

## ElasticSearch Setup
Run the docker image of elasticsearch

1. `docker run -d --name gofr-elasticsearch -p 2012:9200 -p 2013:9300 -e "discovery.type=single-node" elasticsearch:7.10.1`


## Run
Now run the example on path `/zopsmart/gofr/examples/using-elasticsearch` by

1. `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-elasticsearch:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-elasticsearch:$(date +%s) .`

