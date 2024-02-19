package service

import (
	"context"
	"time"
)

type Logger interface {
	Log(args ...interface{})
}

type Log struct {
	Timestamp     time.Time `json:"timestamp"`
	ResponseTime  int64     `json:"latency"`
	CorrelationID string    `json:"correlationId"`
	ResponseCode  int       `json:"responseCode"`
	HTTPMethod    string    `json:"httpMethod"`
	URI           string    `json:"uri"`
}

type ErrorLog struct {
	Log
	ErrorMessage string `json:"errorMessage"`
}

type Metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64)
}
