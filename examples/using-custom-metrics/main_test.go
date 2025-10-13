package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func TestIntegration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(100 * time.Millisecond) // Giving some time to start the server

	c := http.Client{}

	req, _ := http.NewRequest(http.MethodPost, configs.HTTPHost+"/transaction", nil)
	req.Header.Set("content-type", "application/json")

	_, err := c.Do(req)
	if err != nil {
		t.Fatalf("request to /transaction failed %v", err)
	}

	req, _ = http.NewRequest(http.MethodPost, configs.HTTPHost+"/return", nil)

	_, err = c.Do(req)
	if err != nil {
		t.Fatalf("request to /transaction failed %v", err)
	}

	req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d/metrics", configs.MetricsPort), nil)

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("request to localhost:%d/metrics failed: %v", configs.MetricsPort, err)
	}

	body, _ := io.ReadAll(resp.Body)

	strBody := string(body)

	assert.Equal(t, http.StatusOK, resp.StatusCode, "TEST[%d], Failed.\n%s")

	assert.Contains(t, strBody, `product_stock{otel_scope_name="using-metrics",otel_scope_schema_url="",otel_scope_version="v0.1.0"} 50`)
	assert.Contains(t, strBody, `total_credit_day_sale{otel_scope_name="using-metrics",otel_scope_schema_url="",otel_scope_version="v0.1.0",sale_type="credit"} 1000`)
	assert.Contains(t, strBody, `total_credit_day_sale{otel_scope_name="using-metrics",otel_scope_schema_url="",otel_scope_version="v0.1.0",sale_type="credit_return"} -1000`)
	assert.Contains(t, strBody, `transaction_success{otel_scope_name="using-metrics",otel_scope_schema_url="",otel_scope_version="v0.1.0"} 1`)
	assert.Contains(t, strBody, "transaction_time")
}
