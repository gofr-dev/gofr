# Migrations Example

This GoFr example demonstrates the use of `migrations` through a simple HTTP server using MySQL, Redis and Kafka.

### To run the example follow the below steps:
- Run the docker image of MySQL, Redis and Kafka

```console
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=test -p 2001:3306 -d mysql:8.0.30
docker run --name gofr-redis -p 2002:6379 -d redis:7.0.5
docker run --name kafka-1 -p 9092:9092 \
	-e KAFKA_ENABLE_KRAFT=yes \
	-e KAFKA_CFG_PROCESS_ROLES=broker,controller \
	-e KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER \
	-e KAFKA_CFG_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
	-e KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT \
	-e KAFKA_CFG_ADVERTISED_LISTENERS=PLAINTEXT://127.0.0.1:9092 \
	-e KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE=true \
	-e KAFKA_BROKER_ID=1 \
	-e KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=1@127.0.0.1:9093 \
	-e ALLOW_PLAINTEXT_LISTENER=yes \
	-e KAFKA_CFG_NODE_ID=1 \
	-v kafka_data:/bitnami \
	bitnami/kafka:3.4
```

- Now run the example using below command :

```console
go run main.go
```
