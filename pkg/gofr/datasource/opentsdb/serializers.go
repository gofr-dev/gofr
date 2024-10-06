package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type SerialResponse struct {
	StatusCode  int
	Serializers []Serializer `json:"Serializers"`
}

type Serializer struct {
	SerializerName string   `json:"serializer"`
	Formatters     []string `json:"formatters"`
	Parsers        []string `json:"parsers"`
	Class          string   `json:"class,omitempty"`
	ResContType    string   `json:"response_content_type,omitempty"`
	ReqContType    string   `json:"request_content_type,omitempty"`
}

func (serialResp *SerialResponse) SetStatus(code int) {
	serialResp.StatusCode = code
}

func (serialResp *SerialResponse) GetCustomParser() func(respCnt []byte) error {
	return func(respCnt []byte) error {
		return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"Serializers"`, string(respCnt))), &serialResp)
	}
}

func (serialResp *SerialResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(serialResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

func (c *OpentsdbClient) Serializers() (*SerialResponse, error) {
	serialEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, SerializersPath)
	serialResp := SerialResponse{}
	if err := c.sendRequest(GetMethod, serialEndpoint, "", &serialResp); err != nil {
		return nil, err
	}
	return &serialResp, nil
}
