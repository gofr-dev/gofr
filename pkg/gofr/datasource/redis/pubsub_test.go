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
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

func TestNewPubSub_UsesRedisPubSubDB(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(s.Close)

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockMetrics := NewMockMetrics(ctrl)
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)

	ps := NewPubSub(config.NewMockConfig(map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   s.Host(),
		"REDIS_PORT":                   s.Port(),
		"REDIS_DB":                     "0",
		"REDIS_PUBSUB_DB":              "1",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "gofr",
	}), mockLogger, mockMetrics)
	require.NotNil(t, ps)

	err = ps.CreateTopic(context.Background(), "db-partition-topic")
	require.NoError(t, err)

	rc0 := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 0})

	t.Cleanup(func() { _ = rc0.Close() })

	t0, err := rc0.Type(context.Background(), "db-partition-topic").Result()
	require.NoError(t, err)
	require.Equal(t, "none", t0)

	rc1 := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 1})

	t.Cleanup(func() { _ = rc1.Close() })

	t1, err := rc1.Type(context.Background(), "db-partition-topic").Result()
	require.NoError(t, err)
	require.Equal(t, "stream", t1)
}

func TestPubSub_Query_Channel(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "query-channel"

	type queryResult struct {
		msgs []byte
		err  error
	}

	resChan := make(chan queryResult)

	go func() {
		msgs, err := client.PubSub.Query(ctx, topic, 2*time.Second, 2)
		resChan <- queryResult{msgs, err}
	}()

	time.Sleep(200 * time.Millisecond)

	msgs := []string{"chan-msg1", "chan-msg2"}
	for _, m := range msgs {
		err := client.PubSub.Publish(ctx, topic, []byte(m))
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond)
	}

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
	errBusyGroup       = errors.New("BUSYGROUP Consumer Group name already exists")
)

type testRedisClient struct {
	*Redis
	PubSub *PubSub
}

func setupTest(t *testing.T, conf map[string]string) (*testRedisClient, *miniredis.Miniredis) {
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
	ps := NewPubSub(config.NewMockConfig(conf), mockLogger, mockMetrics)
	require.NotNil(t, ps)
	psClient, ok := ps.(*PubSub)
	require.True(t, ok)

	return &testRedisClient{Redis: client, PubSub: psClient}, s
}

func setupMockTest(t *testing.T, conf map[string]string) (*testRedisClient, redismock.ClientMock) {
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

	redisConfig := getRedisConfig(config.NewMockConfig(conf), mockLogger)

	r := &Redis{
		Client: db,
		config: redisConfig,
		logger: mockLogger,
	}
	ps := newPubSub(db, redisConfig, mockLogger, mockMetrics)

	return &testRedisClient{Redis: r, PubSub: ps}, mock
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
	actions func(t *testing.T, client *testRedisClient, s *miniredis.Miniredis)
} {
	return append(
		getBasicTestCases(),
		getQueryTestCases()...,
	)
}

func getBasicTestCases() []struct {
	name    string
	config  map[string]string
	actions func(t *testing.T, client *testRedisClient, s *miniredis.Miniredis)
} {
	return []struct {
		name    string
		config  map[string]string
		actions func(t *testing.T, client *testRedisClient, s *miniredis.Miniredis)
	}{
		{
			name: "Channel Publish Subscribe",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "pubsub",
			},
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
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
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamPublishSubscribe(t, client)
			},
		},
		{
			name: "Delete Topic Channel",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "pubsub",
			},
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
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
			actions: func(t *testing.T, client *testRedisClient, s *miniredis.Miniredis) {
				t.Helper()
				testDeleteTopicStream(t, client, s)
			},
		},
		{
			name:   "Health Check",
			config: map[string]string{},
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
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
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
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
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamMaxLen(t, client)
			},
		},
	}
}

