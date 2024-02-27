# Publisher Example

This GoFr example demonstrates a simple Publisher that publishes to the given topic when an http request is made to it's
matching route.

### To run the example follow the below steps:

- Run the docker image of kafka and zookeeper and ensure that your provided topics are created before publishing
```console
docker run --rm -d -p 2181:2181 -p 443:2008 -p 2008:2008 -p 2009:2009 \
    --env ADVERTISED_LISTENERS=PLAINTEXT://localhost:443,INTERNAL://localhost:2009 \
    --env LISTENERS=PLAINTEXT://0.0.0.0:2008,INTERNAL://0.0.0.0:2009 \
    --env SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,INTERNAL:PLAINTEXT \
    --env INTER_BROKER=INTERNAL \
    --env KAFKA_CREATE_TOPICS="test-topic,test:36:1,krisgeus:12:1:compact" \
    --name gofr-kafka \
    krisgeus/docker-kafka
```

- Now run the example using below command :

```console
go run main.go
```

