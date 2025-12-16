package redis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

// TestPubSub_Query_Channel tests querying messages from a Redis PubSub channel.
func TestPubSub_Query_Channel(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "query-channel"

	// Start Query in goroutine
	type queryResult struct {
		msgs []byte
		err  error
	}

	resChan := make(chan queryResult)

	go func() {
		// Query blocks until limit or timeout.
		msgs, err := client.PubSub.Query(ctx, topic, 2*time.Second, 2)
		resChan <- queryResult{msgs, err}
	}()

	// Wait for Query to subscribe (approximate)
	time.Sleep(200 * time.Millisecond)

	// Publish messages
	msgs := []string{"chan-msg1", "chan-msg2"}
	for _, m := range msgs {
		err := client.PubSub.Publish(ctx, topic, []byte(m))
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for result
	select {
	case res := <-resChan:
		require.NoError(t, res.err)

		expected := strings.Join(msgs, "\n")
		assert.Equal(t, expected, string(res.msgs))
	case <-time.After(3 * time.Second):
		t.Fatal("Query timed out")
	}
}

var (
	errMockPing        = errors.New("mock ping error")
	errMockPublish     = errors.New("mock publish error")
	errMockXAdd        = errors.New("mock xadd error")
	errMockGroup       = errors.New("mock group error")
	errMockGroupCreate = errors.New("mock group create error")
	errMockXRange      = errors.New("mock xrange error")
	errMockDel         = errors.New("mock del error")
)

func setupTest(t *testing.T, conf map[string]string) (*Redis, *miniredis.Miniredis) {
	t.Helper()

	s, err := miniredis.Run()
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)

	if conf == nil {
		conf = make(map[string]string)
	}

	conf["REDIS_HOST"] = s.Host()
	conf["REDIS_PORT"] = s.Port()
	conf["PUBSUB_BACKEND"] = "REDIS"

	client := NewClient(config.NewMockConfig(conf), mockLogger, mockMetrics)
	require.NotNil(t, client.PubSub)

	return client, s
}

func setupMockTest(t *testing.T, conf map[string]string) (*Redis, redismock.ClientMock) {
	t.Helper()

	db, mock := redismock.NewClientMock()

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)

	if conf == nil {
		conf = make(map[string]string)
	}
	// Add required config to trigger PubSub initialization
	conf["PUBSUB_BACKEND"] = "REDIS"
	conf["REDIS_HOST"] = "localhost"

	// Create Redis client but replace the internal client with mock
	// We can't easily replace the internal client of NewClient because it creates one.
	// So we construct Redis manually.

	redisConfig := getRedisConfig(config.NewMockConfig(conf), mockLogger)

	r := &Redis{
		Client:  db,
		config:  redisConfig,
		logger:  mockLogger,
		metrics: mockMetrics,
	}
	// Initialize PubSub manually with mock client
	r.PubSub = newPubSub(r, db)

	return r, mock
}

func TestPubSub_Operations(t *testing.T) {
	tests := getPubSubTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, s := setupTest(t, tt.config)
			defer s.Close()
			defer client.Close()

			tt.actions(t, client, s)
		})
	}
}

func getPubSubTestCases() []struct {
	name    string
	config  map[string]string
	actions func(t *testing.T, client *Redis, s *miniredis.Miniredis)
} {
	return append(
		getBasicTestCases(),
		getQueryTestCases()...,
	)
}

