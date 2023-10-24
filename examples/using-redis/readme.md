#### To run the example follow the steps below:
## Redis Setup
Run the docker image of redis

1. `docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5`
   
## Run
Now run the example on path `/zopsmart/gofr/examples/using-redis` by

1. `go run main.go`

 ## Docker Build
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-redis:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-redis:$(date +%s) .`

   