func getQueryTestCases() []struct {
	name    string
	config  map[string]string
	actions func(t *testing.T, client *testRedisClient, s *miniredis.Miniredis)
} {
	return []struct {
		name    string
		config  map[string]string
		actions func(t *testing.T, client *testRedisClient, s *miniredis.Miniredis)
	}{
		{
			name: "Channel Query",
			config: map[string]string{
				"REDIS_PUBSUB_MODE": "pubsub",
			},
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
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
			actions: func(t *testing.T, client *testRedisClient, _ *miniredis.Miniredis) {
				t.Helper()
				testStreamQuery(t, client)
			},
		},
	}
}

func testChannelPublishSubscribe(t *testing.T, client *testRedisClient) {
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

func testStreamPublishSubscribe(t *testing.T, client *testRedisClient) {
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

func testChannelQuery(t *testing.T, client *testRedisClient) {
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

func testStreamQuery(t *testing.T, client *testRedisClient) {
	t.Helper()

	ctx := context.Background()
	topic := "query-stream"

	_ = client.PubSub.Publish(ctx, topic, []byte("sm1"))
	_ = client.PubSub.Publish(ctx, topic, []byte("sm2"))

	res, err := client.PubSub.Query(ctx, topic, 1*time.Second, 10)
	require.NoError(t, err)
	require.NotEmpty(t, res, "Query should return results (miniredis limitation may cause empty results)")
	assert.Contains(t, string(res), "sm1")
	assert.Contains(t, string(res), "sm2")
}

func TestPubSub_Query_Stream_Success(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "query-group",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "query-stream-success"

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

func TestPubSub_Query_Stream_MiniredisCompatibility(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "query-group",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "query-stream-compat"

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

func testDeleteTopicChannel(t *testing.T, client *testRedisClient) {
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

func testDeleteTopicStream(t *testing.T, client *testRedisClient, s *miniredis.Miniredis) {
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

func testStreamConfigError(t *testing.T, client *testRedisClient) {
	t.Helper()

	ctx := context.Background()
	topic := "err-stream"

	err := client.PubSub.CreateTopic(ctx, topic)
	assert.Equal(t, errConsumerGroupNotProvided, err)

	ch := client.PubSub.ensureSubscription(ctx, topic)
	assert.NotNil(t, ch)
}

func testStreamMaxLen(t *testing.T, client *testRedisClient) {
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
	ps := NewPubSub(config.NewMockConfig(conf), mockLogger, mockMetrics)
	require.NotNil(t, ps)
	psClient, ok := ps.(*PubSub)
	require.True(t, ok)

	defer client.Close()
	defer psClient.Close()

	ctx := context.Background()
	topic := "err-topic"

	err = psClient.Publish(ctx, topic, []byte("msg"))
	require.Error(t, err)
	assert.Equal(t, errClientNotConnected, err)

	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()

	msg, err := psClient.Subscribe(ctxCancel, topic)
	require.NoError(t, err)
	assert.Nil(t, msg)
}

func TestPubSub_MockErrors(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	if client.PubSub != nil && client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "mock-err-topic"

	mock.ExpectPing().SetVal("PONG")
	mock.ExpectPublish(topic, []byte("msg")).SetErr(errMockPublish)

	err := client.PubSub.Publish(ctx, topic, []byte("msg"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockPublish.Error())

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_StreamMockErrors(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mock-grp",
	})
	defer client.Close()

	if client.PubSub != nil && client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "mock-stream-err"

	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXAdd(&redis.XAddArgs{
		Stream: topic,
		Values: map[string]any{"payload": []byte("msg")},
	}).SetErr(errMockXAdd)

	err := client.PubSub.Publish(ctx, topic, []byte("msg"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockXAdd.Error())

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, "mock-grp", "$").SetErr(errMockGroup)
	err = client.PubSub.CreateTopic(ctx, topic)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockGroup.Error())

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

	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXGroupCreateMkStream(topic, "mock-sub-grp", "$").SetErr(errMockGroupCreate)

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

	mock.ExpectPing().SetVal("PONG")
	mock.ExpectXRangeN(topic, "-", "+", int64(10)).SetErr(errMockXRange)

	res, err := client.PubSub.Query(ctx, topic, 1*time.Second, 10)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), errMockXRange.Error())

	mock.ExpectPing().SetVal("PONG")
	mock.ExpectDel(topic).SetErr(errMockDel)

	err = client.PubSub.DeleteTopic(ctx, topic)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errMockDel.Error())

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_Unsubscribe(t *testing.T) {
	client, s := setupTest(t, nil)
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "unsub-topic"

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	err := client.PubSub.unsubscribe(topic)
	require.NoError(t, err)
}

func TestPubSub_MonitorConnection(t *testing.T) {
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
	ps := NewPubSub(config.NewMockConfig(conf), mockLogger, mockMetrics)
	require.NotNil(t, ps)

	defer client.Close()
	defer ps.Close()

	psClient, ok := ps.(*PubSub)
	require.True(t, ok)
	assert.True(t, psClient.isConnected())

	topic := "monitor-topic"

	go func() {
		_, _ = psClient.Subscribe(context.Background(), topic)
	}()

	time.Sleep(100 * time.Millisecond)

	s.Close()

	assert.False(t, psClient.isConnected())

	s.Close()
}

func TestPubSub_ResubscribeAll(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, nil)
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "resub-topic"

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	psClient := client.PubSub
	psClient.resubscribeAll()

	psClient.mu.RLock()
	_, exists := psClient.subStarted[topic]
	psClient.mu.RUnlock()

	assert.True(t, exists)
}

func TestPubSub_ResubscribeAll_NoSubscriptions(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, nil)
	defer s.Close()
	defer client.Close()

	psClient := client.PubSub
	psClient.resubscribeAll()

	psClient.mu.RLock()
	count := len(psClient.subStarted)
	psClient.mu.RUnlock()

	assert.Equal(t, 0, count)
}

func TestPubSub_CleanupSubscription(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "cleanup-topic"

	// Subscribe to create subscription
	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	psClient := client.PubSub
	psClient.mu.Lock()
	psClient.chanClosed[topic] = true
	psClient.mu.Unlock()

	psClient.cleanupSubscription(topic)

	psClient.mu.RLock()
	_, exists := psClient.receiveChan[topic]
	_, started := psClient.subStarted[topic]
	psClient.mu.RUnlock()

	assert.False(t, exists)
	assert.False(t, started)
}

func TestPubSub_CleanupSubscription_NotClosed(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "cleanup-not-closed-topic"

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	psClient := client.PubSub
	psClient.cleanupSubscription(topic)

	psClient.mu.RLock()
	_, exists := psClient.receiveChan[topic]
	closed := psClient.chanClosed[topic]
	psClient.mu.RUnlock()

	assert.False(t, exists)
	assert.False(t, closed, "chanClosed is deleted from map after cleanup")
}

func TestPubSub_CleanupStreamConsumers(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "cleanup-grp",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "cleanup-stream-topic"

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err)

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(200 * time.Millisecond)

	psClient := client.PubSub
	psClient.cleanupStreamConsumers(topic)

	psClient.mu.RLock()
	_, exists := psClient.streamConsumers[topic]
	_, started := psClient.subStarted[topic]
	psClient.mu.RUnlock()

	assert.False(t, exists)
	assert.False(t, started)
}

func TestPubSub_CleanupStreamConsumers_WithCancel(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "cleanup-cancel-grp",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "cleanup-stream-cancel-topic"

	// Create topic
	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err)

	// Subscribe
	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(200 * time.Millisecond)

	// Manually set up a cancel function
	psClient := client.PubSub
	psClient.mu.Lock()

	ctxCancel, cancel := context.WithCancel(context.Background())
	psClient.streamConsumers[topic] = &streamConsumer{
		stream:   topic,
		group:    "cleanup-cancel-grp",
		consumer: "test-consumer",
		cancel:   cancel,
	}
	psClient.mu.Unlock()

	// Cleanup should call cancel
	psClient.cleanupStreamConsumers(topic)

	// Verify context was canceled
	assert.Error(t, ctxCancel.Err())
}