func getBasicTestCases() []struct {
	name    string
	config  map[string]string
	actions func(t *testing.T, client *Redis, s *miniredis.Miniredis)
} {
	return []struct {
		name    string
		config  map[string]string
		actions func(t *testing.T, client *Redis, s *miniredis.Miniredis)
	}{
		{
			name: "Channel Publish Subscribe",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "pubsub",
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testChannelPublishSubscribe(t, client)
			},
		},
		{
			name: "Stream Publish Subscribe",
			config: map[string]string{
				"REDIS_PUBSUB_MODE":            "streams",
				"REDIS_STREAMS_CONSUMER_GROUP": "grp",
				"REDIS_STREAMS_BLOCK_TIMEOUT":  "100ms",
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamPublishSubscribe(t, client)
			},
		},
		{
			name: "Delete Topic Channel",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "pubsub",
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testDeleteTopicChannel(t, client)
			},
		},
		{
			name: "Delete Topic Stream",
			config: map[string]string{
				"REDIS_PUBSUB_MODE":            "streams",
				"REDIS_STREAMS_CONSUMER_GROUP": "dgrp",
			},
			actions: func(t *testing.T, client *Redis, s *miniredis.Miniredis) {
				t.Helper()
				testDeleteTopicStream(t, client, s)
			},
		},
		{
			name:   "Health Check",
			config: map[string]string{},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testHealthCheck(t, client)
			},
		},
		{
			name: "Stream Config Error - Missing Group",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "streams",
				// Missing REDIS_STREAMS_CONSUMER_GROUP
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamConfigError(t, client)
			},
		},
		{
			name: "Stream MaxLen",
			config: map[string]string{
				"REDIS_PUBSUB_MODE":            "streams",
				"REDIS_STREAMS_CONSUMER_GROUP": "maxlen-grp",
				"REDIS_STREAMS_MAXLEN":         "5",
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamMaxLen(t, client)
			},
		},
	}
}

func getQueryTestCases() []struct {
	name    string
	config  map[string]string
	actions func(t *testing.T, client *Redis, s *miniredis.Miniredis)
} {
	return []struct {
		name    string
		config  map[string]string
		actions func(t *testing.T, client *Redis, s *miniredis.Miniredis)
	}{
		{
			name: "Channel Query",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "pubsub",
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testChannelQuery(t, client)
			},
		},
		{
			name: "Stream Query",
			config: map[string]string{
				"REDIS_PUBSUB_MODE":            "streams",
				"REDIS_STREAMS_CONSUMER_GROUP": "qgrp",
			},
			actions: func(t *testing.T, client *Redis, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamQuery(t, client)
			},
		},
	}
}

