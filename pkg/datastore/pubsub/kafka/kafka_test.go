package kafka

import (
	"bytes"
	"crypto/sha512"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg"
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/datastore/pubsub"
	"gofr.dev/pkg/datastore/pubsub/avro"
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/types"
	"gofr.dev/pkg/log"
)

const connectionFailError = "invalid brokers connection failed"

func Test_PubSub_NewKafka(t *testing.T) {
	conf := sarama.NewConfig()
	conf.Consumer.Group.Session.Timeout = 1

	logger := log.NewMockLogger(io.Discard)

	testCases := []struct {
		k       Config
		wantErr bool
	}{
		{Config{Brokers: "localhost:2008,localhost:2009", Topics: []string{"test-topic"}, MaxRetry: 4,
			RetryFrequency: 300, DisableAutoCommit: true}, false},
		{Config{Brokers: "localhost:2008,localhost:2009", Topics: []string{"test-topic"}, MaxRetry: 4,
			RetryFrequency: 300, DisableAutoCommit: false}, false},
		{Config{Brokers: "localhost:2009", Topics: []string{"test-topic"}, Config: conf}, true},
		{Config{Brokers: "localhost:0000", Topics: []string{"test-topic"}}, true},
	}

	for i, tt := range testCases {
		_, err := New(&tt.k, logger)
		if !tt.wantErr && err != nil {
			t.Errorf("FAILED[%v], expected: %v, got: %v", i+1, tt.wantErr, err)
		}

		if tt.wantErr && err == nil {
			t.Errorf("FAILED[%v], expected: %v, got: %v", i+1, tt.wantErr, err)
		}
	}
}

func Test_PubSub_NewKafkaProducer(t *testing.T) {
	tests := []struct {
		config *Config
		err    error
	}{
		{&Config{Brokers: "somehost"}, sarama.ErrOutOfBrokers},
		{&Config{Topics: []string{"some-topic"}, Brokers: "localhost:2009"}, nil},
	}

	for _, test := range tests {
		_, err := NewKafkaProducer(test.config)
		if !errors.Is(err, test.err) {
			t.Errorf("FAILED, expected: %v, got: %v", test.err, err)
		}
	}
}

func Test_PubSub_NewKafkaConsumer(t *testing.T) {
	conf := sarama.NewConfig()
	conf.Consumer.Group.Session.Timeout = 1
	tests := []struct {
		config *Config
		err    error
	}{
		{&Config{Brokers: "localhost:2009", Config: conf}, sarama.ConfigurationError("Consumer.Group.Session.Timeout must be >= 2ms")},
		{&Config{Brokers: "localhost:2009", Topics: []string{"some-topic"}}, nil},
	}

	for _, test := range tests {
		_, err := NewKafkaConsumer(test.config)
		if !reflect.DeepEqual(test.err, err) {
			t.Errorf("FAILED, expected: %v, got: %v", test.err, err)
		}
	}
}

func Test_PubSub_NewKafkaFromEnv(t *testing.T) {
	logger := log.NewLogger()
	config.NewGoDotEnvProvider(logger, "../../../../configs")

	{
		// success case
		_, err := NewKafkaFromEnv()
		if err != nil {
			t.Errorf("FAILED, expected: %v, got: %v", nil, err)
		}
		{
			// error case due to invalid kafka host
			kafkaHosts := os.Getenv("KAFKA_HOSTS")

			t.Setenv("KAFKA_HOSTS", "localhost:9999")

			_, err := NewKafkaFromEnv()
			if err == nil {
				t.Errorf("Failed, expected: %v, got: %v ", brokersErr{}, nil)
			}

			t.Setenv("KAFKA_HOSTS", kafkaHosts)
		}
	}
}

