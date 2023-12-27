package eventhub

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

func TestNew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	conf := config.NewGoDotEnvProvider(log.NewLogger(), "../../../../configs")

	tests := []struct {
		name    string
		c       Config
		wantErr bool
	}{
		{"success connection", Config{
			Namespace:    "zsmisc-dev",
			EventhubName: "healthcheck",
			ClientSecret: conf.Get("AZURE_CLIENT_SECRET"),
			ClientID:     conf.Get("AZURE_CLIENT_ID"),
			TenantID:     conf.Get("AZURE_TENANT_ID"),
		}, false},
		{"error in connection", Config{
			Namespace:    "zsmisc-dev",
			EventhubName: "healthcheck",
			ClientSecret: "AZURE_CLIENT_SECRET",
			ClientID:     "AZURE_CLIENT_ID",
			TenantID:     "AZURE_TENANT_ID",
		}, true},
		{"connecting using SAS", Config{
			Namespace:        "zsmisc-dev",
			EventhubName:     "healthcheck",
			SharedAccessKey:  "dummy-key",
			SharedAccessName: "dummy",
		}, false},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			_, err := New(&tt.c)

			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestEventhub_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	conf := config.NewGoDotEnvProvider(log.NewLogger(), "../../../../configs")
	tests := []struct {
		c    Config
		resp types.Health
	}{
		{Config{Namespace: "zsmisc-dev", EventhubName: "healthcheck",
			ClientSecret: conf.Get("AZURE_CLIENT_SECRET"), ClientID: conf.Get("AZURE_CLIENT_ID"),
			TenantID: conf.Get("AZURE_TENANT_ID")}, types.Health{
			Name: datastore.EventHub, Status: pkg.StatusUp, Host: "zsmisc-dev", Database: "healthcheck"}},
		{Config{Namespace: "host-name", EventhubName: "eventhub"},
			types.Health{Name: datastore.EventHub, Status: pkg.StatusDown, Host: "host-name", Database: "eventhub"}},
	}

	for i, v := range tests {
		testConfig := v.c
		conn, _ := New(&testConfig)

		resp := conn.HealthCheck()
		if !reflect.DeepEqual(resp, v.resp) {
			t.Errorf("[TESTCASE%d]Failed.Got %v\tExpected %v\n", i+1, resp, v.resp)
		}
	}
}

func TestEventhub_HealthCheck_Down(t *testing.T) {
	conf := config.NewGoDotEnvProvider(log.NewLogger(), "../../../../configs")

	{
		// nil conn
		var e Eventhub
		expected := types.Health{
			Name:   datastore.EventHub,
			Status: pkg.StatusDown,
		}

		resp := e.HealthCheck()
		if !reflect.DeepEqual(resp, expected) {
			t.Errorf("Expected %v\tGot %v\n", expected, resp)
		}
	}

	{
		// invalid configs
		c := Config{EventhubName: "eventhub", Namespace: "namespace"}
		expected := types.Health{
			Name:     datastore.EventHub,
			Status:   pkg.StatusDown,
			Host:     c.Namespace,
			Database: c.EventhubName,
		}

		con, _ := New(&c)

		resp := con.HealthCheck()
		if !reflect.DeepEqual(resp, expected) {
			t.Errorf("Expected %v\tGot %v\n", expected, resp)
		}
	}

	{
		// connected but lost connection in between
		c := Config{Namespace: "zsmisc-dev", EventhubName: "healthcheck",
			ClientSecret: conf.Get("AZURE_CLIENT_SECRET"), ClientID: conf.Get("AZURE_CLIENT_ID"),
			TenantID: conf.Get("AZURE_TENANT_ID")}

		expected := types.Health{
			Name:     datastore.EventHub,
			Status:   pkg.StatusDown,
			Host:     c.Namespace,
			Database: c.EventhubName,
		}
		conn, _ := New(&c)

		e, _ := conn.(*Eventhub)
		e.hub = nil

		resp := e.HealthCheck()
		if !reflect.DeepEqual(resp, expected) {
			t.Errorf("Expected %v\tGot %v\n", expected, resp)
		}
	}
}