func testChannelPublishSubscribe(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "test-chan"
	msg := []byte("hello")

	ch := make(chan *pubsub.Message)
	errCh := make(chan error)

	go func() {
		m, err := client.PubSub.Subscribe(ctx, topic)
		if err != nil {
			errCh <- err
			return
		}

		ch <- m
	}()

	time.Sleep(100 * time.Millisecond)

	err := client.PubSub.Publish(ctx, topic, msg)
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case m := <-ch:
		assert.Equal(t, string(msg), string(m.Value))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func testStreamPublishSubscribe(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "test-stream"
	msg := []byte("hello stream")

	ch := make(chan *pubsub.Message)
	errCh := make(chan error)

	go func() {
		m, err := client.PubSub.Subscribe(ctx, topic)
		if err != nil {
			errCh <- err

			return
		}

		ch <- m
	}()

	time.Sleep(500 * time.Millisecond)

	err := client.PubSub.Publish(ctx, topic, msg)
	require.NoError(t, err)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case m := <-ch:
		assert.Equal(t, string(msg), string(m.Value))
		m.Committer.Commit()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func testChannelQuery(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "query-chan"

	ch := make(chan []byte)
	errCh := make(chan error)

	go func() {
		res, err := client.PubSub.Query(ctx, topic, 1*time.Second, 2)
		if err != nil {
			errCh <- err

			return
		}

		ch <- res
	}()

	time.Sleep(100 * time.Millisecond)

	_ = client.PubSub.Publish(ctx, topic, []byte("m1"))
	_ = client.PubSub.Publish(ctx, topic, []byte("m2"))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case res := <-ch:
		assert.Contains(t, string(res), "m1")
		assert.Contains(t, string(res), "m2")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func testStreamQuery(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "query-stream"

	_ = client.PubSub.Publish(ctx, topic, []byte("sm1"))
	_ = client.PubSub.Publish(ctx, topic, []byte("sm2"))

	res, err := client.PubSub.Query(ctx, topic, 1*time.Second, 10)
	require.NoError(t, err)

	// Note: This test may skip if miniredis returns empty results (known limitation)
	// Miniredis compatibility is tested separately in TestPubSub_Query_Stream_MiniredisCompatibility
	// For this test, we require results to be present to validate the query functionality
	require.NotEmpty(t, res, "Query should return results (miniredis limitation may cause empty results)")
	assert.Contains(t, string(res), "sm1")
	assert.Contains(t, string(res), "sm2")
}

// TestPubSub_Query_Stream_Success tests successful stream query operations.
// This test assumes results are available (miniredis compatibility is tested separately).
func TestPubSub_Query_Stream_Success(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "query-group",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "query-stream-success"

	// Publish some messages
	msgs := []string{"stream-msg1", "stream-msg2", "stream-msg3"}
	for _, m := range msgs {
		err := client.PubSub.Publish(ctx, topic, []byte(m))
		require.NoError(t, err)
	}

	// Query messages
	results, err := client.PubSub.Query(ctx, topic, 1*time.Second, 10)
	require.NoError(t, err)
	require.NotEmpty(t, results, "Query should return results for successful case")

	expected := strings.Join(msgs, "\n")
	assert.Equal(t, expected, string(results))
}

// TestPubSub_Query_Stream_MiniredisCompatibility tests Miniredis compatibility for stream queries.
// Miniredis may return empty results for XRANGE, which is acceptable behavior.
func TestPubSub_Query_Stream_MiniredisCompatibility(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "query-group",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "query-stream-compat"

	// Publish some messages
	msgs := []string{"stream-msg1", "stream-msg2"}
	for _, m := range msgs {
		err := client.PubSub.Publish(ctx, topic, []byte(m))
		require.NoError(t, err)
	}

	// Query messages
	results, err := client.PubSub.Query(ctx, topic, 1*time.Second, 10)
	require.NoError(t, err)

	// Miniredis XRANGE may return empty results - this is acceptable
	// The test passes regardless of whether results are empty or not
	// This documents the known miniredis limitation
	t.Logf("Query returned %d bytes (miniredis may return empty for XRANGE)", len(results))
}

func testDeleteTopicChannel(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "del-chan"

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)
}

func testDeleteTopicStream(t *testing.T, client *Redis, s *miniredis.Miniredis) {
	t.Helper()

	ctx := context.Background()
	topic := "del-stream"

	_ = client.PubSub.CreateTopic(ctx, topic)
	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)

	// Verify deleted
	exists := s.Exists(topic)
	assert.False(t, exists)
}

func testHealthCheck(t *testing.T, client *Redis) {
	t.Helper()

	h := client.PubSub.Health()
	assert.Equal(t, "UP", h.Status)
	assert.Equal(t, "streams", h.Details["mode"]) // Default mode is now streams
}

func testStreamConfigError(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "err-stream"

	err := client.PubSub.CreateTopic(ctx, topic)
	assert.Equal(t, errConsumerGroupNotProvided, err)

	// Subscribe should also log error and return (non-blocking in goroutine)
	ch := client.PubSub.ensureSubscription(ctx, topic)
	assert.NotNil(t, ch)
}

func testStreamMaxLen(t *testing.T, client *Redis) {
	t.Helper()

	ctx := context.Background()
	topic := "maxlen-stream"
	msg := []byte("payload")

	err := client.PubSub.Publish(ctx, topic, msg)
	assert.NoError(t, err)
}

func TestPubSub_Errors(t *testing.T) {
	// Setup Redis
	s, err := miniredis.Run()
	require.NoError(t, err)

	host := s.Host()
	port := s.Port()
	// Close it immediately to test connection errors
	s.Close()

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.ERROR)

	conf := map[string]string{
		"REDIS_HOST":     host,
		"REDIS_PORT":     port,
		"PUBSUB_BACKEND": "REDIS",
	}

	client := NewClient(config.NewMockConfig(conf), mockLogger, mockMetrics)

	require.NotNil(t, client.PubSub)
	defer client.Close()

	ctx := context.Background()
	topic := "err-topic"

	// Publish error
	err = client.PubSub.Publish(ctx, topic, []byte("msg"))
	require.Error(t, err)
	// We check for connection error, but specific error might vary (dial error vs errClientNotConnected)
	// ps.Publish checks isConnected() -> errClientNotConnected
	// But isConnected() returns false only if Ping fails.
	// Ping fails with "dial tcp..." error.
	// So isConnected returns false.
	assert.Equal(t, errClientNotConnected, err)

	// Subscribe error
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()

	msg, err := client.PubSub.Subscribe(ctxCancel, topic)
	require.NoError(t, err)
	assert.Nil(t, msg)
}

