# Subscibing to Pub/Sub
Gofr currently supports Apache Kafka and Google PubSub.

## Usage
Adding a subscriber is similar to adding a HTTP handler.

The subscriber handler has the following signature.
```go
func(ctx *gofr.Context) error
```
The context `ctx` provides you with the `Bind()` function to Bind the message value to a given
interface. 

> The returned error determines which messages are to be committed and which ones are to be consumed again.

```go
// First argument is the `topic name` followed by a handler which would process the 
// published messages continuously and asynchronously.
app.Subscribe("order-status", func(ctx *gofr.Context)error{
    // Handle the pub-sub message here
})
```

Using `app.Subscribe` will continuously read a message from the configured `PUBSUB_BACKEND` which
can be either `KAFKA` or `GOOGLE` as of now. These can be configured in your configs folder under `.env`
     
### Kafka configs
```dotenv
PUBSUB_BACKEND=KAFKA
PUBSUB_BROKER=localhost:9092
CONSUMER_ID=order-consumer
```

### Google configs
```dotenv
PUBSUB_BACKEND=GOOGLE
GOOGLE_PROJECT_ID=project-order
GOOGLE_SUBSCRIPTION_NAME=order-consumer
```

> To set GOOGLE_APPLICATION_CREDENTIAL - refer [here](https://cloud.google.com/docs/authentication/application-default-credentials)

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