func TestIsSet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	var e *Eventhub

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../configs")
	conn, _ := New(&Config{Namespace: "zsmisc-dev", EventhubName: "healthcheck",
		ClientSecret: c.Get("AZURE_CLIENT_SECRET"), ClientID: c.Get("AZURE_CLIENT_ID"),
		TenantID: c.Get("AZURE_TENANT_ID")})

	tests := []struct {
		pubsub pubsub.PublisherSubscriber
		resp   bool
	}{
		{e, false},
		{&Eventhub{}, false},
		{conn, true},
	}

	for i, v := range tests {
		resp := v.pubsub.IsSet()
		if resp != v.resp {
			t.Errorf("[TESTCASE%d]Failed.Expected %v\tGot %v\n", i+1, v.resp, resp)
		}
	}
}

func Test_NewEventHubWithAvro(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respMap := map[string]interface{}{"subject": "gofr-value", "version": 2, "id": 293,
			"schema": `{"type":"record","name":"test","fields":[{"name":"ID","type":"string"}]}`}
		_ = json.NewEncoder(w).Encode(respMap)
	}))

	tests := []struct {
		config  AvroWithEventhubConfig
		wantErr bool
	}{
		{
			// success case
			config: AvroWithEventhubConfig{
				EventhubConfig: Config{Namespace: "zsmisc-dev", EventhubName: "healthcheck",
					ClientSecret: c.Get("AZURE_CLIENT_SECRET"), ClientID: c.Get("AZURE_CLIENT_ID"),
					TenantID: c.Get("AZURE_TENANT_ID"), SharedAccessName: "SHARED_ACCESS_NAME", SharedAccessKey: "SHARED_ACCESS_KEY"},
				AvroConfig: avro.Config{URL: server.URL, Version: "", Subject: "gofr-value"},
			}, wantErr: false,
		},
		{
			// failure due wrong eventhub config, so it will not check the avro config
			config: AvroWithEventhubConfig{
				EventhubConfig: Config{Namespace: "zsmisc-dev", EventhubName: "healthcheck",
					ClientSecret: "Fake Client Secret", ClientID: "Fake ClientID",
					TenantID: "Fake TenantID"},
				AvroConfig: avro.Config{
					URL: "dummy-url.com", Subject: "gofr-value",
				},
			}, wantErr: true,
		},
		{
			// failure due to wrong avroConfig
			config: AvroWithEventhubConfig{
				EventhubConfig: Config{Namespace: "zsmisc-dev", EventhubName: "healthcheck",
					ClientSecret: c.Get("AZURE_CLIENT_SECRET"), ClientID: c.Get("AZURE_CLIENT_ID"),
					TenantID: c.Get("AZURE_TENANT_ID"), SharedAccessName: "SHARED_ACCESS_NAME", SharedAccessKey: "SHARED_ACCESS_KEY"},
				AvroConfig: avro.Config{
					URL: "dummy-url.com", Subject: "gofr-value",
				},
			}, wantErr: true,
		},
	}

	for i, tc := range tests {
		tcConfig := tc.config
		_, err := NewEventHubWithAvro(&tcConfig, logger)

		if !tc.wantErr && err != nil {
			t.Errorf("FAILED[%v], expected: %v, got: %v", i+1, tc.wantErr, true)
		}

		if tc.wantErr && err == nil {
			t.Errorf("FAILED[%v], expected: %v, got: %v", i+1, tc.wantErr, false)
		}
	}
}

func TestEventhub_BindError(t *testing.T) {
	svc := &Eventhub{}
	message := map[string]interface{}{}

	val := []byte(`{`)

	err := svc.Bind(val, &message)

	assert.Error(t, err, "Test case failed")
}
