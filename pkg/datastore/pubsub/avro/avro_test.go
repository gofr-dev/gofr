package avro

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/types"
)

type mockPubSub struct {
	Param string
}

func (m *mockPubSub) CommitOffset(pubsub.TopicPartition) {
}

func (m *mockPubSub) PublishEventWithOptions(string, interface{}, map[string]string, *pubsub.PublishOptions) error {
	return nil
}

func (m *mockPubSub) PublishEvent(string, interface{}, map[string]string) error {
	return nil
}

func (m *mockPubSub) Subscribe() (*pubsub.Message, error) {
	if m.Param == "error" {
		return nil, &errors.Response{Reason: "test error"}
	}

	binarySchemaID := []byte(`00000`)
	if m.Param == "id" {
		binarySchemaID = []byte(`00001`)
	}

	return &pubsub.Message{
		SchemaID: 1,
		Topic:    "test_topic",
		Key:      "test",
		Value:    string(binarySchemaID) + `{"name": "test"}`,
		Headers:  map[string]string{"name": "avro-test"},
	}, nil
}

func (m *mockPubSub) SubscribeWithCommit(pubsub.CommitFunc) (*pubsub.Message, error) {
	return m.Subscribe()
}

func (m *mockPubSub) Bind([]byte, interface{}) error {
	return nil
}

func (m *mockPubSub) Ping() error {
	return nil
}

func (m *mockPubSub) HealthCheck() types.Health {
	if m.Param == "kafka" {
		return types.Health{
			Name:     m.Param,
			Status:   pkg.StatusUp,
			Database: datastore.Kafka,
		}
	}

	return types.Health{
		Name:   m.Param,
		Status: pkg.StatusDown,
	}
}

func (m *mockPubSub) IsSet() bool {
	return true
}

type mockSchemaClient struct {
}

func (m *mockSchemaClient) GetSchemaByVersion(subject, _ string) (id int, s string, err error) {
	if subject == "error" {
		return 0, "", &errors.Response{Reason: "test error"}
	}

	schema := `{"name": "name", "type": "string"}`

	return 1, schema, nil
}
func (m *mockSchemaClient) GetSchema(id int) (string, error) {
	if id == 808464433 {
		schema := `{"name": "name", "type": "string"}`
		return schema, nil
	}

	return "", &errors.Response{Reason: "test error"}
}

//nolint:gocognit // reducing the cognitive complexity so all the test cases can be considered
func Test_PubSub_Avro_Publish(t *testing.T) {
	type args struct {
		key   string
		value interface{}
	}

	tests := []struct {
		name             string
		args             args
		mockPubSub       *mockPubSub
		mockSchemaClient *mockSchemaClient
		subject          string
		pubErr           bool
		wantErr          bool
	}{
		{"error converting native to binary", args{key: "testKey", value: nil}, &mockPubSub{}, &mockSchemaClient{}, "test_topic", true, false},
		{"error fetching schema", args{key: "testKey", value: `{"name": "test"}`}, &mockPubSub{}, &mockSchemaClient{}, "error", true, true},
		{"success", args{key: "testKey", value: `{"name": "Rohan"}`}, &mockPubSub{}, &mockSchemaClient{}, "test_topic", false, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			a, err := New(tt.mockPubSub, tt.mockSchemaClient, "latest", tt.subject)

			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if err := a.PublishEvent(tt.args.key, tt.args.value, nil); (err != nil) != tt.pubErr {
					t.Errorf("PublishEvent() error = %v, wantErr %v", err, tt.pubErr)
				}
			}
		})
	}
}

func Test_PubSub_Avro_Subscribe(t *testing.T) {
	tests := []struct {
		name             string
		mockPubSub       pubsub.PublisherSubscriber
		mockSchemaClient *mockSchemaClient
		wantErr          bool
	}{
		{"error from subscribe", &mockPubSub{"error"}, &mockSchemaClient{}, true},
		{"unable to fetch schema", &mockPubSub{}, &mockSchemaClient{}, true},
		{"success", &mockPubSub{"id"}, &mockSchemaClient{}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			a, _ := New(tt.mockPubSub, tt.mockSchemaClient, "latest", "test_topic")

			msg, err := a.Subscribe()
			if (err != nil) != tt.wantErr {
				t.Errorf("Subscribe() error = %v, wantErr %v", err, tt.wantErr)
			}

			if msg != nil && len(msg.Headers) == 0 {
				t.Error("Subscribe() headers expected, got empty headers")
			}
		})
	}
}

