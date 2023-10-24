## MongoDB Example

#### To run the example follow the steps below:

## MongoDB Setup
Run the docker image of MongoDB

1. `docker run --name gofr-mongo -d -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=admin123 -p 2004:27017 mongo:6.0.2`


## Run
Now run the example on path `/zopsmart/gofr/examples/using-mongo` by

1. `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-mongo:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-mongo:$(date +%s) .`