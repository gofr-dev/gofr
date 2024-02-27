package main

import (
	"encoding/json"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.POST("/publish-order", order)

	app.POST("/publish-product", product)

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

func product(ctx *gofr.Context) (interface{}, error) {
	type productInfo struct {
		ProductId string `json:"productId"`
		Price     string `json:"price"`
	}

	var data productInfo

	err := ctx.Bind(&data)
	if err != nil {
		return nil, err
	}

	msg, _ := json.Marshal(data)

	err = ctx.GetPublisher().Publish(ctx, "products", msg)
	if err != nil {
		return nil, err
	}

	return "Published", nil
}