func Test_PubSub_Success(t *testing.T) {
	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	k, err := New(&Config{
		Brokers:        c.Get("KAFKA_HOSTS"),
		Topics:         []string{c.Get("KAFKA_TOPIC")},
		InitialOffsets: OffsetOldest,
		GroupID:        "testing-consumerGroup",
	}, logger)
	if err != nil {
		t.Errorf("Kafka connection failed : %v", err)
		return
	}

	Ping(t, k)
	PublishEvent(t, k)
	SubscribeWithCommit(t, k, c.Get("KAFKA_TOPIC"))
	Subscribe(t, k)
	Pause(t, k)
	Resume(t, k)
	PublishEventOptions(t, k)
}

// Test_PubSubWithOffset check the subscribe operation with custom initial offset value.
func Test_PubSub_WithOffset(t *testing.T) {
	t.Skip("skipping testing in short mode")

	logger := log.NewLogger()
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := c.Get("KAFKA_TOPIC")
	// prereqisite
	k, err := New(&Config{
		Brokers:        c.Get("KAFKA_HOSTS"),
		Topics:         []string{topic},
		InitialOffsets: OffsetOldest,
		GroupID:        "testing-consumerGroup",
		Offsets:        []pubsub.TopicPartition{{Topic: topic, Partition: 0, Offset: 1}},
	}, logger)

	if err != nil {
		t.Errorf("Kafka connection failed : %v", err)
		return
	}

	Ping(t, k)
	// In this we are first trying to publish some messages then we are consuming 1 message
	PublishMessage(t, k)
	// SubscribeWithCommit will only subscribe and commit messages from 0th and the 1st offset.
	SubscribeWithCommit(t, k, topic)
	// Subscribe to ensure every message in the queue is subscribed.
	Subscribe(t, k)
}

func PublishEvent(t *testing.T, k *Kafka) {
	type args struct {
		key   string
		value interface{}
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"success writing message", args{"testKey", "testValue"}, false},
		{"success writing message", args{"testKey1", "testValue1"}, false},
		{"success writing message", args{"testKey2", "testValue2"}, false},
		{"success writing message", args{"testKey3", "testValue3"}, false},
		{"json error in message", args{"testKey", make(chan int)}, true},
	}

	for _, tt := range tests {
		tt := tt
		if err := k.PublishEvent(tt.args.key, tt.args.value, map[string]string{
			"header": "value",
		}); (err != nil) != tt.wantErr {
			t.Errorf("PublishEvent() error = %v, wantErr %v", err, tt.wantErr)
		}
	}
}

func PublishEventOptions(t *testing.T, k *Kafka) {
	options := &pubsub.PublishOptions{
		Topic:     "testTopic",
		Timestamp: time.Now(),
	}

	testcases := []struct {
		desc    string
		options *pubsub.PublishOptions
	}{
		{"Publish With Options", options},
		{"Publish Without Options", nil},
	}

	for i, tc := range testcases {
		err := k.PublishEventWithOptions("testKey", "testValue", map[string]string{
			"header": "value",
		}, tc.options)
		assert.NoErrorf(t, err, "Testcase[%d] failed", i)
	}
}

func SubscribeWithCommit(t *testing.T, k *Kafka, topic string) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)

	count := 0
	commitFunc := func(msg *pubsub.Message) (bool, bool) {
		logger.Infof("Message received: %v, Topic: %v", msg.Value, msg.Topic)

		if count < 1 {
			count++

			return true, true
		}

		return false, false
	}

	_, _ = k.SubscribeWithCommit(commitFunc)

	expectedMsg1 := fmt.Sprintf("Message received: \\\"testValue\\\", Topic: %v", topic)
	expectedMsg2 := fmt.Sprintf("Message received: \\\"testValue1\\\", Topic: %v", topic)

	if !strings.Contains(b.String(), expectedMsg1) {
		t.Errorf("FAILED expected: %v, got: %v", expectedMsg1, b.String())
	}

	if !strings.Contains(b.String(), expectedMsg2) {
		t.Errorf("FAILED expected: %v, got: %v", expectedMsg2, b.String())
	}

	msg, err := k.SubscribeWithCommit(nil)
	if err != nil {
		t.Errorf("FAILED, expected no error when commitFunc is not provided, got: %v", err)
	}

	k.CommitOffset(pubsub.TopicPartition{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
	})
}

