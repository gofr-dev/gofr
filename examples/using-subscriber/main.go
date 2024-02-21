package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.Subscribe("products", func(c *gofr.Context) error {
		var data struct {
			ProductId string `json:"productId"`
			Price     string `json:"price"`
		}

		err := c.Bind(&data)
		if err != nil {
			c.Logger.Error(err)

			return nil
		}

		c.Logger.Info("Received product ", data)

		return nil
	})

	app.Subscribe("order-logs", func(c *gofr.Context) error {
		var data struct {
			OrderId string `json:"orderId"`
			Status  string `json:"status"`
		}

		err := c.Bind(&data)
		if err != nil {
			c.Logger.Error(err)

			return nil
		}

		c.Logger.Info("Received order ", data)

		return nil
	})

	app.Run()
}