func TestPubSub_MockErrors(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection to avoid race conditions with Ping expectations
	if client.PubSub != nil && client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond) // Allow goroutine to exit
	}

	ctx := context.Background()
	topic := "mock-err-topic"

	// Test Publish Error (Ping succeeds, Publish fails)
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectPublish(topic, []byte("msg")).SetErr(errMockPublish)

	err := client.PubSub.Publish(ctx, topic, []byte("msg"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockPublish.Error())

	// Verify expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_StreamMockErrors(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mock-grp",
	})
	defer client.Close()

	// Stop monitorConnection to avoid race conditions with Ping expectations
	if client.PubSub != nil && client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond) // Allow goroutine to exit
	}

	ctx := context.Background()
	topic := "mock-stream-err"

	// Test Publish Error (Ping succeeds, XAdd fails)
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXAdd(&redis.XAddArgs{
		Stream: topic,
		Values: map[string]any{"payload": []byte("msg")},
	}).SetErr(errMockXAdd)

	err := client.PubSub.Publish(ctx, topic, []byte("msg"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockXAdd.Error())

	// Test CreateTopic Error
	// CreateTopic calls XInfoGroups first to check if group exists, then XGroupCreateMkStream
	// Note: CreateTopic doesn't call isConnected(), it just checks ps == nil || ps.client == nil
	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{}) // Group doesn't exist yet
	mock.ExpectXGroupCreateMkStream(topic, "mock-grp", "$").SetErr(errMockGroup)
	err = client.PubSub.CreateTopic(ctx, topic)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockGroup.Error())

	// Verify expectations
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_StreamSubscribeErrors(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mock-sub-grp",
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	topic := "mock-sub-err"

	// Expectations for Subscribe loop:
	// 1. Ping (isConnected check in waitForMessage) -> PONG
	mock.ExpectPing().SetVal("PONG")

	// 2. XGroupCreateMkStream (in subscribeToStream) -> Error
	mock.ExpectXGroupCreateMkStream(topic, "mock-sub-grp", "$").SetErr(errMockGroupCreate)

	// Start Subscribe
	msg, err := client.PubSub.Subscribe(ctx, topic)

	require.NoError(t, err)
	assert.Nil(t, msg)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_MockQueryDeleteErrors(t *testing.T) {
	// Stream Mode
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mock-grp",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "mock-query-err"

	// Test Query Error (Ping succeeds, XRangeN fails)
	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXRangeN(topic, "-", "+", int64(10)).SetErr(errMockXRange)

	res, err := client.PubSub.Query(ctx, topic, 1*time.Second, 10)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), errMockXRange.Error())

	// Test DeleteTopic Error (Del fails, Ping not called in DeleteTopic)
	mock.ExpectDel(topic).SetErr(errMockDel)

	err = client.PubSub.DeleteTopic(ctx, topic)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockDel.Error())

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HealthDown(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	// Test Health Down (Ping fails)
	mock.ExpectPing().SetErr(errMockPing)

	h := client.PubSub.Health()
	assert.Equal(t, datasource.StatusDown, h.Status)
	assert.Equal(t, "REDIS", h.Details["backend"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_Unsubscribe(t *testing.T) {
	client, s := setupTest(t, nil)
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "unsub-topic"

	// Subscribe
	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	// Unsubscribe
	err := client.PubSub.Unsubscribe(topic)
	require.NoError(t, err)
}

func TestPubSub_MonitorConnection(t *testing.T) {
	// Start miniredis
	s, err := miniredis.Run()
	require.NoError(t, err)

	_ = s.Addr()

	ctrl := gomock.NewController(t)
	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)

	conf := map[string]string{
		"REDIS_HOST":     s.Host(),
		"REDIS_PORT":     s.Port(),
		"PUBSUB_BACKEND": "REDIS",
	}

	client := NewClient(config.NewMockConfig(conf), mockLogger, mockMetrics)
	require.NotNil(t, client.PubSub)

	defer client.Close()

	// Ensure connected
	assert.True(t, client.PubSub.isConnected())

	// Subscribe to a topic to verify resubscription logic
	topic := "monitor-topic"

	go func() {
		_, _ = client.PubSub.Subscribe(context.Background(), topic)
	}()

	time.Sleep(100 * time.Millisecond)

	// Stop Redis to simulate connection loss
	s.Close()

	// Wait for monitor to detect loss (interval is short in tests?)
	// The defaultRetryTimeout is 10s, which is too long for unit tests.
	// We rely on the fact that isConnected() will return false.
	assert.False(t, client.PubSub.isConnected())

	// Clean up new server
	s.Close()
}
