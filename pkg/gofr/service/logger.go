package service

type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type HTTPCallLog struct {
	MessageId    string `json:"messageId"`
	ResponseCode int    `json:"responseCode"`
	ResponseTime int64  `json:"responseTime"`
	Method       string `json:"method"`
	URI          string `json:"uri"`
}