func Subscribe(t *testing.T, k *Kafka) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"success reading message", false},
	}

	for _, tt := range tests {
		msg, err := k.Subscribe()
		if (err != nil) != tt.wantErr {
			t.Errorf("Subscribe() error = %v, wantErr %v", err, tt.wantErr)
			return
		}

		if msg == nil {
			t.Errorf("Subscribe(): expected message, got nil")
			return
		}

		if len(msg.Headers) != 1 {
			t.Errorf("Subscribe() only one message header should be present, found: %v", len(msg.Headers))
		}
	}
}

func Ping(t *testing.T, k *Kafka) {
	err := k.Ping()
	if err != nil && err.Error() == connectionFailError {
		t.Errorf("FAILED, expected: successful ping, got: %v", err)
	}

	k.config.Brokers = "localhost"

	err = k.Ping()
	if err == nil {
		t.Error("FAILED, expected: unsuccessful ping, got: nil")
	}
}

func Pause(t *testing.T, k *Kafka) {
	err := k.Pause()
	if err != nil {
		t.Errorf("FAILED, expected: successful Pause, got: %v", err)
	}
}

func Resume(t *testing.T, k *Kafka) {
	err := k.Resume()
	if err != nil {
		t.Errorf("FAILED, expected: successful Resume, got: %v", err)
	}
}

func Test_PubSub_convertKafkaConfig(t *testing.T) {
	expectedConfig := sarama.NewConfig()
	setDefaultConfig(expectedConfig)

	expectedConfig.Version = sarama.MaxVersion
	expectedConfig.Consumer.Group.Member.UserData = []byte("1")
	expectedConfig.Consumer.Offsets.Initial = OffsetOldest

	saramaCfg := expectedConfig
	saramaCfg.ClientID = ""

	kafkaConfig := &Config{GroupID: "1", MaxRetry: 3, InitialOffsets: OffsetOldest, RetryFrequency: 10,
		DisableAutoCommit: true, Config: saramaCfg}

	convertKafkaConfig(kafkaConfig)

	kafkaConfig.Config.Producer.Partitioner = nil
	expectedConfig.Producer.Partitioner = nil

	assert.Equal(t, expectedConfig, kafkaConfig.Config)
}

func Test_PubSub_processSASLConfigs(t *testing.T) {
	expConfig := sarama.NewConfig()

	expConfig.Net.SASL.User = "testuser"
	expConfig.Net.SASL.Password = "password"
	expConfig.Net.SASL.Handshake = true
	expConfig.Net.SASL.Enable = true
	expConfig.Net.TLS.Enable = true
	expConfig.Net.TLS.Config = &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // TLS InsecureSkipVerify set true.
	}

	scramClientGeneratorFunc := func() sarama.SCRAMClient {
		return &XDGSCRAMClient{HashGeneratorFcn: sha512.New}
	}

	tests := []struct {
		desc                     string
		mechanism                string
		SCRAMClientGeneratorFunc func() sarama.SCRAMClient
	}{
		{"using mechanism SASL/PLAIN", PLAIN, nil},
		{"using SASL mechanism SCRAM", SASLTypeSCRAMSHA512, scramClientGeneratorFunc},
	}

	for i, tc := range tests {
		expConfig.Net.SASL.Mechanism = sarama.SASLMechanism(tc.mechanism)
		expConfig.Net.SASL.SCRAMClientGeneratorFunc = tc.SCRAMClientGeneratorFunc

		saslConfig := SASLConfig{
			User:      "testuser",
			Password:  "password",
			Mechanism: tc.mechanism,
		}

		conf := sarama.NewConfig()
		processSASLConfigs(saslConfig, conf)

		conf.Net.SASL.SCRAMClientGeneratorFunc = nil
		expConfig.Net.SASL.SCRAMClientGeneratorFunc = nil
		conf.Producer.Partitioner = nil
		expConfig.Producer.Partitioner = nil

		assert.Equal(t, expConfig, conf, "TEST[%d], failed.\n%s", i, tc.desc)
	}
}