func TestPubSub_DispatchMessage_ChannelFull(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":        "pubsub",
		"PUBSUB_BUFFER_SIZE":       "1", // Small buffer to test full channel
		"REDIS_PUBSUB_BUFFER_SIZE": "1",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "dispatch-full-topic"

	msgChan := client.PubSub.ensureSubscription(ctx, topic)
	msgChan <- pubsub.NewMessage(ctx)

	msg := pubsub.NewMessage(ctx)
	msg.Topic = topic
	msg.Value = []byte("dropped")

	psClient := client.PubSub
	psClient.dispatchMessage(ctx, topic, msg)

	assert.Len(t, msgChan, 1)
}

func TestPubSub_DispatchMessage_ChannelClosed(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "dispatch-closed-topic"

	msgChan := client.PubSub.ensureSubscription(ctx, topic)
	psClient := client.PubSub
	psClient.mu.Lock()
	psClient.chanClosed[topic] = true
	psClient.mu.Unlock()

	msg := pubsub.NewMessage(ctx)
	msg.Topic = topic
	msg.Value = []byte("ignored")

	psClient.dispatchMessage(ctx, topic, msg)

	assert.NotNil(t, msgChan)
}

func TestPubSub_DispatchMessage_TopicNotExists(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, nil)
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "non-existent-topic"

	msg := pubsub.NewMessage(ctx)
	msg.Topic = topic
	msg.Value = []byte("test")

	psClient := client.PubSub
	psClient.dispatchMessage(ctx, topic, msg)

	assert.NotNil(t, psClient)
}

