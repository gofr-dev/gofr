# Subscriber Example

This GoFr example demonstrates a simple Subscriber that subscribes asynchronously to a given topic and commits based
on the handler response.

### To run the example follow the below steps:

- Run the docker image of kafka and zookeeper and ensure that your provided topics are created before subscribing.
```console
docker run -d \
  --name zookeeper \
  -e ZOOKEEPER_CLIENT_PORT=2181 \
  -e ZOOKEEPER_TICK_TIME=2000 \
  confluentinc/cp-zookeeper:7.0.1
  
docker run -d \
  --name broker \
  -p 9092:9092 \
  --link zookeeper \
  -e KAFKA_BROKER_ID=1 \
  -e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 \
  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,PLAINTEXT_INTERNAL:PLAINTEXT \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092,PLAINTEXT_INTERNAL://broker:29092 \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  confluentinc/cp-kafka:7.0.1
```

- Now run the example using below command :
```console
go run main.go
```

