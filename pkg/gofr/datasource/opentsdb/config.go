package opentsdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"time"
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
	logger     Logger
	tracer     trace.Tracer
}

func (cfgResp *ConfigResponse) SetStatus(ctx context.Context, code int) {
	_, span := cfgResp.addTrace(ctx, "SetStatus")

	status := "SUCCESS"
	var message string

	defer sendOperationStats(cfgResp.logger, time.Now(), "SetStatus-AggregatorResp", &status, &message, span)
	message = fmt.Sprintf("set response code : %d", code)

	cfgResp.StatusCode = code
}

func (cfgResp *ConfigResponse) GetCustomParser(ctx context.Context) func(respCnt []byte) error {
	_, span := cfgResp.addTrace(ctx, "GetCustomParser")

	status := "FAIL"
	var message string

	defer sendOperationStats(cfgResp.logger, time.Now(), "GetCustomParser-AggregatorResp", &status, &message, span)

	return func(resp []byte) error {
		err := json.Unmarshal([]byte(fmt.Sprintf("{%s:%s}", `"aggregators"`, string(resp))), &cfgResp)
		if err != nil {
			message = fmt.Sprintf("unmarshal cfgResp response error: %s", err)
			cfgResp.logger.Errorf(message)

			return err
		}

		status = "SUCCESS"
		message = fmt.Sprint("Custom parsing successful")

		return nil
	}
}

func (cfgResp *ConfigResponse) String(ctx context.Context) string {
	_, span := cfgResp.addTrace(ctx, "ToString")

	status := "FAIL"
	var message string

	defer sendOperationStats(cfgResp.logger, time.Now(), "ToString-ConfigResp", &status, &message, span)

	buffer := bytes.NewBuffer(nil)

	content, err := json.Marshal(cfgResp)
	if err != nil {
		message = fmt.Sprintf("marshal config response error: %s", err.Error())
		cfgResp.logger.Errorf(message)
	}
	buffer.WriteString(fmt.Sprintf("%s\n", string(content)))

	status = "SUCCESS"
	message = fmt.Sprint("config response converted to string successfully")

	return buffer.String()
}

func (c *OpentsdbClient) Config() (*ConfigResponse, error) {
	tracedctx, span := c.addTrace(c.ctx, "Config")
	c.ctx = tracedctx

	status := "FAIL"
	var message string

	defer sendOperationStats(c.logger, time.Now(), "Config", &status, &message, span)

	configEndpoint := fmt.Sprintf("%s%s", c.tsdbEndpoint, ConfigPath)
	cfgResp := ConfigResponse{logger: c.logger, tracer: c.tracer}
	if err := c.sendRequest(GetMethod, configEndpoint, "", &cfgResp); err != nil {

		message = fmt.Sprintf("error while processing request at url %s: %s", configEndpoint, err)
		return nil, err
	}

	status = "SUCCESS"
	message = fmt.Sprint("config response retrieved successfully")

	return &cfgResp, nil
}