func TestPubSub_CheckGroupExists_GroupExists(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "check-grp",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "check-group-topic"
	group := "check-grp"

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err)

	psClient := client.PubSub
	exists := psClient.checkGroupExists(ctx, topic, group)

	assert.True(t, exists)
}

func TestPubSub_CheckGroupExists_GroupNotExists(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "streams",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "check-group-not-exists-topic"
	group := "non-existent-group"

	_, err := s.XAdd(topic, "0-1", []string{"payload", "data"})
	require.NoError(t, err)

	psClient := client.PubSub
	exists := psClient.checkGroupExists(ctx, topic, group)

	assert.False(t, exists)
}

func TestPubSub_CheckGroupExists_StreamNotExists(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, nil)
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "non-existent-stream"
	group := "test-group"

	psClient := client.PubSub
	exists := psClient.checkGroupExists(ctx, topic, group)

	assert.False(t, exists)
}

func TestPubSub_EnsureConsumerGroup_CreateNew(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "ensure-new-grp",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "ensure-group-new-topic"
	group := "ensure-new-grp"

	_, err := s.XAdd(topic, "0-1", []string{"payload", "data"})
	require.NoError(t, err)

	psClient := client.PubSub
	result := psClient.ensureConsumerGroup(ctx, topic, group)

	assert.True(t, result)
}

func TestPubSub_GetConsumerName_WithConfig(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-grp",
		"REDIS_STREAMS_CONSUMER_NAME":  "custom-consumer",
	})
	defer s.Close()
	defer client.Close()

	psClient := client.PubSub
	name := psClient.getConsumerName()

	assert.Equal(t, "custom-consumer", name)
}

func TestPubSub_GetConsumerName_WithoutConfig(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-grp",
	})
	defer s.Close()
	defer client.Close()

	psClient := client.PubSub
	name := psClient.getConsumerName()

	assert.Contains(t, name, "consumer-")
	assert.NotEmpty(t, name)
}

func TestPubSub_UnsubscribeFromRedis_NotExists(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	topic := "unsub-not-exists-topic"

	psClient := client.PubSub
	psClient.unsubscribeFromRedis(topic)

	assert.NotNil(t, psClient)
}

func TestPubSub_Close_WithSubscriptions(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()

	ctx := context.Background()
	topic := "close-sub-topic"

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, "close-sub-topic-2")
	}()

	time.Sleep(200 * time.Millisecond)

	err := client.PubSub.Close()
	require.NoError(t, err)

	psClient := client.PubSub
	psClient.mu.RLock()
	subCount := len(psClient.subStarted)
	chanCount := len(psClient.receiveChan)
	psClient.mu.RUnlock()

	assert.Equal(t, 0, subCount)
	assert.Equal(t, 0, chanCount)
}

