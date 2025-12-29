# Publisher Subscriber

Publisher Subscriber is an architectural design pattern for asynchronous communication between different entities.
These could be different applications or different instances of the same application.
Thus, the movement of messages between the components is made possible without the components being aware of each other's
identities, meaning the components are decoupled.
This makes the application/system more flexible and scalable as each component can be 
scaled and maintained according to its own requirement.

## Design choice

In GoFr application if a user wants to use the Publisher-Subscriber design, it supports several message brokers, 
including Apache Kafka, Google PubSub, MQTT, NATS JetStream, and Redis Pub/Sub.
The initialization of the PubSub is done in an IoC container which handles the PubSub client dependency.
With this, the control lies with the framework and thus promotes modularity, testability, and re-usability.
Users can do publish and subscribe to multiple topics in a single application, by providing the topic name.
Users can access the methods of the container to get the Publisher and Subscriber interface to perform subscription 
to get a single message or publish a message on the message broker.
> Container is part of the GoFr Context

## Configuration and Setup

Some of the configurations that are required to configure the PubSub backend that an application is to use
that are specific for the type of message broker user wants to use. 
`PUBSUB_BACKEND` defines which message broker the application needs to use.

### Kafka

#### Configs
{% table %}
- Name
- Description
- Required
- Default
- Example
- Valid format

---

- `PUBSUB_BACKEND`
- Using Apache Kafka as message broker.
- `+`
-
- `KAFKA`
- Not empty string

---

- `PUBSUB_BROKER`
- Address to connect to kafka broker. Multiple brokers can be added as comma separated values.
- `+`
-
- `localhost:9092` or `localhost:8087,localhost:8088,localhost:8089`
- Not empty string

---

- `CONSUMER_ID`
- Consumer group id to uniquely identify the consumer group.
- if consuming
-
- `order-consumer`
- Not empty string

---

- `PUBSUB_OFFSET`
- Determines from whence the consumer group should begin consuming when it finds a partition without a committed offset.
- `-`
- `-1`
- `10`
- int

---

- `KAFKA_BATCH_SIZE`
- Limit on how many messages will be buffered before being sent to a partition.
- `-`
- `100`
- `10`
- Positive int

---

- `KAFKA_BATCH_BYTES`
- Limit the maximum size of a request in bytes before being sent to a partition.
- `-`
- `1048576`
- `65536`
- Positive int

---

- `KAFKA_BATCH_TIMEOUT`
- Time limit on how often incomplete message batches will be flushed to Kafka (in milliseconds).
- `-`
- `1000`
- `300`
- Positive int

---

- `KAFKA_SECURITY_PROTOCOL`
- Security protocol used to communicate with Kafka (e.g., PLAINTEXT, SSL, SASL_PLAINTEXT, SASL_SSL).
- `-`
- `PLAINTEXT`
- `SASL_SSL`
- String

---

- `KAFKA_SASL_MECHANISM`
- SASL mechanism for authentication (e.g., PLAIN, SCRAM-SHA-256, SCRAM-SHA-512).
- `-`
- `""`
- `PLAIN`
- String

---

- `KAFKA_SASL_USERNAME`
- Username for SASL authentication.
- `-`
- `""`
- `user`
- String

---

- `KAFKA_SASL_PASSWORD`
- Password for SASL authentication.
- `-`
- `""`
- `password`
- String

---

- `KAFKA_TLS_CERT_FILE`
- Path to the TLS certificate file.
- `-`
- `""`
- `/path/to/cert.pem`
- Path

---

- `KAFKA_TLS_KEY_FILE`
- Path to the TLS key file.
- `-`
- `""`
- `/path/to/key.pem`
- Path

---

- `KAFKA_TLS_CA_CERT_FILE`
- Path to the TLS CA certificate file.
- `-`
- `""`
- `/path/to/ca.pem`
- Path

---

- `KAFKA_TLS_INSECURE_SKIP_VERIFY`
- Skip TLS certificate verification.
- `-`
- `false`
- `true`
- Boolean

{% /table %}

