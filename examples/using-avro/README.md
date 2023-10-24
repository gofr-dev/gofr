# USING AVRO
An app using avro built using gofr.

## RUN
To run the app follow the below steps:

1. ` go run main.go`

This will start the server at port 9111.

## DOCKER BUILD
To Build a docker image, follow the below steps:

On non linux machines :
1. `GOOS=linux go build main.go` This will build a go binary
2. `docker build -t using-avro:$(date +%s) .`

On linux machines(Ubuntu/Mac):
1. `go build main.go` This will build a go binary
2. `docker build -t using-avro:$(date +%s) .`


# AVRO PubSub

##Instructions

Avro pub sub uses Kafka to write or read messages.

In order for this example to run:

- If PUBSUB_BACKEND is AVRO

    1. Provide the mandatory configs required for avro: AVRO_SCHEMA_URL
    2. Avro uses KAFKA streams, so mandatory configs for Kafka needs to be provided: KAFKA_HOSTS and KAFKA_TOPIC
    
- If PUBSUB_BACKEND is KAFKA

    1. KAFKA_HOSTS and KAFKA_TOPIC are the mandatory configs