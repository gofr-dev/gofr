package service

import "time"

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
	ErrorMessage string `json:"error_message"`
}
