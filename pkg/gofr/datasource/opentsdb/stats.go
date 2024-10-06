package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type StatsResponse struct {
	StatusCode int
	Metrics    []MetricInfo `json:"Metrics"`
}

type MetricInfo struct {
	Metric    string            `json:"metric"`
	Timestamp int64             `json:"timestamp"`
	Value     interface{}       `json:"value"`
	Tags      map[string]string `json:"tags"`
}

func (statsResp *StatsResponse) SetStatus(code int) {
	statsResp.StatusCode = code
}

func (statsResp *StatsResponse) GetCustomParser() func(respCnt []byte) error {
	return func(respCnt []byte) error {
		return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"Metrics"`, string(respCnt))), &statsResp)
	}
}

func (statsResp *StatsResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(statsResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

func (c *OpentsdbClient) Stats() (*StatsResponse, error) {
	statsEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, StatsPath)
	statsResp := StatsResponse{}
	if err := c.sendRequest(GetMethod, statsEndpoint, "", &statsResp); err != nil {
		return nil, err
	}
	return &statsResp, nil
}
