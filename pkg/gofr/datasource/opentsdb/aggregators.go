package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators scene.
// It holds the status code and the response values defined in the
// (http://opentsdb.net/docs/build/html/api_http/aggregators.html).
type AggregatorsResponse struct {
	StatusCode  int
	Aggregators []string `json:"aggregators"`
}

func (aggreResp *AggregatorsResponse) SetStatus(code int) {
	aggreResp.StatusCode = code
}

func (aggreResp *AggregatorsResponse) GetCustomParser() func(resp []byte) error {
	return func(resp []byte) error {
		return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"aggregators"`, string(resp))), &aggreResp)
	}
}

func (aggreResp *AggregatorsResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(aggreResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

func (c *OpentsdbClient) Aggregators() (*AggregatorsResponse, error) {
	aggregatorsEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, AggregatorPath)
	aggreResp := AggregatorsResponse{}
	if err := c.sendRequest(GetMethod, aggregatorsEndpoint, "", &aggreResp); err != nil {
		return nil, err
	}
	return &aggreResp, nil
}
