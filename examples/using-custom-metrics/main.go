package main

import (
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/metrics"
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

	metrics.Manager().NewCounter(transactionSuccessful, "used to track the count of successful transactions")
	metrics.Manager().NewUpDownCounter(totalCreditDaySales, "used to track the total credit sales in a day")
	metrics.Manager().NewGauge(productStock, "used to track the number of products in stock")
	metrics.Manager().NewHistogram(transactionTime, "used to track the time taken by a transaction",
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

	metrics.Manager().IncrementCounter(c, transactionSuccessful)

	tranTime := time.Now().Sub(transactionStartTime).Microseconds()

	metrics.Manager().RecordHistogram(c, transactionTime, float64(tranTime))
	metrics.Manager().DeltaUpDownCounter(c, totalCreditDaySales, 1000, "sale_type", "credit")
	metrics.Manager().SetGauge(productStock, 10)

	return "Transaction Successful", nil
}

func ReturnHandler(c *gofr.Context) (interface{}, error) {
	// logic to create a sales return
	metrics.Manager().DeltaUpDownCounter(c, totalCreditDaySales, -1000, "sale_type", "credit_return")

	// Update the Gauge metric for product stock
	metrics.Manager().SetGauge(productStock, 50)

	return "Return Successful", nil
}
