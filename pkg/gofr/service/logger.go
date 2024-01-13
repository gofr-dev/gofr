package service

import "time"

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type Log struct {
	Timestamp     time.Time `json:"timestamp"`
	ResponseTime  int64     `json:"latency"`
	CorrelationID string    `json:"correlationId"`
	ResponseCode  int       `json:"responseCode"`
	HTTPMethod    string    `json:"http_method"`
	Endpoint      string    `json:"endpoint"`
	URI           string    `json:"uri"`
}

type ErrorLog struct {
	Log
	ErrorMessage string `json:"error_message"`
}
