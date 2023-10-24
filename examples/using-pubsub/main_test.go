package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gofr.dev/examples/using-pubsub/handlers"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/request"
)

//nolint:gocognit // need to wait for topic to be created so retry logic is to be added
func TestServerRun(t *testing.T) {
	t.Setenv("KAFKA_TOPIC", "kafka-pubsub")

	go main()
	time.Sleep(3 * time.Second)

	tests := []struct {
		desc       string
		method     string
		endpoint   string
		resp       string
		statusCode int
	}{
		{"publish", http.MethodGet, "http://localhost:9112/pub?id=1", "", http.StatusOK},
		{"subscribe", http.MethodGet, "http://localhost:9112/sub", "1", http.StatusOK},
	}

	for i, tc := range tests {
		req, _ := request.NewMock(tc.method, tc.endpoint, nil)
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
					time.Sleep(5 * time.Second)
					continue
				}

				t.Errorf("Testcase[%v] FAILED.\tExpected %v\tGot %v\n%s", i, http.StatusOK, resp.StatusCode, tc.desc)

				break
			}

			if resp.StatusCode != tc.statusCode {
				t.Errorf("Testcase[%v] FAILED.\tExpected %v\tGot %v\n%s", i, tc.statusCode, resp.StatusCode, tc.desc)
			}

			// checks whether bind avro.Unmarshal functionality works fine
			if tc.resp != "" && resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)

				m := struct {
					Data handlers.Person `json:"data"`
				}{}
				_ = json.Unmarshal(body, &m)

				if m.Data.ID != tc.resp {
					t.Errorf("Testcase[%v] FAILED.\tExpected: %v, Got: %v\n%s", i, tc.resp, m.Data.ID, tc.desc)
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

	_ = json.Unmarshal(body, &errResp)

	return strings.Contains(errResp.Errors[0].Reason, "Leader Not Available: the cluster is in the middle of a leadership election")
}
