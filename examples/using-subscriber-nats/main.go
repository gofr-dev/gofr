package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.Subscribe("products", func(c *gofr.Context) error {
		c.Logger.Debug("Received message on 'products' subject")

		var productInfo struct {
			ProductID int     `json:"productId"`
			Price     float64 `json:"price"`
		}

		err := c.Bind(&productInfo)
		if err != nil {
			c.Logger.Error("Error binding product message:", err)
			return nil
		}

		c.Logger.Info("Received product", productInfo)

		return nil
	})

	app.Subscribe("order-logs", func(c *gofr.Context) error {
		c.Logger.Debug("Received message on 'order-logs' subject")

		var orderStatus struct {
			OrderID string `json:"orderId"`
			Status  string `json:"status"`
		}

		err := c.Bind(&orderStatus)
		if err != nil {
			c.Logger.Error("Error binding order message:", err)
			return nil
		}

		c.Logger.Info("Received order", orderStatus)

		return nil
	})

	fmt.Println("Subscribing to 'products' and 'order-logs' subjects...")
	app.Run()
}