```dotenv
PUBSUB_BACKEND=KAFKA# using apache kafka as message broker
PUBSUB_BROKER=localhost:9092
CONSUMER_ID=order-consumer
KAFKA_BATCH_SIZE=1000
KAFKA_BATCH_BYTES=1048576
KAFKA_BATCH_TIMEOUT=300
KAFKA_SASL_MECHANISM=PLAIN
KAFKA_SASL_USERNAME=user
KAFKA_SASL_PASSWORD=password
KAFKA_TLS_CERT_FILE=/path/to/cert.pem
KAFKA_TLS_KEY_FILE=/path/to/key.pem
KAFKA_TLS_CA_CERT_FILE=/path/to/ca.pem
KAFKA_TLS_INSECURE_SKIP_VERIFY=true

#### Docker setup
```shell
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

### GOOGLE

#### Configs
```dotenv
PUBSUB_BACKEND=GOOGLE                   // using Google PubSub as message broker
GOOGLE_PROJECT_ID=project-order         // google projectId where the PubSub is configured
GOOGLE_SUBSCRIPTION_NAME=order-consumer // unique subscription name to identify the subscribing entity
```

#### Docker setup
```shell
docker pull gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators
docker run --name=gcloud-emulator -d -p 8086:8086 \
	gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators gcloud beta emulators pubsub start --project=test123 \
	--host-port=0.0.0.0:8086
```
> **Note**: To set GOOGLE_APPLICATION_CREDENTIAL - refer {% new-tab-link title="here" href="https://cloud.google.com/docs/authentication/application-default-credentials" /%}

> **Note**: In Google PubSub only one subscription name can access one topic, framework appends the topic name and subscription name to form the
> unique subscription name on the Google client.

### MQTT

#### Configs
```dotenv
PUBSUB_BACKEND=MQTT            // using MQTT as pubsub
MQTT_HOST=localhost            // broker host URL
MQTT_PORT=1883                 // broker port
MQTT_CLIENT_ID_SUFFIX=test     // suffix to a random generated client-id(uuid v4)

#some additional configs(optional)
MQTT_PROTOCOL=tcp              // protocol for connecting to broker can be tcp, tls, ws or wss
MQTT_MESSAGE_ORDER=true  // config to maintain/retain message publish order, by default this is false
MQTT_USER=username       // authentication username
MQTT_PASSWORD=password   // authentication password 
```
> **Note** : If `MQTT_HOST` config is not provided, the application will connect to a public broker
> {% new-tab-link title="EMQX Broker" href="https://www.emqx.com/en/mqtt/public-mqtt5-broker" /%}

#### Docker setup
```shell 
docker run -d \
	--name mqtt \
	-p 8883:8883 \
	-v \
	eclipse-mosquitto:latest <path-to >/mosquitto.conf:/mosquitto/config/mosquitto.conf
```
> **Note**: find the default mosquitto config file {% new-tab-link title="here" href="https://github.com/eclipse/mosquitto/blob/master/mosquitto.conf" /%}
 
### NATS JetStream

NATS JetStream is supported as an external PubSub provider, meaning if you're not using it, it won't be added to your binary.

**References**

https://docs.nats.io/
https://docs.nats.io/nats-concepts/jetstream
https://docs.nats.io/using-nats/developer/connecting/creds

#### Configs
```dotenv
PUBSUB_BACKEND=NATS
PUBSUB_BROKER=nats://localhost:4222
NATS_STREAM=mystream
NATS_SUBJECTS=orders.*,shipments.*
NATS_MAX_WAIT=5s
NATS_MAX_PULL_WAIT=500ms
NATS_CONSUMER=my-consumer
NATS_CREDS_FILE=/path/to/creds.json
```

#### Setup

To set up NATS JetStream, follow these steps:

1. Import the external driver for NATS JetStream:

```bash
go get gofr.dev/pkg/gofr/datasources/pubsub/nats
```

2. Use the `AddPubSub` method to add the NATS JetStream driver to your application:

