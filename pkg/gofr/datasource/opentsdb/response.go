package opentsdb

import (
	"context"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"strings"
	"time"
)

// AggregatorsResponse acts as the implementation of Response in the /api/aggregators.
// It holds the status code and the response values defined in the
// [OpenTSDB Official Docs]: http://opentsdb.net/docs/build/html/api_http/aggregators.html.
type AggregatorsResponse struct {
	StatusCode  int
	Aggregators []string `json:"aggregators"`
	logger      Logger
	tracer      trace.Tracer
	ctx         context.Context
}

// VersionResponse is the struct implementation for /api/version.
type VersionResponse struct {
	StatusCode  int
	VersionInfo map[string]any
	logger      Logger
	tracer      trace.Tracer
	ctx         context.Context
}

func (*PutResponse) getCustomParser() func(respCnt []byte) error {
	return nil
}

func (queryResp *QueryResponse) getCustomParser() func(respCnt []byte) error {
	return queryParserHelper(queryResp.ctx, queryResp.logger, queryResp, "GetCustomParser-Query")
}

func (queryLastResp *QueryLastResponse) getCustomParser() func(respCnt []byte) error {
	return queryParserHelper(queryLastResp.ctx, queryLastResp.logger, queryLastResp, "GetCustomParser-QueryLast")
}

func (verResp *VersionResponse) getCustomParser() func(respCnt []byte) error {
	return customParserHelper(verResp.ctx, verResp, "GetCustomParser-VersionResp", verResp.logger,
		func(resp []byte) error {
			v := make(map[string]any, 0)

			err := json.Unmarshal(resp, &v)
			if err != nil {
				return err
			}

			verResp.VersionInfo = v

			return nil
		})
}

func (annotResp *AnnotationResponse) getCustomParser() func(respCnt []byte) error {
	return customParserHelper(annotResp.ctx, annotResp, "getCustomParser-Annotation", annotResp.logger,
		func(resp []byte) error {
			if len(resp) == 0 {
				return nil
			}

			return json.Unmarshal(resp, &annotResp)
		})
}

func (aggreResp *AggregatorsResponse) getCustomParser() func(respCnt []byte) error {
	return customParserHelper(aggreResp.ctx, aggreResp, "GetCustomParser-Aggregator", aggreResp.logger,
		func(resp []byte) error {
			j := make([]string, 0)

			err := json.Unmarshal(resp, &j)
			if err != nil {
				return err
			}

			aggreResp.Aggregators = j

			return nil
		})
}

type genericResponse interface {
	addTrace(ctx context.Context, operation string) trace.Span
}

// sendRequest dispatches an HTTP request to the OpenTSDB server, using the provided
// method, URL, and body content. It returns the parsed response or an error, if any.
func (c *Client) sendRequest(ctx context.Context, method, url, reqBodyCnt string, parsedResp Response) error {
	span := c.addTrace(ctx, "sendRequest")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "sendRequest", &status, &message, span)

	// Create the HTTP request, attaching the context if available.
	req, err := http.NewRequest(method, url, strings.NewReader(reqBodyCnt))
	if ctx != nil {
		req = req.WithContext(ctx)
	}

	if err != nil {
		errRequestCreation := fmt.Errorf("failed to create request for %s %s: %w", method, url, err)

		message = fmt.Sprint(errRequestCreation)

		return errRequestCreation
	}

	// Set the request headers.
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	// Send the request and handle the response.
	resp, err := c.client.Do(req)
	if err != nil {
		errSendingRequest := fmt.Errorf("failed to send request for %s %s: %w", method, url, err)

		message = fmt.Sprint(errSendingRequest)

		return errSendingRequest
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("client error: %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	// Read and parse the response.
	jsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		errReading := fmt.Errorf("failed to read response body for %s %s: %w", method, url, err)

		message = fmt.Sprint(errReading)

		return errReading
	}

	parser := parsedResp.getCustomParser()
	if parser == nil {
		// Use the default JSON unmarshaller if no custom parser is provided.
		if err = json.Unmarshal(jsonBytes, parsedResp); err != nil {
			errUnmarshaling := fmt.Errorf("failed to unmarshal response body for %s %s: %w", method, url, err)

			message = fmt.Sprint(errUnmarshaling)

			return errUnmarshaling
		}
	} else {
		// Use the custom parser if available.
		if err := parser(jsonBytes); err != nil {
			message = fmt.Sprintf("failed to parse response body through custom parser %s %s: %v", method, url, err)
			return err
		}
	}

	status = StatusSuccess
	message = fmt.Sprintf("%s request sent at : %s", method, url)

	return nil
}

func (c *Client) version(ctx context.Context, verResp *VersionResponse) error {
	span := c.addTrace(ctx, "Version")

	status := StatusFailed

	var message string

	defer sendOperationStats(c.logger, time.Now(), "Version", &status, &message, span)

	verEndpoint := fmt.Sprintf("%s%s", c.endpoint, VersionPath)
	verResp.logger = c.logger
	verResp.tracer = c.tracer
	verResp.ctx = ctx

	if err := c.sendRequest(ctx, http.MethodGet, verEndpoint, "", verResp); err != nil {
		message = fmt.Sprintf("error while processing request at URL %s: %s", verEndpoint, err)
		return err
	}

	status = StatusSuccess
	message = "version response retrieved successfully."

	return nil
}

// isValidOperateMethod checks if the provided HTTP method is valid for
// operations such as POST, PUT, or DELETE.
func (c *Client) isValidOperateMethod(ctx context.Context, method string) bool {
	span := c.addTrace(ctx, "isValidOperateMethod")

	status := StatusSuccess

	var message string

	defer sendOperationStats(c.logger, time.Now(), "isValidOperateMethod", &status, &message, span)

	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return false
	}

	validMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPut}
	for _, validMethod := range validMethods {
		if method == validMethod {
			return true
		}
	}

	return false
}

func customParserHelper(ctx context.Context, resp genericResponse, operation string, logger Logger,
	unmarshalFunc func([]byte) error) func([]byte) error {
	span := resp.addTrace(ctx, operation)

	status := StatusFailed

	var message string

	defer sendOperationStats(logger, time.Now(), operation, &status, &message, span)

	return func(result []byte) error {
		err := unmarshalFunc(result)
		if err != nil {
			message = fmt.Sprintf("unmarshal %s error: %s", operation, err)
			logger.Errorf(message)

			return err
		}

		status = StatusSuccess
		message = fmt.Sprintf("%s custom parsing was successful.", operation)

		return nil
	}
}

func queryParserHelper(ctx context.Context, logger Logger, obj genericResponse, methodName string) func(respCnt []byte) error {
	return customParserHelper(ctx, obj, methodName, logger, func(resp []byte) error {
		originRespStr := string(resp)

		var respStr string

		if len(resp) != 0 && resp[0] == '[' && resp[len(resp)-1] == ']' {
			respStr = fmt.Sprintf(`{"queryRespCnts":%s}`, originRespStr)
		} else {
			respStr = originRespStr
		}

		return json.Unmarshal([]byte(respStr), obj)
	})
}
