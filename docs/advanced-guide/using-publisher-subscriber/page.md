# Publisher Subscriber

Publisher Subscriber is an architectural design pattern for asynchronous communication between different entities.
These could be different applications or different instances of the same application.
Thus, the movement of messages between the components is made possible without the components being aware of each other's
identities, meaning the components are decoupled.
This makes the application/system more flexible and scalable as each component can be 
scaled and maintained according to its own requirement.

## Design choice

In GoFr application if a user wants to use the Publisher-Subscriber design, it supports two message brokers—Apache Kafka
and Google PubSub.
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

### KAFKA

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
- Address to connect to kafka broker.
- `+`
-
- `localhost:9092`
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

{% /table %}

```dotenv
PUBSUB_BACKEND=KAFKA# using apache kafka as message broker
PUBSUB_BROKER=localhost:9092
CONSUMER_ID=order-consumer
KAFKA_BATCH_SIZE=1000
KAFKA_BATCH_BYTES=1048576
KAFKA_BATCH_TIMEOUT=300
```

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
PUBSUB_BACKEND=MQTT            // using Mqtt as pubsub
MQTT_HOST=localhost            // broker host url
MQTT_PORT=1883                 // broker port
MQTT_CLIENT_ID_SUFFIX=test     // suffix to a random generated client-id(uuid v4)

#some additional configs(optional)
MQTT_PROTOCOL=tcp              // protocol for connecting to broker can be tcp, tls, ws or wss
MQTT_MESSAGE_ORDER=true  // config to maintain/retain message publish order, by default this is false
MQTT_USER=username       // authentication username
MQTT_PASSWORD=password   // authentication password 
```
> **Note** : If `MQTT_HOST` config is not provided, the application will connect to a public broker
> {% new-tab-link title="HiveMQ" href="https://www.hivemq.com/mqtt/public-mqtt-broker/" /%}

#### Docker setup
```shell 
docker run -d \
  --name mqtt \
  -p 8883:8883 \
  -v <path-to>/mosquitto.conf:/mosquitto/config/mosquitto.conf \
  eclipse-mosquitto:latest
```
> **Note**: find the default mosquitto config file {% new-tab-link title="here" href="https://github.com/eclipse/mosquitto/blob/master/mosquitto.conf" /%}

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
can be either `KAFKA` or `GOOGLE` as of now. These can be configured in the configs folder under `.env`

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

func order(ctx *gofr.Context) (interface{}, error) {
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