func Test_PubSub_KafkaAuthentication(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := c.Get("KAFKA_TOPIC")
	topics := strings.Split(topic, ",")

	tests := []struct {
		userName      string
		pass          string
		authMechanism string
		err           error
	}{
		{"", "", PLAIN, nil},
		{"gofr.dev", "gofr.dev", "", errInvalidMechanism},
	}

	for i, tc := range tests {
		cfg := &Config{

			Brokers: c.Get("KAFKA_HOSTS"),
			Topics:  topics,
			SASL:    SASLConfig{User: tc.userName, Password: tc.pass, Mechanism: tc.authMechanism},
		}

		_, err := New(cfg, log.NewMockLogger(io.Discard))

		assert.Equal(t, tc.err, err, "TEST[%d], failed.\n%s", i)
	}
}

func Test_PubSub_invalidSaslMechanism(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := c.Get("KAFKA_TOPIC")
	topics := strings.Split(topic, ",")

	cfg := &Config{
		Brokers: c.Get("KAFKA_HOSTS"),
		Topics:  topics,
		SASL: SASLConfig{
			User:      "invalid-user-name",
			Password:  "password",
			Mechanism: "invalid-mechanism",
		},
	}

	kafka, err := New(cfg, logger)

	assert.Equal(t, errInvalidMechanism, err)

	assert.Nil(t, kafka)
}

func Test_PubSub_KafkaHealthCheck(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := c.Get("KAFKA_TOPIC")
	topics := strings.Split(topic, ",")
	testCases := []struct {
		config     Config
		expected   types.Health
		logMessage string
	}{
		{Config{Brokers: c.Get("KAFKA_HOSTS"), Topics: topics}, types.Health{Name: datastore.Kafka,
			Status: pkg.StatusUp, Host: c.Get("KAFKA_HOSTS"), Database: topic}, ""},
		{Config{Brokers: "random", Topics: topics}, types.Health{Name: datastore.Kafka, Status: pkg.StatusDown,
			Host: "random", Database: topic}, "Health check failed"},
	}

	for i, tc := range testCases {
		conn, _ := New(&tc.config, logger)
		output := conn.HealthCheck()

		if !reflect.DeepEqual(tc.expected, output) {
			t.Errorf("[TESTCASE%v]Failed. Got%v Expected%v", i+1, output, tc.expected)
		}

		if !strings.Contains(b.String(), tc.logMessage) {
			t.Errorf("Test Failed \nExpected: %v\nGot: %v", tc.logMessage, b.String())
		}
	}
}

func Test_PubSub_Kafka_HealthCheckDown(t *testing.T) {
	b := new(bytes.Buffer)
	logger := log.NewMockLogger(b)
	expLog := "Health check failed"

	c := &Config{
		Brokers: "localhost:2003",
		Topics:  []string{"unknown-topic"},
	}
	conn1, _ := New(c, logger)

	var conn2 *Kafka

	conn3 := new(Kafka)
	conn3.config = c
	conn3.logger = logger

	testcases := []struct {
		desc             string
		conn             *Kafka
		expectedResponse types.Health
	}{
		{"connection with config", conn1, types.Health{Name: datastore.Kafka, Status: pkg.StatusDown, Host: c.Brokers, Database: c.Topics[0]}},
		{"connection without config", conn2, types.Health{Name: datastore.Kafka, Status: pkg.StatusDown}},
		{"connection using new", conn3, types.Health{Name: datastore.Kafka, Status: pkg.StatusDown, Host: c.Brokers, Database: c.Topics[0]}},
	}
	for i, tc := range testcases {
		healthCheck := tc.conn.HealthCheck()

		assert.Equal(t, tc.expectedResponse, healthCheck, "TEST[%d], failed.\n%s", i, tc.desc)

		if !strings.Contains(b.String(), expLog) {
			t.Errorf("Test Failed \nExpected: %v\nGot: %v", expLog, b.String())
		}
	}
}

