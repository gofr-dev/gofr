package avro

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
	"gofr.dev/pkg/service"
)

func Test_PubSub_SchemaRegistryClient_GetSchemaByVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "error") {
			w.Header().Set("StatusCode", "500")
			_ = json.NewEncoder(w).Encode(`{`)
		}

		w.Header().Set("Content-Type", "application/json")
		respMap := map[string]interface{}{"subject": "gofr-value", "version": 2, "id": 293,
			"schema": `{"type":"record","name":"test","fields":[{"name":"ID","type":"string"}]}`}
		_ = json.NewEncoder(w).Encode(respMap)
	}))

	type args struct {
		subject string
		version string
	}

	tests := []struct {
		name    string
		args    args
		url     string
		want    int
		wantErr bool
	}{
		{"success response", args{version: "latest", subject: "gofr"}, server.URL, 293, false},
		{"error while fetching the schema", args{version: "latest", subject: "error"}, server.URL, 0, true},
		{"http client error", args{version: "10", subject: "error"}, "testURL", 0, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := NewSchemaRegistryClient([]string{tt.url}, "", "")

			got, _, err := client.GetSchemaByVersion(tt.args.subject, tt.args.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSchemaByVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("GetSchemaByVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:gocognit // cannot reduce the complexity further
func Test_PubSub_SchemaRegistryClient_GetSchema(t *testing.T) {
	logger := log.NewLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "0") {
			w.Header().Set("StatusCode", "500")
			_ = json.NewEncoder(w).Encode(`{`)
		}

		w.Header().Set("Content-Type", "application/json")
		respMap := map[string]interface{}{"subject": "gofr-value", "version": 2, "id": 293,
			"schema": `{"type":"record","name":"test","fields":[{"name":"ID","type":"string"}]}`}
		_ = json.NewEncoder(w).Encode(respMap)
	}))

	wantSchema := `{"type":"record","name":"test","fields":[{"name":"ID","type":"string"}]}`

	tests := []struct {
		name       string
		id         int
		clientID   string
		url        string
		wantSchema string
		wantErr    bool
	}{
		{"error in response", 0, "", "server.URL", "", true},
		{"success case with empty clientID", 1, "", server.URL, wantSchema, false},
		{"success case with clientID", 1, "1", server.URL, "1", false},
		{"error from http client", 1, "", "server.URL", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			httpSvc := service.NewHTTPServiceWithOptions(tt.url, logger, nil)

			client := &SchemaRegistryClient{
				SchemaRegistryConnect: []string{tt.url},
				retries:               1,
				httpSvc:               []service.HTTP{httpSvc},
				data:                  map[int]string{},
			}

			client.data[tt.id] = tt.clientID

			got, err := client.GetSchema(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantSchema != "" && got != tt.wantSchema {
				t.Errorf("GetSchema() got = %v, \n\twant %v", got, tt.wantSchema)
			}
		})
	}
}

func Test_PubSub_httpCall(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))

	httpSvc := service.NewHTTPServiceWithOptions(server.URL, logger, nil)

	client := &SchemaRegistryClient{
		SchemaRegistryConnect: []string{server.URL},
		retries:               1,
		httpSvc:               []service.HTTP{httpSvc},
		data:                  map[int]string{},
	}

	_, err := client.httpCall(server.URL)
	assert.IsType(t, errors.ForbiddenRequest{}, err)
}

func Test_PubSub_checkForbiddenRequest(t *testing.T) {
	testcases := []struct {
		testResp    service.Response
		uri         string
		expectedErr error
	}{
		{testResp: service.Response{Body: []byte(""), StatusCode: http.StatusForbidden}, uri: "", expectedErr: errors.ForbiddenRequest{URL: ""}},
		{testResp: service.Response{}, uri: "", expectedErr: nil},
	}

	for _, tc := range testcases {
		err := checkForbiddenRequest(&tc.testResp, tc.uri)
		assert.Equal(t, tc.expectedErr, err)
	}
}
