package main

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntegration(t *testing.T) {
	const host = "http://localhost:9011"
	go main()
	time.Sleep(time.Second * 1) // Giving some time to start the server

	c := http.Client{}

	req, _ := http.NewRequest(http.MethodPost, host+"/transaction", nil)
	req.Header.Set("content-type", "application/json")

	_, err := c.Do(req)
	if err != nil {
		t.Fatalf("request to /transaction failed %v", err)
	}

	req, _ = http.NewRequest(http.MethodPost, host+"/return", nil)

	_, err = c.Do(req)
	if err != nil {
		t.Fatalf("request to /transaction failed %v", err)
	}

	req, _ = http.NewRequest(http.MethodGet, "http://localhost:2120/metrics", nil)

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("request to localhost:2120/metrics failed %v", err)
	}

	body, _ := io.ReadAll(resp.Body)

	strBody := string(body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST[%d], Failed.\n%s")

	assert.Contains(t, strBody, `product_stock{otel_scope_name="using-metrics",otel_scope_version="v0.1.0"} 50`)
	assert.Contains(t, strBody, `total_credit_day_sale{otel_scope_name="using-metrics",otel_scope_version="v0.1.0",sale_type="credit"} 1000`)
	assert.Contains(t, strBody, `total_credit_day_sale{otel_scope_name="using-metrics",otel_scope_version="v0.1.0",sale_type="credit_return"} -1000`)
	assert.Contains(t, strBody, `transaction_success_total{otel_scope_name="using-metrics",otel_scope_version="v0.1.0"} 1`)
	assert.Contains(t, strBody, "transaction_time")
}