```go   
app := gofr.New()

app.AddPubSub(nats.New(nats.Config{
    Server:     "nats://localhost:4222",
    Stream: nats.StreamConfig{
        Stream:   "mystream",
        Subjects: []string{"orders.*", "shipments.*"},
    },
    MaxWait:     5 * time.Second,
    MaxPullWait: 500 * time.Millisecond,
    Consumer:    "my-consumer",
    CredsFile:   "/path/to/creds.json",
}))
```

#### Docker setup
```shell
docker run -d \
	--name nats \
	-p 4222:4222 \
	-p 8222:8222 \
	-v \
	nats:2.9.16 <path-to >/nats.conf:/nats/config/nats.conf
```

#### Configuration Options

| Name | Description | Required | Default | Example |
|------|-------------|----------|---------|---------|
| `PUBSUB_BACKEND` | Set to "NATS" to use NATS JetStream as the message broker | Yes | - | `NATS` |
| `PUBSUB_BROKER` | NATS server URL | Yes | - | `nats://localhost:4222` |
| `NATS_STREAM` | Name of the NATS stream | Yes | - | `mystream` |
| `NATS_SUBJECTS` | Comma-separated list of subjects to subscribe to | Yes | - | `orders.*,shipments.*` |
| `NATS_MAX_WAIT` | Maximum wait time for batch requests | No | - | `5s` |
| `NATS_MAX_PULL_WAIT` | Maximum wait time for individual pull requests | No | 0 | `500ms` |
| `NATS_CONSUMER` | Name of the NATS consumer | No | - | `my-consumer` |
| `NATS_CREDS_FILE` | Path to the credentials file for authentication | No | - | `/path/to/creds.json` |

#### Usage

When subscribing or publishing using NATS JetStream, make sure to use the appropriate subject name that matches your stream configuration.
For more information on setting up and using NATS JetStream, refer to the official NATS documentation.

### Redis Pub/Sub

Redis Pub/Sub is a lightweight messaging system. GoFr supports two modes:
1. **Streams Mode** (Default): Uses Redis Streams for persistent messaging with consumer groups and acknowledgments.
2. **PubSub Mode**: Standard Redis Pub/Sub (fire-and-forget, no persistence).

#### Redis connection

Redis Pub/Sub uses the same Redis connection configuration as the Redis datasource (`REDIS_HOST`, `REDIS_PORT`, `REDIS_DB`, TLS, etc.).
See the config reference: `https://gofr.dev/docs/references/configs#redis`.

#### Example `.env`

```dotenv
PUBSUB_BACKEND=REDIS
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_USER=myuser
REDIS_PASSWORD=mypassword
REDIS_DB=0
REDIS_PUBSUB_DB=1
REDIS_TLS_ENABLED=true
REDIS_TLS_CA_CERT=/path/to/ca.pem
REDIS_TLS_CERT=/path/to/cert.pem
REDIS_TLS_KEY=/path/to/key.pem

# Streams mode (default) - requires consumer group
REDIS_STREAMS_CONSUMER_GROUP=my-group
REDIS_STREAMS_CONSUMER_NAME=my-consumer
REDIS_STREAMS_BLOCK_TIMEOUT=5s
REDIS_STREAMS_PEL_RATIO=0.7  # 70% PEL, 30% new messages
REDIS_STREAMS_MAXLEN=1000

# To use PubSub mode instead, set:
# REDIS_PUBSUB_MODE=pubsub
```

#### Docker setup

```shell
docker run -d \
	--name redis \
	-p 6379:6379 \
	redis:7-alpine
```

For Redis with password authentication:

```shell
docker run -d \
	--name redis \
	-p 6379:6379 \
	redis:7-alpine redis-server --requirepass mypassword
```

#### Redis configs

The following configs apply specifically to Redis Pub/Sub behavior. For base Redis connection/TLS configs, refer to
`https://gofr.dev/docs/references/configs#redis`.
{% table %}
- Name
- Description
- Default
- Example

---

- `PUBSUB_BACKEND`
- Set to `REDIS` to use Redis as the Pub/Sub backend.
- -
- `REDIS`

---

- `REDIS_PUBSUB_MODE`
- Mode: `streams` (default, at-least-once) or `pubsub` (at-most-once)
- `streams`
- `pubsub`

---