func Test_PubSub_IsSet(t *testing.T) {
	var k *Kafka

	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := c.Get("KAFKA_TOPIC")
	conn, _ := New(&Config{Brokers: c.Get("KAFKA_HOSTS"), Topics: strings.Split(topic, ",")}, logger)

	testcases := []struct {
		k    *Kafka
		resp bool
	}{
		{k, false},
		{&Kafka{}, false},
		{&Kafka{Producer: conn.Producer}, false},
		{&Kafka{Consumer: conn.Consumer}, false},
		{conn, true},
	}

	for i, v := range testcases {
		resp := v.k.IsSet()
		if resp != v.resp {
			t.Errorf("[TESTCASE%d]Failed.Expected %v\tGot %v\n", i+1, v.resp, resp)
		}
	}
}

func Test_PubSub_SubscribeError(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := "dummy-topic"
	conn, _ := New(&Config{Brokers: c.Get("KAFKA_HOSTS"), Topics: strings.Split(topic, ",")}, logger)

	_ = conn.Consumer.ConsumerGroup.Close()

	if _, err := conn.Subscribe(); err == nil {
		t.Errorf("FAILED, expected error from subcribe got nil")
	}
}

func Test_PubSub_SubscribeWithCommitError(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")
	topic := "dummy-topic"
	conn, _ := New(&Config{Brokers: c.Get("KAFKA_HOSTS"), Topics: strings.Split(topic, ",")}, logger)

	_ = conn.Consumer.ConsumerGroup.Close()

	if _, err := conn.SubscribeWithCommit(nil); err == nil {
		t.Errorf("FAILED, expected error from subcribe got nil")
	}
}

func PublishMessage(t *testing.T, k *Kafka) {
	tests := []struct {
		key   string
		value interface{}
	}{
		{"testKey", "testValue"},
		{"testKey1", "testValue1"},
		{"testKey2", "testValue2"},
		{"testKey3", "testValue3"},
	}

	for i, tc := range tests {
		if err := k.PublishEvent(tc.key, tc.value, map[string]string{
			"header": "value",
		}); err != nil {
			t.Errorf("Failed[%v] expected error as nil\n got %v", i, err)
		}
	}
}

func Test_PubSub_populateOffsetTopic(t *testing.T) {
	tests := []struct {
		config         *Config
		expectedConfig *Config
	}{
		{&Config{Topics: []string{"test-topic"}}, &Config{Topics: []string{"test-topic"}}},
		{&Config{Offsets: []pubsub.TopicPartition{}}, &Config{Offsets: []pubsub.TopicPartition{}}},
		{&Config{Offsets: []pubsub.TopicPartition{{Offset: 1}}}, &Config{Offsets: []pubsub.TopicPartition{{Offset: 1}}}},
		{&Config{Topics: []string{"test-topic"}, Offsets: []pubsub.TopicPartition{{Offset: 1, Topic: "test-custom-topic"}, {Offset: 2}}},
			&Config{Topics: []string{"test-topic"}, Offsets: []pubsub.TopicPartition{{Offset: 1, Topic: "test-custom-topic"},
				{Offset: 2, Topic: "test-topic"}}}},
	}

	for i, tc := range tests {
		populateOffsetTopic(tc.config)
		assert.Equal(t, tc.expectedConfig, tc.config, i)
	}
}

