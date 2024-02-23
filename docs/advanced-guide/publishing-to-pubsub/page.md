# Publishing to Pub/Sub
Gofr currently supports Apache Kafka and Google PubSub.

## Usage
To publish to pub/sub topic following configurations has to be done in .env.

### Kafka configs
```dotenv
PUBSUB_BACKEND=KAFKA
PUBSUB_BROKER=localhost:9092
```

### Google configs
```dotenv
PUBSUB_BACKEND=GOOGLE
GOOGLE_PROJECT_ID=project-order
```
> To set GOOGLE_APPLICATION_CREDENTIAL - refer [here](https://cloud.google.com/docs/authentication/application-default-credentials)

Messages can be published to a topic from gofr context.

```go
ctx.GetPublisher().Publish(ctx, "topic", msg)
```

### Example
```go
package main

import (
	"encoding/json"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.GET("/publish-order", order)

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
