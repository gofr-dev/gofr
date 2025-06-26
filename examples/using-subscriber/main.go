package main

import (
	"gofr.dev/examples/using-subscriber/migrations"
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.Migrate(migrations.All())

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
