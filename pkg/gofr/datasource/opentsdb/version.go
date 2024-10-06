// version.go contains the structs and methods for the implementation of /api/version.
package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]string `json:"VersionInfo"`
}

func (verResp *VersionResponse) SetStatus(code int) {
	verResp.StatusCode = code
}

func (verResp *VersionResponse) GetCustomParser() func(respCnt []byte) error {
	return func(respCnt []byte) error {
		return json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"VersionInfo"`, string(respCnt))), &verResp)
	}
}

func (verResp *VersionResponse) String() string {
	buffer := bytes.NewBuffer(nil)
	content, _ := json.Marshal(verResp)
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))
	return buffer.String()
}

func (c *OpentsdbClient) Version() (*VersionResponse, error) {
	verEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, VersionPath)
	verResp := VersionResponse{}
	if err := c.sendRequest(GetMethod, verEndpoint, "", &verResp); err != nil {
		return nil, err
	}
	return &verResp, nil
}
