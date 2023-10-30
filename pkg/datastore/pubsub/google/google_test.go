//go:build !skip

package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/stretchr/testify/assert"

	gpubsub "cloud.google.com/go/pubsub"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

func Test_PubSub_New(t *testing.T) {
	t.Setenv("PUBSUB_BACKEND", "google")
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	incorrectTopicName := fmt.Sprintf("testTopic%v", time.Now())
	logger := log.NewMockLogger(io.Discard)

	testCases := []struct {
		desc   string
		config *Config
		expErr error
	}{
		{"Success case: correct credentials provided", &Config{ProjectID: "test123", TopicName: "test",
			SubscriptionDetails: &Subscription{Name: "subsTest"}, TimeoutDuration: 30}, nil},
		{"Failure case: incorrect topic provided", &Config{ProjectID: "test123", TopicName: incorrectTopicName,
			SubscriptionDetails: &Subscription{Name: "subsTest"}, TimeoutDuration: 30}, &apierror.APIError{}},
		{"Failure case: incorrect credentials provided", &Config{ProjectID: "invalid project", TopicName: "invalid topic", TimeoutDuration: 5},
			&apierror.APIError{}},
		{"Failure case: timeout is 0", &Config{ProjectID: "invalid project", TopicName: "invalid topic", TimeoutDuration: 0},
			&apierror.APIError{}},
	}

	for i, tc := range testCases {
		res, err := New(tc.config, logger)

		assert.IsTypef(t, &GCPubSub{}, res, "Test [%d] Failed: %v", i+1, tc.desc)
		assert.IsTypef(t, tc.expErr, err, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func Test_PubSub_createSubscription(t *testing.T) {
	g := initializeTest(t)

	g.config.Topic = g.client.Topic(g.config.TopicName)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.config.TimeoutDuration)*time.Second)

	defer func() {
		_ = g.client.Subscription("incorrectSubscription").Delete(ctx)

		cancel()
	}()

	cfg := config.NewGoDotEnvProvider(log.NewMockLogger(io.Discard), "../../../../configs")

	testCases := []struct {
		desc         string
		Subscription *Subscription
		expErr       error
	}{
		{"Success case: correct subscription name provided", &Subscription{Name: cfg.Get("GOOGLE_SUBSCRIPTION_NAME")}, nil},
		{"Success case: incorrect subscription name provided", &Subscription{Name: "incorrectSubscription"}, nil},
		{"Failure case: invalid subscription name provided", &Subscription{Name: "9999"}, errors.Error("")},
	}

	for i, tc := range testCases {
		g.config.SubscriptionDetails = tc.Subscription
		err := createSubscription(ctx, g)

		assert.IsTypef(t, tc.expErr, err, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func Test_PubSub_SubscribeWithCommit(t *testing.T) {
	t.Setenv("PUBSUB_BACKEND", "google")
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	conf := config.NewGoDotEnvProvider(log.NewLogger(), "../../../../configs")

	sampleData := struct {
		ID    string `avro:"Id"`
		Name  string `avro:"Name"`
		Email string `avro:"Email"`
	}{
		ID:    "1",
		Name:  "Rohan",
		Email: "rohan@email.xyz",
	}
	byteData, _ := json.Marshal(sampleData)

	configs := Config{
		ProjectID:           conf.Get("GOOGLE_PROJECT_ID"),
		TopicName:           conf.Get("GOOGLE_TOPIC_NAME"),
		TimeoutDuration:     30,
		SubscriptionDetails: &Subscription{Name: conf.Get("GOOGLE_SUBSCRIPTION_NAME")},
	}

	conn, err := New(&configs, log.NewMockLogger(new(bytes.Buffer)))
	if err != nil {
		t.Fatal(err)
	}

	_ = conn.PublishEvent("", sampleData, nil)

	res, err := conn.SubscribeWithCommit(nil)

	assert.Equal(t, res, &pubsub.Message{
		Value: string(byteData),
		Topic: conf.Get("GOOGLE_TOPIC_NAME")}, "Testcase Failed")

	assert.Equal(t, err != nil, false, "Testcase Failed")
}

func Test_PubSub_PublishEventWithOptions(t *testing.T) {
	g := initializeTest(t)

	g.config.Topic = g.client.Topic(g.config.TopicName)

	unsupportedType := func() {
		fmt.Println("Error case")
	}

	testCases := []struct {
		desc    string
		value   interface{}
		options *pubsub.PublishOptions
		expErr  error
	}{
		{"Success case: value and options are given", "test value",
			&pubsub.PublishOptions{Topic: g.config.TopicName, Timestamp: time.Now()}, nil},
		{"Error case: value type is unsupported", unsupportedType,
			&pubsub.PublishOptions{Topic: g.config.TopicName, Timestamp: time.Now()}, &json.UnsupportedTypeError{}},
		{"Success case: options is nil", "test value", nil, nil},
	}

	for i, tc := range testCases {
		err := g.PublishEventWithOptions("", tc.value, map[string]string{}, tc.options)

		assert.IsTypef(t, tc.expErr, err, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func Test_PubSub_PublishEvent(t *testing.T) {
	g := initializeTest(t)

	g.config.Topic = g.client.Topic(g.config.TopicName)

	err := g.PublishEvent("", "test value", map[string]string{})

	assert.Nilf(t, err, "Test Failed")
}

func Test_PubSub_Bind(t *testing.T) {
	t.Setenv("PUBSUB_BACKEND", "google")
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	g := GCPubSub{}

	message := []byte(`{"name":"Rohan","email":"rohan@email.xyz"}`)
	data := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{}

	err := g.Bind(message, &data)

	assert.Nilf(t, err, "Test Failed. Expected Nil Got %v", err)
}

func Test_PubSub_Ping(t *testing.T) {
	g := initializeTest(t)

	testCases := []struct {
		desc      string
		topicName string
		expErr    error
	}{
		{"Success case: connection exists", "test", nil},
		{"Success case: value and options are given", "incorrectTopic", errors.Error("")},
	}

	for i, tc := range testCases {
		g.config.TopicName = tc.topicName
		err := g.Ping()

		assert.IsTypef(t, tc.expErr, err, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func Test_PubSub_Ping_Error(t *testing.T) {
	g := initializeTest(t)
	err := g.client.Close()

	if err != nil {
		t.Fatal("Unable to close connection")
	}

	g.config.TopicName = "test"
	err = g.Ping()
	assert.IsTypef(t, &apierror.APIError{}, err, "Tescase Failed: %v")
}

func Test_PubSub_Ping_ClientNotSet(t *testing.T) {
	g := initializeTest(t)
	g.client = nil

	err := g.Ping()

	assert.Equalf(t, errors.Error("Google Pubsub not initialized"), err, "Test Failed: client is not initialized")
}

func Test_PubSub_HealthCheck(t *testing.T) {
	g := initializeTest(t)

	expHealth := types.Health{
		Name:   datastore.GooglePubSub,
		Status: pkg.StatusUp,
		Host:   g.config.TopicName,
	}

	health := g.HealthCheck()

	assert.Equalf(t, expHealth, health, "Test Failed: client is not initialized")
}

func Test_PubSub_HealthCheck_Failed(t *testing.T) {
	g := initializeTest(t)

	// updating topic name to non-existing topic name
	// so that Ping() returns error
	g.config.TopicName = "incorrectTopic"

	expHealth := types.Health{
		Name:   datastore.GooglePubSub,
		Status: pkg.StatusDown,
		Host:   g.config.TopicName,
	}

	health := g.HealthCheck()

	assert.Equalf(t, expHealth, health, "Test Failed: client is not initialized")
}

func Test_PubSub_HealthCheck_ClientNotSet(t *testing.T) {
	g := initializeTest(t)
	g.client = nil

	expHealth := types.Health{
		Name:   datastore.GooglePubSub,
		Status: pkg.StatusDown,
	}

	health := g.HealthCheck()

	assert.Equalf(t, expHealth, health, "Test Failed: client is not initialized")
}

func Test_PubSub_IsSet(t *testing.T) {
	t.Setenv("PUBSUB_BACKEND", "google")
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	testCases := []struct {
		desc   string
		client *gpubsub.Client
		expRes bool
	}{
		{"Success case: when client is not nil", &gpubsub.Client{}, true},
		{"Failure case: when client is nil", nil, false},
	}
	g := GCPubSub{}

	for i, tc := range testCases {
		g.client = tc.client
		res := g.IsSet()

		assert.Equal(t, tc.expRes, res, "Test [%d] Failed: %v", i+1, tc.desc)
	}
}

func initializeTest(t *testing.T) *GCPubSub {
	t.Setenv("PUBSUB_BACKEND", "google")
	t.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8086")

	logger := log.NewMockLogger(io.Discard)
	cfg := config.NewGoDotEnvProvider(logger, "../../../../configs")
	pubsubCfg := &Config{ProjectID: cfg.Get("GOOGLE_PROJECT_ID"), TopicName: cfg.Get("GOOGLE_TOPIC_NAME"), TimeoutDuration: 30}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(pubsubCfg.TimeoutDuration)*time.Second)

	defer cancel()

	client, err := gpubsub.NewClient(ctx, cfg.Get("GOOGLE_PROJECT_ID"))
	if err != nil {
		t.Fatalf("Unable to create client: %v", err)
	}

	return &GCPubSub{config: pubsubCfg, client: client, logger: logger}
}