func Test_PubSub_NewKafkaWithAvro(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respMap := map[string]interface{}{"subject": "gofr-value", "version": 2, "id": 293,
			"schema": `{"type":"record","name":"test","fields":[{"name":"ID","type":"string"}]}`}
		_ = json.NewEncoder(w).Encode(respMap)
	}))

	testCases := []struct {
		config  AvroWithKafkaConfig
		wantErr bool
	}{
		// Success cases, kafkaConfig and avroConfig both are right
		{AvroWithKafkaConfig{KafkaConfig: Config{Brokers: "localhost:2008,localhost:2009", Topics: []string{"test-topic"}},
			AvroConfig: avro.Config{URL: server.URL, Version: "", Subject: "gofr-value"}}, false},
		// failure due wrong kafkaConfig, so it wil not check the avroConfig
		{AvroWithKafkaConfig{KafkaConfig: Config{Brokers: "localhost:0000", Topics: []string{"test-topic"}},
			AvroConfig: avro.Config{URL: server.URL, Version: "", Subject: "gofr-value"}}, true},
		// failure due to wrong avroConfig
		{AvroWithKafkaConfig{KafkaConfig: Config{Brokers: "localhost:2008", Topics: []string{"test-topic"}},
			AvroConfig: avro.Config{URL: "dummy-url.com", Subject: "gofr-value"}}, true},
	}

	for i, tc := range testCases {
		_, err := NewKafkaWithAvro(&tc.config, logger)
		if !tc.wantErr && err != nil {
			t.Errorf("FAILED[%v], expected: %v, got: %v", i+1, tc.wantErr, true)
		}

		if tc.wantErr && err == nil {
			t.Errorf("FAILED[%v], expected: %v, got: %v", i+1, tc.wantErr, false)
		}
	}
}

func Test_PubSub_Printf(t *testing.T) {
	tests := []struct {
		format    string
		input     []interface{}
		expOutput string
	}{
		{"%s %s %s", []interface{}{"log", struct{ Name string }{"data"}, map[string]interface{}{"key": "value"}},
			"log {data} map[key:value]"},
		{"print data %v %s", []interface{}{123, map[string]interface{}{"key": "value"}},
			"print data 123 map[key:value]"},
	}

	for i, tc := range tests {
		b := new(bytes.Buffer)
		kl := kafkaLogger{logger: log.NewMockLogger(b)}

		kl.Printf(tc.format, tc.input...)

		if !strings.Contains(b.String(), tc.expOutput) {
			t.Errorf("failed[%v] expected %v\n got %v", i, tc.expOutput, b.String())
		}
	}
}

func Test_PubSub_Print(t *testing.T) {
	input := []interface{}{"Print the sys log,", "Print kafka Log", map[string]interface{}{"key": "value"}}
	expOutput := "Print the sys log, Print kafka Log"

	b := new(bytes.Buffer)
	kl := kafkaLogger{logger: log.NewMockLogger(b)}

	kl.Print(input...)

	if !strings.Contains(b.String(), expOutput) {
		t.Errorf("failed expected %v\n got %v", expOutput, b.String())
	}
}

func Test_PubSub_Println(t *testing.T) {
	input := []interface{}{"Print the sys log,", "Print kafka Log", map[string]interface{}{"key": "value"}}
	expOutput := "Print the sys log, Print kafka Log"

	b := new(bytes.Buffer)
	kl := kafkaLogger{logger: log.NewMockLogger(b)}

	kl.Println(input...)

	if !strings.Contains(b.String(), expOutput) {
		t.Errorf("failed expected %v\n got %v", expOutput, b.String())
	}
}

func Test_PubSub_Kafka_SubscribeNilMessage(t *testing.T) {
	logger := log.NewMockLogger(io.Discard)
	c := config.NewGoDotEnvProvider(logger, "../../../../configs")

	topic := "test-topic"
	conn, _ := New(&Config{Brokers: c.Get("KAFKA_HOSTS"), Topics: strings.Split(topic, ",")}, logger)

	// close the channel to get the msg as nil
	close(conn.Consumer.ConsumerGroupHandler.msg)

	msg, err := conn.subscribeMessage()
	if msg != nil {
		t.Errorf("Failed: Expected Message: %v, Got: %v", nil, msg)
	}

	assert.Equal(t, errConsumeMsg, err)
}