- `REDIS_STREAMS_CONSUMER_GROUP`
- Consumer group name (required in streams mode)
- -
- `mygroup`

---

- `REDIS_STREAMS_CONSUMER_NAME`
- Consumer name (optional; auto-generated if empty)
- -
- `consumer-1`

---

- `REDIS_STREAMS_BLOCK_TIMEOUT`
- Blocking timeout for stream reads. Lower values (1s-2s) = faster detection, higher CPU. Higher values (10s-30s) = lower CPU, higher latency.
- `5s`
- `2s` or `30s`

---

- `REDIS_STREAMS_PEL_RATIO`
- Ratio of PEL (pending) messages to read vs new messages (0.0-1.0). Controls priority: ratio determines initial split, then remaining capacity is filled from the other source. 0.7 = prioritize 70% PEL, fill remaining with new. 0.0 = prioritize new, fill remaining with PEL. 1.0 = prioritize PEL, fill remaining with new.
- `0.7`
- `0.5` or `0.8`

---

- `REDIS_STREAMS_MAXLEN`
- Max stream length for trimming (approximate). Set to `0` for unlimited.
- `0` (unlimited)
- `10000`

---

- `REDIS_PUBSUB_DB`
- Redis DB for Pub/Sub operations. Keep different from `REDIS_DB` when using migrations + streams mode.
- `15`
- `1`

---

- `REDIS_PUBSUB_BUFFER_SIZE`
- Message buffer size
- `100`
- `1000`

---

- `REDIS_PUBSUB_QUERY_TIMEOUT`
- Timeout for Query operations
- `5s`
- `30s`

---

- `REDIS_PUBSUB_QUERY_LIMIT`
- Message limit for Query operations
- `10`
- `50`
{% /table %}

For Redis with TLS:

```shell
docker run -d \
	--name redis \
	-p 6379:6379 \
	-v /path/to/certs:/tls \
	redis:7-alpine redis-server \
	--tls-port 6380 \
	--port 0 \
	--tls-cert-file /tls/redis.crt \
	--tls-key-file /tls/redis.key \
	--tls-ca-cert-file /tls/ca.crt
```

> **Note**: Topics are auto-created on first publish. When using GoFr migrations with Streams mode, keep `REDIS_DB` and `REDIS_PUBSUB_DB` separate (defaults: 0 and 15). For `REDIS_STREAMS_BLOCK_TIMEOUT`: use 1s-2s for real-time or 10s-30s for batch processing.

### Azure Event Hubs
GoFr supports Event Hubs starting gofr version v1.22.0.

While subscribing gofr reads from all the partitions of the consumer group provided in the configuration reducing hassle to manage them.

#### Configs

Azure Event Hubs is supported as an external PubSub provider such that if you are not using it, it doesn't get added in your binary.

Import the external driver for `eventhub` using the following command.

```bash
go get gofr.dev/pkg/gofr/datasource/pubsub/eventhub
```

Use the `AddPubSub` method of GoFr's app to connect

**Example**
```go
app := gofr.New()
    
    app.AddPubSub(eventhub.New(eventhub.Config{
       ConnectionString:          "Endpoint=sb://gofr-dev.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=<key>",
       ContainerConnectionString: "DefaultEndpointsProtocol=https;AccountName=gofrdev;AccountKey=<key>;EndpointSuffix=core.windows.net",
       StorageServiceURL:         "https://gofrdev.windows.net/",
       StorageContainerName:      "test",
       EventhubName:              "test1",
       ConsumerGroup:             "$Default",
    }))
```

While subscribing/publishing from Event Hubs make sure to keep the topic-name same as event-hub name.

#### Setup

