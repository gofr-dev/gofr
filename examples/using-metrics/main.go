package main

import (
	"fmt"
	"gofr.dev/pkg/gofr"
	"time"
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
	a.Metrics().NewUpDownCounter(totalCreditDaySales, "used to track the total credit sales in a day")
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

	c.MetricsManager.IncrementCounter(c, transactionSuccessful)

	tranTime := time.Now().Sub(transactionStartTime).Microseconds()

	c.MetricsManager.RecordHistogram(c, transactionTime, float64(tranTime))
	c.MetricsManager.DeltaUpDownCounter(c, totalCreditDaySales, 1000)
	c.MetricsManager.SetGauge(productStock, 10)

	return fmt.Sprintf("transaction successful"), nil
}

func ReturnHandler(c *gofr.Context) (interface{}, error) {
	// logic to create a sales return
	c.MetricsManager.DeltaUpDownCounter(c, totalCreditDaySales, -1000)

	// Update the Gauge metric for product stock
	c.MetricsManager.SetGauge(productStock, 50)

	return "Return Successful", nil
}
