package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gofr.dev/examples/using-pubsub/handlers"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/request"
)

//nolint:gocognit // need to wait for topic to be created so retry logic is to be added
func Test_PubSub_ServerRun(t *testing.T) {
	// avro schema registry test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := map[string]interface{}{
			"subject": "gofr-value",
			"version": 3,
			"id":      303,
			"schema": "{\"type\":\"record\",\"name\":\"person\"," +
				"\"fields\":[{\"name\":\"Id\",\"type\":\"string\"}," +
				"{\"name\":\"Name\",\"type\":\"string\"}," +
				"{\"name\":\"Email\",\"type\":\"string\"}]}",
		}

		reBytes, _ := json.Marshal(re)
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	t.Setenv("AVRO_SCHEMA_URL", ts.URL)

	t.Setenv("KAFKA_TOPIC", "avro-pubsub")

	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		desc       string
		endpoint   string
		resp       string
		statusCode int
	}{
		{"produce", "/pub?id=1", "", http.StatusOK},
		{"consume", "/sub", "1", http.StatusOK},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(http.MethodGet, "http://localhost:9111"+tc.endpoint, nil)
		c := http.Client{}

		for j := 0; j < 5; j++ {
			resp, err := c.Do(req)
			if err != nil {
				t.Errorf("TEST[%v] Failed.\tHTTP request encountered Err: %v\n%s", i, err, tc.desc)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				// retry is required since, creation of topic takes time
				if checkRetry(resp.Body) {
					time.Sleep(3 * time.Second)
					continue
				}

				t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)

				break
			}

			if resp.StatusCode != tc.statusCode {
				t.Errorf("TEST[%v] Failed.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
			}

			// checks whether bind avro.Unmarshal functionality works fine
			if tc.resp != "" && resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)

				m := struct {
					Data handlers.Person `json:"data"`
				}{}
				_ = json.Unmarshal(body, &m)

				if m.Data.ID != tc.resp {
					t.Errorf("TEST[%v] FAILED.\tExpected: %v,\tGot: %v\n%s", i, tc.resp, m.Data.ID, tc.desc)
				}
			}

			_ = resp.Body.Close()

			break
		}
	}
}

func checkRetry(respBody io.Reader) bool {
	body, _ := io.ReadAll(respBody)

	errResp := struct {
		Errors []errors.Response `json:"errors"`
	}{}

	if len(errResp.Errors) == 0 {
		return false
	}

	_ = json.Unmarshal(body, &errResp)

	return strings.Contains(errResp.Errors[0].Reason, "Leader Not Available: the cluster is in the middle of a leadership election")
}