1. To set up Azure Event Hubs refer the following [documentation](https://learn.microsoft.com/en-us/azure/event-hubs/event-hubs-create).

2. As GoFr manages reading from all the partitions it needs to store the information about what has been read and what is left for that GoFr uses Azure Container which can be setup from the following [documentation](https://learn.microsoft.com/en-us/azure/storage/blobs/blob-containers-portal).

##### Mandatory Configs Configuration Map
{% table %}
- ConnectionString
- [connection-string-primary-key](https://learn.microsoft.com/en-us/azure/event-hubs/event-hubs-get-connection-string)

---

- ContainerConnectionString
- [ConnectionString](https://learn.microsoft.com/en-us/azure/storage/common/storage-account-keys-manage?toc=%2Fazure%2Fstorage%2Fblobs%2Ftoc.json&bc=%2Fazure%2Fstorage%2Fblobs%2Fbreadcrumb%2Ftoc.json&tabs=azure-portal#view-account-access-keys)


---

- StorageServiceURL
- [Blob Service URL](https://learn.microsoft.com/en-us/azure/storage/common/storage-account-get-info?tabs=portal#get-service-endpoints-for-the-storage-account)

---

- StorageContainerName
- [Container Name](https://learn.microsoft.com/en-us/azure/storage/blobs/blob-containers-portal#create-a-container)

---

- EventhubName
- [Eventhub](https://learn.microsoft.com/en-us/azure/event-hubs/event-hubs-create#create-an-event-hub)

{% /table %}

#### Example


## Subscribing
Adding a subscriber is similar to adding an HTTP handler, which makes it easier to develop scalable applications,
as it decoupled from the Sender/Publisher.
Users can define a subscriber handler and do the message processing and
use `app.Subscribe` to inject the handler into the application.
This is inversion of control pattern, which lets the control stay with the framework and eases the development 
and debugging process.

The subscriber handler has the following signature.
```go
func (ctx *gofr.Context) error
```

`Subscribe` method of GoFr App will continuously read a message from the configured `PUBSUB_BACKEND` which
can be `KAFKA`, `GOOGLE`, `MQTT`, `NATS`, `REDIS`, or `AZURE_EVENTHUB`. These can be configured in the configs folder under `.env`

> The returned error determines which messages are to be committed and which ones are to be consumed again.

```go
// First argument is the `topic name` followed by a handler which would process the 
// published messages continuously and asynchronously.
app.Subscribe("order-status", func(ctx *gofr.Context)error{
    // Handle the pub-sub message here
})
```

The context `ctx` provides user with the following methods:

* `Bind()` - Binds the message value to a given data type. Message can be converted to `struct`, `map[string]any`, `int`, `bool`, `float64` and `string` types.
* `Param(p string)/PathParam(p string)` - Returns the topic when the same is passed as param.


### Example
```go
package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.Subscribe("order-status", func(c *gofr.Context) error {
		var orderStatus struct {
			OrderId string `json:"orderId"`
			Status  string `json:"status"`
		}

		err := c.Bind(&orderStatus)
		if err != nil {
			c.Logger.Error(err)

			// returning nil here as we would like to ignore the
			// incompatible message and continue reading forward
			return nil
		}

		c.Logger.Info("Received order ", orderStatus)

		return nil
	})

	app.Run()
}
```

## Publishing
The publishing of message is advised to done at the point where the message is being generated.
To facilitate this, user can access the publishing interface from `gofr Context(ctx)` to publish messages.

```go
ctx.GetPublisher().Publish(ctx, "topic", msg)
```

Users can provide the topic to which the message is to be published. 
GoFr also supports multiple topic publishing.
This is beneficial as applications may need to send multiple kinds of messages in multiple topics.

### Example
```go
package main

import (
	"encoding/json"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.POST("/publish-order", order)

	app.Run()
}

func order(ctx *gofr.Context) (any, error) {
	type orderStatus struct {
		OrderId string `json:"orderId"`
		Status  string `json:"status"`
	}

	var data orderStatus

	err := ctx.Bind(&data)
	if err != nil {
		return nil, err
	}

	msg, _ := json.Marshal(data)

	err = ctx.GetPublisher().Publish(ctx, "order-logs", msg)
	if err != nil {
		return nil, err
	}

	return "Published", nil
}
```
> #### Check out the following examples on how to publish/subscribe to given topics:
> ##### [Subscribing Topics](https://github.com/gofr-dev/gofr/blob/main/examples/using-subscriber/main.go)
> ##### [Publishing Topics](https://github.com/gofr-dev/gofr/blob/main/examples/using-publisher/main.go)