func Test_PubSub_Error(t *testing.T) {
	brokerErr := brokersErr{}
	expectedError := connectionFailError

	err := brokerErr.Error()

	assert.Equal(t, expectedError, err, "Test [%d] Failed. Expected: %s, Got: %s", expectedError, err)
}

func Test_PubSub_Kafka_Bind(t *testing.T) {
	k := Kafka{}

	message := []byte(`{"name":"Rohan","email":"rohan@email.xyz"}`)
	data := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{}

	err := k.Bind(message, &data)

	assert.Nil(t, err, "TEST [%d] Failed. Expected Nil Got %v", err)
}

func Test_PubSub_SetDefaultConfig_Metadata(t *testing.T) {
	mockConfig := &sarama.Config{}

	setDefaultConfig(mockConfig)

	assert.Equal(t, mockConfig.Metadata.Retry.Backoff, time.Duration(100000000),
		"Expected Metadata.Retry.Backoff to be 100000000, got %d", mockConfig.Metadata.Retry.Backoff)

	assert.Equal(t, mockConfig.Metadata.Retry.Max, 5,
		"Expected Metadata.Retry.Max to be 5, got %d", mockConfig.Metadata.Retry.Max)
}
func Test_PubSub_SetDefaultConfig_Producer(t *testing.T) {
	mockConfig := &sarama.Config{}

	setDefaultConfig(mockConfig)

	assert.Equal(t, mockConfig.Producer.Retry.Backoff, time.Duration(5000000),
		"Expected Producer.Retry.Backoff to be 5000000, got %d", mockConfig.Producer.Retry.Backoff)

	assert.Equal(t, mockConfig.Producer.Timeout, time.Duration(300000000),
		"Expected Producer.Timeout to be 300000000, got %d", mockConfig.Producer.Timeout)

	assert.Equal(t, mockConfig.Producer.RequiredAcks, sarama.RequiredAcks(1),
		"Expected Producer.RequiredAcks to be 1, got %d", mockConfig.Producer.RequiredAcks)

	assert.Equal(t, mockConfig.Producer.Return.Successes, true,
		"Expected Producer.Return.Successes to be true", mockConfig.Producer.Return.Successes)

	assert.Equal(t, mockConfig.Producer.Return.Errors, true,
		"Expected Producer.Return.Errors to be true", mockConfig.Producer.Return.Errors)
}
func Test_PubSub_SetDefaultConfig_Consumer(t *testing.T) {
	mockConfig := &sarama.Config{}

	setDefaultConfig(mockConfig)

	assert.Equal(t, mockConfig.Consumer.Retry.Backoff, time.Duration(5000000),
		"Expected Consumer.Retry.Backoff to be 5000000, got %d", mockConfig.Consumer.Retry.Backoff)

	assert.Equal(t, mockConfig.Consumer.MaxWaitTime, time.Duration(300000000),
		"Expected Consumer.MaxWaitTime to be 300000000, got %d", mockConfig.Consumer.MaxWaitTime)

	assert.Equal(t, mockConfig.Consumer.Group.Heartbeat.Interval, time.Duration(1000000),
		"Expected Consumer.Group.Heartbeat.Interval to be 1000000, got %d", mockConfig.Consumer.Group.Heartbeat.Interval)
}

func Test_PubSub_ConsumerHandler_Cleanup(t *testing.T) {
	consumer := &ConsumerHandler{}

	session := &MockConsumerGroupSession{}

	err := consumer.Cleanup(session)

	assert.Nil(t, err, "Cleanup should not return an error")
}

type MockConsumerGroupSession struct {
	sarama.ConsumerGroupSession
}

func (m *MockConsumerGroupSession) MarkOffset(string, int32, int64, string) {
}
func (m *MockConsumerGroupSession) MarkMessage(*sarama.ConsumerMessage, string) {}
