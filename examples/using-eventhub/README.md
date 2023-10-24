# USING EVENTHUB
An app using eventhub built using gofr.

## Database Setup
Run the docker image of Databases

```
  docker run --rm -d -p 2181:2181 -p 443:2008 -p 2008:2008 -p 2009:2009 \
      --env ADVERTISED_LISTENERS=PLAINTEXT://localhost:443,INTERNAL://localhost:2009 \
      --env LISTENERS=PLAINTEXT://0.0.0.0:2008,INTERNAL://0.0.0.0:2009 \
      --env SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,INTERNAL:PLAINTEXT \
      --env INTER_BROKER=INTERNAL \
      --env KAFKA_CREATE_TOPICS="test-topic,test:36:1,krisgeus:12:1:compact" \
      --name gofr-kafka \
      krisgeus/docker-kafka
  ```

## RUN
To run the app follow the below steps:

1. ` go run main.go`

This will start the server at port 9113.

## DOCKER BUILD
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-eventhub:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-eventhub:$(date +%s) .`