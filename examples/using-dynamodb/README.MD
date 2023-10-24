## DynamoDB Example

#### To run the example follow the steps below:

## DynamoDB Setup
Run the docker image of DynamoDB

1. `docker run -d --name gofr-dynamodb -p 2021:8000 amazon/dynamodb-local:1.22.0`

Create table by running the following command in `/examples/using-dynamodb` path

1. `go test`

   ***Note :*** It should PASS all the test cases <br></br>

## Run
Now run the example on path `/zopsmart/gofr/examples/using-dynamodb` by

1. `go run main.go`

## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-dynamodb:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-dynamodb:$(date +%s) .`

