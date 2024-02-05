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

	err := a.Metrics().NewCounter(transactionSuccessful, "used to track the count of successful transactions")
	if err != nil {
		fmt.Printf("failed to register metric %v : %v\n", transactionSuccessful, err)
	}

	err = a.Metrics().NewUpDownCounter(totalCreditDaySales, "used to track the total credit sales in a day")
	if err != nil {
		fmt.Printf("failed to register metric %v : %v\n", totalCreditDaySales, err)
	}

	err = a.Metrics().NewHistogram(transactionTime, "used to track the time taken by a transaction",
		5, 10, 15, 20, 25, 35)
	if err != nil {
		fmt.Printf("failed to register metric %v : %v\n", transactionTime, err)
	}

	err = a.Metrics().NewGauge(productStock, "used to track the number of products in stock")
	if err != nil {
		fmt.Printf("failed to register metric %v : %v\n", productStock, err)
	}

	// Add all the routes
	a.POST("/transaction", TransactionHandler)
	a.POST("/return", ReturnHandler)

	// Run the application
	a.Run()
}

func TransactionHandler(c *gofr.Context) (interface{}, error) {
	// transaction logic

	transactionStartTime := time.Now()

	err := c.MetricsManager.IncCounter(c, transactionSuccessful)
	if err != nil {
		c.Logger.Info("unable to increase the metric %v : %v", transactionSuccessful, err)
	}

	tranTime := time.Now().Sub(transactionStartTime).Microseconds()

	c.MetricsManager.RecordHistogram(c, transactionTime, float64(tranTime))
	if err != nil {
		c.Logger.Info("unable to increase the metric %v : %v", totalCreditDaySales, err)
	}

	err = c.MetricsManager.DeltaUpDownCounter(c, totalCreditDaySales, 1000)
	if err != nil {
		c.Logger.Info("unable to increase the metric %v : %v", totalCreditDaySales, err)
	}

	err = c.MetricsManager.SetGauge(productStock, 10)
	if err != nil {
		c.Logger.Info("unable to set the gauge metric %v : %v", productStock, err)
	}

	return fmt.Sprintf("transaction successful"), nil
}

func ReturnHandler(c *gofr.Context) (interface{}, error) {
	// logic to create a sales return

	err := c.MetricsManager.DeltaUpDownCounter(c, totalCreditDaySales, -1000)
	if err != nil {
		c.Logger.Info("unable to increase the metric %v : %v", totalCreditDaySales, err)
	}

	// Update the Gauge metric for product stock
	err = c.MetricsManager.SetGauge(productStock, 50)
	if err != nil {
		c.Logger.Info("unable to set the gauge metric %v : %v", productStock, err)
	}

	return "Return Successful", nil
}
