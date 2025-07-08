package couchbase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"go.uber.org/mock/gomock"
)

func TestClient_New(t *testing.T) {
	client := New(&Config{
		Host:              "localhost",
		User:              "Administrator",
		Password:          "password",
		Bucket:            "gofr",
		ConnectionTimeout: time.Second * 5,
	})

	require.NotNil(t, client)
}

func TestClient_Upsert(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)
	cluster := NewMockclusterProvider(ctrl)

	tests := []struct {
		name       string
		key        string
		document   any
		result     any
		setupMocks func(*MockbucketProvider, *MockcollectionProvider)
		client     *Client
		wantErr    error
	}{
		{
			name:     "success: upsert document with *gocb.MutationResult",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &gocb.MutationResult{},
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider) {
				gomock.InOrder(bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil))
			},
			wantErr: nil,
		},
		{
			name:     "success: upsert document with **gocb.MutationResult",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   new(*gocb.MutationResult),
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider) {
				gomock.InOrder(bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil))
			},
			wantErr: nil,
		},
		{
			name:     "error: from collection.Upsert",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &gocb.MutationResult{},
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider) {
				gomock.InOrder(bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(nil, assert.AnError))
			},
			wantErr: assert.AnError,
		},
		{
			name:     "error: wrong result type",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &struct{}{},
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider) {
				gomock.InOrder(bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil))
			},
			wantErr: errWrongResultType,
		},
		{
			name:       "error: bucket not initialized",
			key:        "test-key",
			document:   map[string]string{"key": "value"},
			result:     &gocb.MutationResult{},
			setupMocks: func(_ *MockbucketProvider, _ *MockcollectionProvider) {},
			client: &Client{
				bucket: nil,
			},
			wantErr: errBucketNotInitialized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := NewMockbucketProvider(ctrl)
			collection := NewMockcollectionProvider(ctrl)

			tt.setupMocks(bucket, collection)

			client := tt.client
			if client == nil {
				client = &Client{
					cluster: cluster,
					bucket:  bucket,
					config:  &Config{},
					logger:  logger,
					metrics: metrics,
				}
			}

			err := client.Upsert(context.Background(), tt.key, tt.document, tt.result)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)
	cluster := NewMockclusterProvider(ctrl)

	tests := []struct {
		name       string
		key        string
		result     any
		setupMocks func(*MockbucketProvider, *MockcollectionProvider, *MockgetResultProvider)
		client     *Client
		wantErr    error
	}{
		{
			name:   "success: get document",
			key:    "test-key",
			result: &struct{}{},
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider, getResult *MockgetResultProvider) {
				gomock.InOrder(
					bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Get("test-key", gomock.Any()).Return(getResult, nil),
					getResult.EXPECT().Content(gomock.Any()).Return(nil),
				)
			},
			wantErr: nil,
		},
		{
			name:   "error: from collection.Get",
			key:    "test-key",
			result: &struct{}{},
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider, _ *MockgetResultProvider) {
				gomock.InOrder(
					bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Get("test-key", gomock.Any()).Return(nil, assert.AnError),
				)
			},
			wantErr: assert.AnError,
		},
		{
			name:   "error: from getResult.Content",
			key:    "test-key",
			result: &struct{}{},
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider, getResult *MockgetResultProvider) {
				gomock.InOrder(
					bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Get("test-key", gomock.Any()).Return(getResult, nil),
					getResult.EXPECT().Content(gomock.Any()).Return(assert.AnError),
				)
			},
			wantErr: assert.AnError,
		},
		{
			name:       "error: bucket not initialized",
			key:        "test-key",
			result:     nil,
			setupMocks: func(_ *MockbucketProvider, _ *MockcollectionProvider, _ *MockgetResultProvider) {},
			client: &Client{
				bucket: nil,
			},
			wantErr: errBucketNotInitialized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := NewMockbucketProvider(ctrl)
			collection := NewMockcollectionProvider(ctrl)
			getResult := NewMockgetResultProvider(ctrl)

			tt.setupMocks(bucket, collection, getResult)

			client := tt.client
			if client == nil {
				client = &Client{
					cluster: cluster,
					bucket:  bucket,
					config:  &Config{},
					logger:  logger,
					metrics: metrics,
				}
			}

			err := client.Get(context.Background(), tt.key, tt.result)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_Remove(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)
	cluster := NewMockclusterProvider(ctrl)

	tests := []struct {
		name       string
		key        string
		setupMocks func(*MockbucketProvider, *MockcollectionProvider)
		client     *Client
		wantErr    error
	}{
		{
			name: "success: remove document",
			key:  "test-key",
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider) {
				gomock.InOrder(
					bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Remove("test-key", gomock.Any()).Return(&gocb.MutationResult{}, nil),
				)
			},
			wantErr: nil,
		},
		{
			name: "error: from collection.Remove",
			key:  "test-key",
			setupMocks: func(bucket *MockbucketProvider, collection *MockcollectionProvider) {
				gomock.InOrder(
					bucket.EXPECT().DefaultCollection().Return(collection),
					collection.EXPECT().Remove("test-key", gomock.Any()).Return(nil, assert.AnError),
				)
			},
			wantErr: assert.AnError,
		},
		{
			name:       "error: bucket not initialized",
			key:        "test-key",
			setupMocks: func(_ *MockbucketProvider, _ *MockcollectionProvider) {},
			client: &Client{
				bucket: nil,
			},
			wantErr: errBucketNotInitialized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := NewMockbucketProvider(ctrl)
			collection := NewMockcollectionProvider(ctrl)

			tt.setupMocks(bucket, collection)

			client := tt.client
			if client == nil {
				client = &Client{
					cluster: cluster,
					bucket:  bucket,
					config:  &Config{},
					logger:  logger,
					metrics: metrics,
				}
			}

			err := client.Remove(context.Background(), tt.key)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_Query(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)

	tests := []struct {
		name       string
		statement  string
		params     map[string]any
		result     any
		setupMocks func(*MockclusterProvider, *MockresultProvider)
		client     *Client
		wantErr    error
	}{
		{
			name:      "success: N1QL query",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setupMocks: func(cluster *MockclusterProvider, queryResult *MockresultProvider) {
				gomock.InOrder(cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(queryResult, nil),
					queryResult.EXPECT().Next().Return(true),
					queryResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					queryResult.EXPECT().Next().Return(false),
					queryResult.EXPECT().Err().Return(nil),
					queryResult.EXPECT().Close().Return(nil))
			},
			wantErr: nil,
		},
		{
			name:      "error: from cluster.Query",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setupMocks: func(cluster *MockclusterProvider, _ *MockresultProvider) {
				cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
		{
			name:      "error: N1QL query iteration error",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setupMocks: func(cluster *MockclusterProvider, queryResult *MockresultProvider) {
				gomock.InOrder(cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(queryResult, nil),
					queryResult.EXPECT().Next().Return(false),
					queryResult.EXPECT().Err().Return(assert.AnError),
					queryResult.EXPECT().Close().Return(nil))
			},
			wantErr: assert.AnError,
		},
		{
			name:      "error: failed to unmarshal N1QL results into target",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &struct{}{}, // This will cause unmarshal error if tempResults is not compatible
			setupMocks: func(cluster *MockclusterProvider, queryResult *MockresultProvider) {
				gomock.InOrder(cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(queryResult, nil),
					queryResult.EXPECT().Next().Return(true),
					queryResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					queryResult.EXPECT().Next().Return(false),
					queryResult.EXPECT().Err().Return(nil),
					queryResult.EXPECT().Close().Return(nil))
			},
			wantErr: errFailedToUnmarshalN1QL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := NewMockclusterProvider(ctrl)
			queryResult := NewMockresultProvider(ctrl)

			tt.setupMocks(cluster, queryResult)

			client := tt.client
			if client == nil {
				client = &Client{
					cluster: cluster,
					config:  &Config{},
					logger:  logger,
					metrics: metrics,
				}
			}

			err := client.Query(context.Background(), tt.statement, tt.params, tt.result)

			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_UseLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	client := New(&Config{})
	client.UseLogger(logger)
	assert.Equal(t, logger, client.logger)
}

func TestClient_UseMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	client := New(&Config{})
	client.UseMetrics(metrics)
	assert.Equal(t, metrics, client.metrics)
}

func TestClient_UseTracer(t *testing.T) {
	client := New(&Config{})
	client.UseTracer(trace.DefaultTracer)
	assert.NotNil(t, client.tracer)
}

func TestClient_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cluster := NewMockclusterProvider(ctrl)
	cluster.EXPECT().Close(nil).Return(nil)

	client := &Client{
		cluster: cluster,
	}

	err := client.Close(nil)
	assert.NoError(t, err)
}

func TestClient_HealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name       string
		setupMocks func(*MockclusterProvider)
		wantStatus string
		wantErr    error
	}{
		{
			name: "success: cluster is up",
			setupMocks: func(cluster *MockclusterProvider) {
				cluster.EXPECT().Ping(nil).Return(&gocb.PingResult{}, nil)
			},
			wantStatus: "UP",
			wantErr:    nil,
		},
		{
			name: "error: cluster is down",
			setupMocks: func(cluster *MockclusterProvider) {
				cluster.EXPECT().Ping(nil).Return(nil, assert.AnError)
			},
			wantStatus: "DOWN",
			wantErr:    errStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := NewMockclusterProvider(ctrl)
			tt.setupMocks(cluster)

			client := &Client{
				cluster: cluster,
				config: &Config{
					Host:   "localhost",
					Bucket: "gofr",
				},
			}

			health, err := client.HealthCheck()

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantStatus, health.(*Health).Status)
		})
	}
}

