package service

type HTTPCallLog struct {
	MessageId    string `json:"messageId"`
	ResponseCode int    `json:"responseCode"`
	ResponseTime int64  `json:"responseTime"`
	Method       string `json:"method"`
	URI          string `json:"uri"`
}