func Test_PubSub_Avro_SubscribeWithCommit(t *testing.T) {
	commitFunc := func(msg *pubsub.Message) (bool, bool) {
		return true, false
	}

	tests := []struct {
		name             string
		mockPubSub       pubsub.PublisherSubscriber
		mockSchemaClient *mockSchemaClient
		wantErr          bool
	}{
		{"error from subscribe", &mockPubSub{"error"}, &mockSchemaClient{}, true},
		{"unable to fetch schema", &mockPubSub{}, &mockSchemaClient{}, true},
		{"success commit and stop consuming", &mockPubSub{"id"}, &mockSchemaClient{}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			a, _ := New(tt.mockPubSub, tt.mockSchemaClient, "latest", "test_topic")

			if _, err := a.SubscribeWithCommit(commitFunc); (err != nil) != tt.wantErr {
				t.Errorf("Subscribe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_PubSub_Avro_Ping(t *testing.T) {
	m := mockPubSub{}
	sc := mockSchemaClient{}
	a, _ := New(&m, &sc, "latest", "test_topic")

	if err := a.Ping(); err != nil {
		t.Errorf("FAILED, expected successful ping")
	}
}

func Test_PubSub_Avro_HealthCheck(t *testing.T) {
	tests := []struct {
		desc    string
		m       mockPubSub
		expResp types.Health
	}{
		{"valid case: kafka", mockPubSub{"kafka"}, types.Health{Name: "kafka", Status: pkg.StatusUp, Database: datastore.Kafka}},
		{"invalid case", mockPubSub{"invalid"}, types.Health{Name: "invalid", Status: pkg.StatusDown}},
	}

	for i, tc := range tests {
		sc := mockSchemaClient{}
		a, _ := New(&tc.m, &sc, "latest", "test_topic")

		resp := a.HealthCheck()

		assert.Equal(t, tc.expResp, resp, "Testcase [%d] failed.", i)
	}
}

func Test_PubSub_Avro_IsSet(t *testing.T) {
	var a *Avro
	tests := []struct {
		a    *Avro
		resp bool
	}{
		{a, false},
		{&Avro{}, false},
	}

	for i, v := range tests {
		resp := v.a.IsSet()
		if resp != v.resp {
			t.Errorf("[TESTCASE%d]Failed.Expected %v\tGot %v\n", i+1, v.resp, resp)
		}
	}
}

func Test_PubSub_NewAvro(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respMap := map[string]interface{}{"subject": "gofr-value", "version": 2, "id": 293,
			"schema": `{"type":"record","name":"test","fields":[{"name":"ID","type":"string"}]}`}
		_ = json.NewEncoder(w).Encode(respMap)
	}))

	var mockPubSub pubsub.PublisherSubscriber

	tests := []struct {
		desc string
		cfgs *Config
	}{
		{"empty configs", &Config{URL: "", Subject: ""}},
		{"without avro subject", &Config{URL: server.URL, Subject: ""}},
		{"with avro subject", &Config{URL: server.URL, Subject: "gofr-value"}},
	}

	for i, tc := range tests {
		_, err := NewWithConfig(tc.cfgs, mockPubSub)

		assert.Nil(t, err, "Testcase [%d] failed.", i)
	}
}

func Test_PubSub__NewAvroError(t *testing.T) {
	forbiddenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))

	var mockPubSub pubsub.PublisherSubscriber

	cfgs := []*Config{
		{URL: "dummy-url", Subject: "gofr-value", Version: "latest"},
		{URL: forbiddenServer.URL, Subject: "gofr-value"},
	}

	for i, cfg := range cfgs {
		_, err := NewWithConfig(cfg, mockPubSub)

		assert.NotNil(t, err, "Testcase [%d] failed.", i)
	}
}

func Test_PubSub__PublishEventWithOptionsError(t *testing.T) {
	var (
		options *pubsub.PublishOptions
		avro    Avro
		expErr  = &errors.Response{Code: "Missing schema", Reason: "Avro is initialized without schema"}
	)

	err := avro.PublishEventWithOptions("1", "value", map[string]string{}, options)

	assert.Equal(t, expErr, err, "Testcase failed. Expected: %v Got: %v", expErr, err)
}