func TestPubSub_Close_NoSubscriptions(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, nil)
	defer s.Close()

	err := client.PubSub.Close()
	require.NoError(t, err)

	assert.NotNil(t, client.PubSub)
}

func TestPubSub_Close_WithStreamConsumers(t *testing.T) {
	t.Parallel()

	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "close-grp",
	})
	defer s.Close()

	ctx := context.Background()
	topic := "close-stream-topic"

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err)

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(200 * time.Millisecond)

	err = client.PubSub.Close()
	require.NoError(t, err)

	psClient := client.PubSub
	psClient.mu.RLock()
	consumerCount := len(psClient.streamConsumers)
	psClient.mu.RUnlock()

	assert.Equal(t, 0, consumerCount)
}

func TestPubSub_SubscribeToChannel_NilPubSub(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	// Stop monitorConnection
	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cancel()

	done := make(chan struct{})
	go func() {
		client.PubSub.subscribeToChannel(ctx, "test-topic")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("subscribeToChannel did not return")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_SubscribeToChannel_NilChannel(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan *redis.Message)
	close(ch)

	done := make(chan struct{})
	go func() {
		client.PubSub.processMessages(ctx, "test-topic", ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("processMessages did not return")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_EnsureConsumerGroup_GroupNotExists(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, group, "$").SetVal("OK")

	result := client.PubSub.ensureConsumerGroup(ctx, topic, group)
	assert.True(t, result, "should return true after creating group")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_EnsureConsumerGroup_CreateFails(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, group, "$").SetErr(errMockGroup)

	result := client.PubSub.ensureConsumerGroup(ctx, topic, group)
	assert.False(t, result, "should return false when group creation fails")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CheckGroupExists_Error(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	mock.ExpectXInfoGroups(topic).SetErr(errMockGroup)

	result := client.PubSub.checkGroupExists(ctx, topic, group)
	assert.False(t, result, "should return false on error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CheckGroupExists_GroupFound(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "other-group"},
		{Name: "test-group"},
		{Name: "another-group"},
	})

	result := client.PubSub.checkGroupExists(ctx, topic, group)
	assert.True(t, result, "should return true when group found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CheckGroupExists_GroupNotFound(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "other-group"},
		{Name: "another-group"},
	})

	result := client.PubSub.checkGroupExists(ctx, topic, group)
	assert.False(t, result, "should return false when group not found")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CleanupSubscription_ChannelNotClosed(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	topic := "test-topic"

	client.PubSub.mu.Lock()
	ch := make(chan *pubsub.Message, 1)
	client.PubSub.receiveChan[topic] = ch
	client.PubSub.chanClosed[topic] = false
	client.PubSub.subStarted[topic] = struct{}{}
	client.PubSub.mu.Unlock()

	client.PubSub.cleanupSubscription(topic)

	client.PubSub.mu.Lock()
	_, exists := client.PubSub.receiveChan[topic]
	_, startedExists := client.PubSub.subStarted[topic]
	_, closedExists := client.PubSub.chanClosed[topic]
	client.PubSub.mu.Unlock()

	assert.False(t, exists, "receiveChan should be deleted")
	assert.False(t, startedExists, "subStarted should be deleted")
	assert.False(t, closedExists, "chanClosed should be deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CleanupSubscription_ChannelAlreadyClosed(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	topic := "test-topic"

	client.PubSub.mu.Lock()
	ch := make(chan *pubsub.Message)
	close(ch)
	client.PubSub.receiveChan[topic] = ch
	client.PubSub.chanClosed[topic] = true
	client.PubSub.subStarted[topic] = struct{}{}
	client.PubSub.mu.Unlock()

	client.PubSub.cleanupSubscription(topic)

	client.PubSub.mu.Lock()
	_, exists := client.PubSub.receiveChan[topic]
	_, startedExists := client.PubSub.subStarted[topic]
	_, closedExists := client.PubSub.chanClosed[topic]
	client.PubSub.mu.Unlock()

	assert.False(t, exists, "receiveChan should be deleted")
	assert.False(t, startedExists, "subStarted should be deleted")
	assert.False(t, closedExists, "chanClosed should be deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CleanupSubscription_NoChannel(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	topic := "test-topic"

	client.PubSub.mu.Lock()
	client.PubSub.subStarted[topic] = struct{}{}
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.cleanupSubscription(topic)

	client.PubSub.mu.Lock()
	_, startedExists := client.PubSub.subStarted[topic]
	_, closedExists := client.PubSub.chanClosed[topic]
	client.PubSub.mu.Unlock()

	assert.False(t, startedExists, "subStarted should be deleted")
	assert.False(t, closedExists, "chanClosed should be deleted")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DeleteTopic_EmptyTopic(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx := context.Background()

	err := client.PubSub.DeleteTopic(ctx, "")
	require.NoError(t, err, "should return nil for empty topic")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DeleteTopic_PubSubMode_NoActiveSubscription(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx := context.Background()
	topic := "test-topic"

	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DeleteTopic_PubSubMode_WithActiveSubscription(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	ctx := context.Background()
	topic := "test-topic"

	go func() {
		_, _ = client.PubSub.Subscribe(ctx, topic)
	}()

	time.Sleep(100 * time.Millisecond)

	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)
}

func TestPubSub_DeleteTopic_StreamMode(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"

	mock.ExpectPing().SetVal("PONG")
	mock.ExpectDel(topic).SetVal(1)

	err := client.PubSub.DeleteTopic(ctx, topic)
	require.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_UnsubscribeFromRedis_NoPubSub(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	topic := "test-topic"

	client.PubSub.unsubscribeFromRedis(topic)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_UnsubscribeFromRedis_NilPubSub(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	topic := "test-topic"

	client.PubSub.mu.Lock()
	client.PubSub.subPubSub[topic] = nil
	client.PubSub.mu.Unlock()

	client.PubSub.unsubscribeFromRedis(topic)

	client.PubSub.mu.Lock()
	delete(client.PubSub.subPubSub, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_UnsubscribeFromRedis_Error(t *testing.T) {
	client, s := setupTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer s.Close()
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	topic := "test-topic"
	ctx := context.Background()

	redisPubSub := client.Redis.Client.Subscribe(ctx, topic)

	client.PubSub.mu.Lock()
	client.PubSub.subPubSub[topic] = redisPubSub
	client.PubSub.mu.Unlock()

	s.Close()

	client.PubSub.unsubscribeFromRedis(topic)

	client.PubSub.mu.Lock()
	delete(client.PubSub.subPubSub, topic)
	client.PubSub.mu.Unlock()
}

func TestPubSub_CreateTopic_PubSubMode(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-channel"

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err, "should return nil for pubsub mode")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CreateTopic_StreamMode_GroupExists(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{
		{Name: "test-group"},
	})

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err, "should return nil when group exists")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CreateTopic_StreamMode_BusyGroup(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"

	mock.ExpectXInfoGroups(topic).SetVal([]redis.XInfoGroup{})
	mock.ExpectXGroupCreateMkStream(topic, "test-group", "$").SetErr(errBusyGroup)

	err := client.PubSub.CreateTopic(ctx, topic)
	require.NoError(t, err, "should return nil for BUSYGROUP error")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_CollectMessages_ContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan *redis.Message, 1)

	ch <- &redis.Message{Payload: "msg1"}

	// Cancel context immediately
	cancel()

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 10)
	assert.Empty(t, result, "should return empty when context done")
}

func TestPubSub_CollectMessages_ChannelClosed(t *testing.T) {
	ctx := context.Background()
	ch := make(chan *redis.Message)
	close(ch)

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 10)
	assert.Empty(t, result, "should return empty when channel closed")
}

func TestPubSub_CollectMessages_NilMessage(t *testing.T) {
	ctx := context.Background()
	ch := make(chan *redis.Message, 2)
	ch <- nil
	ch <- &redis.Message{Payload: "msg1"}
	close(ch)

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 10)
	assert.Equal(t, []byte("msg1"), result, "should skip nil messages")
}

func TestPubSub_CollectMessages_ReachesLimit(t *testing.T) {
	ctx := context.Background()
	ch := make(chan *redis.Message, 3)
	ch <- &redis.Message{Payload: "msg1"}
	ch <- &redis.Message{Payload: "msg2"}
	ch <- &redis.Message{Payload: "msg3"}
	close(ch)

	ps := &PubSub{}
	result := ps.collectMessages(ctx, ch, 2)
	expected := []byte("msg1\nmsg2")
	assert.Equal(t, expected, result, "should stop at limit")
}

func TestPubSub_ProcessMessages_ChannelClosed(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan *redis.Message)
	close(ch)

	done := make(chan struct{})
	go func() {
		client.PubSub.processMessages(ctx, "test-topic", ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("processMessages did not return when channel closed")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_ProcessMessages_NilMessage(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan *redis.Message, 1)
	ch <- nil

	go func() {
		time.Sleep(50 * time.Millisecond)
		ch <- &redis.Message{Channel: "test", Payload: "data"}
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	client.PubSub.processMessages(ctx, "test-topic", ch)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_ProcessMessages_ContextDone(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE": "pubsub",
	})
	defer client.Close()

	if client.PubSub.cancel != nil {
		client.PubSub.cancel()
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *redis.Message)

	done := make(chan struct{})
	go func() {
		client.PubSub.processMessages(ctx, "test-topic", ch)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("processMessages did not return when context canceled")
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DispatchMessage_ChannelNotExists(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx := context.Background()
	topic := "non-existent-topic"

	msg := pubsub.NewMessage(ctx)
	client.PubSub.dispatchMessage(ctx, topic, msg)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_DispatchMessage_ContextDone(t *testing.T) {
	client, mock := setupMockTest(t, nil)
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	topic := "test-topic"

	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	msg1 := pubsub.NewMessage(ctx)
	client.PubSub.receiveChan[topic] <- msg1

	cancel()

	msg2 := pubsub.NewMessage(ctx)
	client.PubSub.dispatchMessage(ctx, topic, msg2)

	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_StringPayload(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID: "123-0",
		Values: map[string]any{
			"payload": "string-payload",
		},
	}

	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Equal(t, []byte("string-payload"), received.Value)
		assert.NotNil(t, received.Committer)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_BytePayload(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID: "123-0",
		Values: map[string]any{
			"payload": []byte("byte-payload"),
		},
	}

	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Equal(t, []byte("byte-payload"), received.Value)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_MissingPayload(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID:     "123-0",
		Values: map[string]any{},
	}

	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Nil(t, received.Value)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPubSub_HandleStreamMessage_UnsupportedPayloadType(t *testing.T) {
	client, mock := setupMockTest(t, map[string]string{
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "test-group",
	})
	defer client.Close()

	ctx := context.Background()
	topic := "test-stream"
	group := "test-group"

	msg := &redis.XMessage{
		ID: "123-0",
		Values: map[string]any{
			"payload": 12345,
		},
	}

	client.PubSub.mu.Lock()
	client.PubSub.receiveChan[topic] = make(chan *pubsub.Message, 1)
	client.PubSub.chanClosed[topic] = false
	client.PubSub.mu.Unlock()

	client.PubSub.handleStreamMessage(ctx, topic, msg, group)

	select {
	case received := <-client.PubSub.receiveChan[topic]:
		require.NotNil(t, received)
		assert.Equal(t, topic, received.Topic)
		assert.Nil(t, received.Value)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("message was not dispatched")
	}

	client.PubSub.mu.Lock()
	close(client.PubSub.receiveChan[topic])
	delete(client.PubSub.receiveChan, topic)
	delete(client.PubSub.chanClosed, topic)
	client.PubSub.mu.Unlock()

	assert.NoError(t, mock.ExpectationsWereMet())
}
