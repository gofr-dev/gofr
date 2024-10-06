package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type DropcachesResponse struct {
	StatusCode     int
	DropcachesInfo map[string]string `json:"DropcachesInfo"`
}

func (dropResp *DropcachesResponse) SetStatus(code int) {
	dropResp.StatusCode = code
}

func (dropResp *DropcachesResponse) GetCustomParser() func(respCnt []byte) error {
	return func(respCnt []byte) error {
		return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"DropcachesInfo"`, string(respCnt))), &dropResp)
	}
}

func (dropResp *DropcachesResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(dropResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

func (c *OpentsdbClient) Dropcaches() (*DropcachesResponse, error) {
	dropEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, DropcachesPath)
	dropResp := DropcachesResponse{}
	if err := c.sendRequest(GetMethod, dropEndpoint, "", &dropResp); err != nil {
		return nil, err
	}
	return &dropResp, nil
}
