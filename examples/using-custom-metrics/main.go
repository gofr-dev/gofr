package main

import (
	"time"

	"gofr.dev/pkg/gofr"
)

// This example simulates the usage of custom metrics for transactions of an ecommerce store.

const (
	transactionSuccessful = "transaction_success"
	transactionTime       = "transaction_time"
	totalCreditDaySales   = "total_credit_day_sale"
	productStock          = "product_stock"
)

func main() {
	// Create a new application
	a := gofr.New()

	a.Metrics().NewCounter(transactionSuccessful, "used to track the count of successful transactions")
	a.Metrics().NewGauge(productStock, "used to track the number of products in stock")
	a.Metrics().NewHistogram(transactionTime, "used to track the time taken by a transaction",
		5, 10, 15, 20, 25, 35)

	// Add all the routes
	a.POST("/transaction", TransactionHandler)
	a.POST("/return", ReturnHandler)

	// Run the application
	a.Run()
}

func TransactionHandler(c *gofr.Context) (interface{}, error) {
	transactionStartTime := time.Now()

	// transaction logic

	c.Metrics().IncrementCounter(c, transactionSuccessful)

	tranTime := time.Now().Sub(transactionStartTime).Milliseconds()

	c.Metrics().RecordHistogram(c, transactionTime, float64(tranTime))
	c.Metrics().SetGauge(productStock, 10)

	return "Transaction Successful", nil
}

func ReturnHandler(c *gofr.Context) (interface{}, error) {
	// Update the Gauge metric for product stock
	c.Metrics().SetGauge(productStock, 50)

	return "Return Successful", nil
}