func Test_generateCouchbaseURI(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		want    string
		wantErr bool
	}{
		{
			name: "success: with host",
			config: &Config{
				Host: "localhost",
			},
			want:    "couchbase://localhost",
			wantErr: false,
		},
		{
			name: "success: with URI",
			config: &Config{
				URI: "couchbase://remotehost",
			},
			want:    "couchbase://remotehost",
			wantErr: false,
		},
		{
			name:    "error: missing host and URI",
			config:  &Config{},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateCouchbaseURI(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateCouchbaseURI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("generateCouchbaseURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_AnalyticsQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	logger := NewMockLogger(ctrl)
	metrics := NewMockMetrics(ctrl)

	tests := []struct {
		name       string
		statement  string
		params     map[string]any
		result     any
		setupMocks func(*MockclusterProvider, *MockresultProvider)
		client     *Client
		wantErr    error
	}{
		{
			name:      "success: Analytics query",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setupMocks: func(cluster *MockclusterProvider, analyticsResult *MockresultProvider) {
				gomock.InOrder(cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(analyticsResult, nil),
					analyticsResult.EXPECT().Next().Return(true),
					analyticsResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test_analytics"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					analyticsResult.EXPECT().Next().Return(false),
					analyticsResult.EXPECT().Err().Return(nil),
					analyticsResult.EXPECT().Close().Return(nil))
			},
			wantErr: nil,
		},
		{
			name:      "error: from cluster.AnalyticsQuery",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setupMocks: func(cluster *MockclusterProvider, _ *MockresultProvider) {
				cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: assert.AnError,
		},
		{
			name:      "error: failed to unmarshal Analytics query row",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setupMocks: func(cluster *MockclusterProvider, analyticsResult *MockresultProvider) {
				gomock.InOrder(cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(analyticsResult, nil),
					analyticsResult.EXPECT().Next().Return(true),
					analyticsResult.EXPECT().Row(gomock.Any()).Return(assert.AnError),
					analyticsResult.EXPECT().Close().Return(nil))
			},
			wantErr: assert.AnError,
		},
		{
			name:      "error: failed to unmarshal analytics results into target",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &struct{}{}, // This will cause unmarshal error if tempResults is not compatible
			setupMocks: func(cluster *MockclusterProvider, analyticsResult *MockresultProvider) {
				gomock.InOrder(cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(analyticsResult, nil),
					analyticsResult.EXPECT().Next().Return(true),
					analyticsResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test_analytics"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					analyticsResult.EXPECT().Next().Return(false),
					analyticsResult.EXPECT().Err().Return(nil),
					analyticsResult.EXPECT().Close().Return(nil))
			},
			wantErr: errFailedToUnmarshalAnalytics,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := NewMockclusterProvider(ctrl)
			analyticsResult := NewMockresultProvider(ctrl)

			tt.setupMocks(cluster, analyticsResult)

			client := tt.client
			if client == nil {
				client = &Client{
					cluster: cluster,
					config:  &Config{},
					logger:  logger,
					metrics: metrics,
				}
			}

			err := client.AnalyticsQuery(context.Background(), tt.statement, tt.params, tt.result)

			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
