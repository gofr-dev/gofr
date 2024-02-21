package main

import (
	"encoding/json"
	"math/rand"
	"strconv"

	"gofr.dev/pkg/gofr"
)

func main() {
	app := gofr.New()

	app.GET("/publish-order", order)

	app.GET("/publish-product", product)

	app.Run()
}

func order(ctx *gofr.Context) (interface{}, error) {
	type data struct {
		OrderId string `json:"orderId"`
		Status  string `json:"status"`
	}

	d := data{
		OrderId: strconv.Itoa(rand.Int()),
		Status:  "COMPLETED",
	}

	ctx.Log("Published order ", d)

	msg, _ := json.Marshal(d)

	err := ctx.GetPublisher().Publish(ctx, "order-logs", msg)
	if err != nil {
		return nil, err
	}

	return "Published", nil
}

func product(ctx *gofr.Context) (interface{}, error) {
	type data struct {
		ProductId string `json:"productId"`
		Price     string `json:"price"`
	}

	d := data{
		ProductId: strconv.Itoa(rand.Int()),
		Price:     strconv.Itoa(rand.Int() % 100),
	}

	ctx.Log("Published product ", d)

	msg, _ := json.Marshal(d)

	err := ctx.GetPublisher().Publish(ctx, "products", msg)
	if err != nil {
		return nil, err
	}

	return "Published", nil
}
