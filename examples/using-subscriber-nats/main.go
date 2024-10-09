package main

import (
	"os"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/pubsub/nats"
)

func main() {
	app := gofr.New()

	subjects := strings.Split(app.Config.Get("NATS_SUBJECTS"), ",")

	natsClient := nats.New(&nats.Config{
		Server: os.Getenv("PUBSUB_BROKER"),
		Stream: nats.StreamConfig{
			Stream:   os.Getenv("NATS_STREAM"),
			Subjects: subjects,
			MaxBytes: 1024 * 1024 * 512, // 512MB
		},
		MaxWait:     5 * time.Second,
		BatchSize:   100,
		MaxPullWait: 10,
		Consumer:    os.Getenv("NATS_CONSUMER"),
		CredsFile:   os.Getenv("NATS_CREDS_FILE"),
	})
	natsClient.UseLogger(app.Logger)
	natsClient.UseMetrics(app.Metrics())

	app.AddPubSub(natsClient)

	app.Subscribe("products", func(c *gofr.Context) error {
		var productInfo struct {
			ProductId string `json:"productId"`
			Price     string `json:"price"`
		}

		err := c.Bind(&productInfo)
		if err != nil {
			c.Logger.Error(err)

			return nil
		}

		c.Logger.Info("Received product", productInfo)

		return nil
	})

	app.Subscribe("order-logs", func(c *gofr.Context) error {
		var orderStatus struct {
			OrderId string `json:"orderId"`
			Status  string `json:"status"`
		}

		err := c.Bind(&orderStatus)
		if err != nil {
			c.Logger.Error(err)

			return nil
		}

		c.Logger.Info("Received order", orderStatus)

		return nil
	})

	app.Run()
}
