package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type OpenTSDBConfig struct {

	// The host of the target opentsdb, is a required non-empty string which is
	// in the format of ip:port without http:// prefix or a domain.
	OpentsdbHost string

	// A pointer of http.Tranport is used by the opentsdb client.
	// This value is optional, and if it is not set, client.DefaultTransport, which
	// enables tcp keepalive mode, will be used in the opentsdb client.
	Transport *http.Transport

	// The maximal number of datapoints which will be inserted into the opentsdb
	// via one calling of /api/put method.
	// This value is optional, and if it is not set, client.DefaultMaxPutPointsNum
	// will be used in the opentsdb client.
	MaxPutPointsNum int

	// The detect delta number of datapoints which will be used in client.Put()
	// to split a large group of datapoints into small batches.
	// This value is optional, and if it is not set, client.DefaultDetectDeltaNum
	// will be used in the opentsdb client.
	DetectDeltaNum int

	// The maximal body content length per /api/put method to insert datapoints
	// into opentsdb.
	// This value is optional, and if it is not set, client.DefaultMaxPutPointsNum
	// will be used in the opentsdb client.
	MaxContentLength int
}

type ConfigResponse struct {
	StatusCode int
	Configs    map[string]string `json:"configs"`
}

func (cfgResp *ConfigResponse) SetStatus(code int) {
	cfgResp.StatusCode = code
}

func (cfgResp *ConfigResponse) GetCustomParser() func(respCnt []byte) error {
	return func(respCnt []byte) error {
		return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"Configs"`, string(respCnt))), &cfgResp)
	}
}

func (cfgResp *ConfigResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(cfgResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

func (c *OpentsdbClient) Config() (*ConfigResponse, error) {
	configEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, ConfigPath)
	cfgResp := ConfigResponse{}
	if err := c.sendRequest(GetMethod, configEndpoint, "", &cfgResp); err != nil {
		return nil, err
	}
	return &cfgResp, nil
}
