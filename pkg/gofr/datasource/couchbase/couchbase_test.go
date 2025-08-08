package couchbase

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

var (
	errMockTransaction = errors.New("transaction failed")
	errLogic           = errors.New("logic error")
)

type testMocks struct {
	logger       *MockLogger
	metrics      *MockMetrics
	cluster      *MockclusterProvider
	bucket       *MockbucketProvider
	transactions *MocktransactionsProvider
	collection   *MockcollectionProvider
	getResult    *MockgetResultProvider
	scope        *MockscopeProvider
	queryResult  *MockresultProvider
}

func newTestMocks(t *testing.T) *testMocks {
	t.Helper()
	ctrl := gomock.NewController(t)

	return &testMocks{
		logger:       NewMockLogger(ctrl),
		metrics:      NewMockMetrics(ctrl),
		cluster:      NewMockclusterProvider(ctrl),
		bucket:       NewMockbucketProvider(ctrl),
		transactions: NewMocktransactionsProvider(ctrl),
		collection:   NewMockcollectionProvider(ctrl),
		getResult:    NewMockgetResultProvider(ctrl),
		scope:        NewMockscopeProvider(ctrl),
		queryResult:  NewMockresultProvider(ctrl),
	}
}

func TestClient_New(t *testing.T) {
	client := New(&Config{
		Host:              "localhost",
		User:              "Administrator",
		Password:          "password",
		Bucket:            "gofr",
		ConnectionTimeout: time.Second * 5,
		URI:               "",
	})

	require.NotNil(t, client)
}

func TestClient_Upsert(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		document any
		result   any
		setup    func(mocks *testMocks) *Client
		wantErr  error
	}{
		{
			name:     "success: upsert document with *gocb.MutationResult",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &gocb.MutationResult{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil),
					mocks.logger.EXPECT().Debug(gomock.Any()),
					mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes(),
					mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name:     "success: upsert document with **gocb.MutationResult",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   func() **gocb.MutationResult { var res *gocb.MutationResult; return &res }(),
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil),
					mocks.logger.EXPECT().Debug(gomock.Any()))
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name:     "error: from collection.Upsert",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &gocb.MutationResult{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(nil, gocb.ErrDocumentExists),
					mocks.logger.EXPECT().Debug(gomock.Any()),
					mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes(),
					mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrDocumentExists,
		},
		{
			name:     "error: wrong result type",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &struct{}{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Upsert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil),
					mocks.logger.EXPECT().Debug(gomock.Any()),
					mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes(),
					mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: errWrongResultType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			err := client.Upsert(t.Context(), tt.key, tt.document, tt.result)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestClient_Insert(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		document any
		result   any
		setup    func(mocks *testMocks) *Client
		wantErr  error
	}{
		{
			name:     "success: insert document with *gocb.MutationResult",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &gocb.MutationResult{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Insert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil),
					mocks.logger.EXPECT().Debug(gomock.Any()),
					mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes(),
					mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name:     "error: from collection.Insert",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &gocb.MutationResult{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Insert("test-key", gomock.Any(), gomock.Any()).Return(nil, gocb.ErrDocumentExists),
					mocks.logger.EXPECT().Debug(gomock.Any()),
					mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes(),
					mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrDocumentExists,
		},
		{
			name:     "error: wrong result type",
			key:      "test-key",
			document: map[string]string{"key": "value"},
			result:   &struct{}{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Insert("test-key", gomock.Any(), gomock.Any()).Return(&gocb.MutationResult{}, nil),
					mocks.logger.EXPECT().Debug(gomock.Any()),
					mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes(),
					mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: errWrongResultType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			err := client.Insert(t.Context(), tt.key, tt.document, tt.result)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestClient_Get(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		result  any
		setup   func(mocks *testMocks) *Client
		wantErr error
	}{
		{
			name:   "success: get document",
			key:    "test-key",
			result: &struct{}{},
			setup: func(mocks *testMocks) *Client {
				mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection)
				mocks.collection.EXPECT().Get("test-key", gomock.Any()).Return(mocks.getResult, nil)
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.getResult.EXPECT().Content(gomock.Any()).Return(nil)
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name:   "error: from collection.Get",
			key:    "test-key",
			result: &struct{}{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(
					mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Get("test-key", gomock.Any()).Return(nil, gocb.ErrDocumentNotFound),
					mocks.logger.EXPECT().Debug(gomock.Any()),
				)
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any())
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrDocumentNotFound,
		},
		{
			name:   "error: from getResult.Content",
			key:    "test-key",
			result: &struct{}{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(
					mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Get("test-key", gomock.Any()).Return(mocks.getResult, nil),
					mocks.getResult.EXPECT().Content(gomock.Any()).Return(gocb.ErrDecodingFailure),
				)
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrDecodingFailure,
		},
		{
			name:   "error: bucket not initialized",
			key:    "test-key",
			result: nil,
			setup: func(mocks *testMocks) *Client {
				mocks.logger.EXPECT().Error("bucket not initialized")
				client := &Client{bucket: nil}
				client.UseLogger(mocks.logger)
				return client
			},
			wantErr: errBucketNotInitialized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			err := client.Get(t.Context(), tt.key, tt.result)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestClient_Remove(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		setup   func(mocks *testMocks) *Client
		wantErr error
	}{
		{
			name: "success: remove document",
			key:  "test-key",
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(
					mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Remove("test-key", gomock.Any()).Return(&gocb.MutationResult{}, nil),
					mocks.logger.EXPECT().Debug(gomock.Any()),
				)
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name: "error: from collection.Remove",
			key:  "test-key",
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(
					mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection),
					mocks.collection.EXPECT().Remove("test-key", gomock.Any()).Return(nil, gocb.ErrDocumentNotFound),
					mocks.logger.EXPECT().Debug(gomock.Any()),
				)
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, bucket: mocks.bucket, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrDocumentNotFound,
		},
		{
			name: "error: bucket not initialized",
			key:  "test-key",
			setup: func(mocks *testMocks) *Client {
				mocks.logger.EXPECT().Error("bucket not initialized")
				client := &Client{bucket: nil}
				client.UseLogger(mocks.logger)

				return client
			},
			wantErr: errBucketNotInitialized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			err := client.Remove(t.Context(), tt.key)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_DefaultCollection(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(mocks *testMocks) *Client
		wantCollection *Collection
	}{
		{
			name: "success: default collection returned",
			setup: func(mocks *testMocks) *Client {
				mocks.bucket.EXPECT().DefaultCollection().Return(mocks.collection)
				return &Client{
					bucket:  mocks.bucket,
					logger:  mocks.logger,
					metrics: mocks.metrics,
					tracer:  noop.NewTracerProvider().Tracer("test"),
				}
			},
			wantCollection: &Collection{
				collection: NewMockcollectionProvider(gomock.NewController(t)),
			},
		},
		{
			name: "error: bucket not initialized",
			setup: func(mocks *testMocks) *Client {
				mocks.logger.EXPECT().Error("bucket not initialized")
				return &Client{
					bucket: nil,
					logger: mocks.logger,
				}
			},
			wantCollection: &Collection{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			got := client.defaultCollection()

			// We cannot directly compare the collection, so we check for nil and non-nil cases.
			if tt.wantCollection == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestClient_Scope(t *testing.T) {
	tests := []struct {
		name      string
		scopeName string
		setup     func(mocks *testMocks) *Client
		wantScope *Scope
	}{
		{
			name:      "success: scope returned",
			scopeName: "test-scope",
			setup: func(mocks *testMocks) *Client {
				mocks.bucket.EXPECT().Scope("test-scope").Return(mocks.scope)
				return &Client{
					bucket:  mocks.bucket,
					logger:  mocks.logger,
					metrics: mocks.metrics,
					tracer:  noop.NewTracerProvider().Tracer("test"),
				}
			},
			wantScope: &Scope{
				scope: NewMockscopeProvider(gomock.NewController(t)),
			},
		},
		{
			name:      "error: bucket not initialized",
			scopeName: "test-scope",
			setup: func(mocks *testMocks) *Client {
				mocks.logger.EXPECT().Error("bucket not initialized")
				return &Client{
					bucket: nil,
					logger: mocks.logger,
				}
			},
			wantScope: &Scope{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			got := client.scope(tt.scopeName)

			if tt.wantScope == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestClient_RunTransaction(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mocks *testMocks) *Client
		logic   func(any) error
		wantErr error
	}{
		{
			name: "success: transaction runs",
			setup: func(mocks *testMocks) *Client {
				mocks.cluster.EXPECT().Transactions().Return(mocks.transactions)
				mocks.transactions.EXPECT().Run(gomock.Any(), gomock.Any()).Return(&gocb.TransactionResult{}, nil)
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				return &Client{
					bucket:  mocks.bucket,
					cluster: mocks.cluster,
					logger:  mocks.logger,
					metrics: mocks.metrics,
					config:  &Config{Bucket: "bucket"},
					tracer:  noop.NewTracerProvider().Tracer("test"),
				}
			},
			logic: func(any) error {
				return nil
			},
		},
		{
			name: "error: cluster not initialized",
			setup: func(*testMocks) *Client {
				return &Client{cluster: nil}
			},
			logic: func(any) error {
				return nil
			},
			wantErr: errClustertNotInitialized,
		},
		{
			name: "error: transaction fails",
			setup: func(mocks *testMocks) *Client {
				mocks.cluster.EXPECT().Transactions().Return(mocks.transactions)
				mocks.transactions.EXPECT().Run(gomock.Any(), gomock.Any()).Return(nil, errMockTransaction)
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any()).AnyTimes()
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				return &Client{
					cluster: mocks.cluster,
					config:  &Config{Bucket: "bucket"},
					logger:  mocks.logger,
					metrics: mocks.metrics,
					tracer:  noop.NewTracerProvider().Tracer("test"),
				}
			},
			logic: func(any) error {
				return errLogic
			},
			wantErr: errMockTransaction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)

			_, err := client.RunTransaction(t.Context(), tt.logic)
			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_Query(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		params    map[string]any
		result    any
		setup     func(mocks *testMocks) *Client
		wantErr   error
	}{
		{
			name:      "success: N1QL query",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(mocks.queryResult, nil),
					mocks.queryResult.EXPECT().Next().Return(true),
					mocks.queryResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					mocks.queryResult.EXPECT().Next().Return(false),
					mocks.queryResult.EXPECT().Err().Return(nil),
					mocks.queryResult.EXPECT().Close().Return(nil))
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name:      "error: from cluster.Query",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setup: func(mocks *testMocks) *Client {
				mocks.cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, gocb.ErrPlanningFailure)
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrPlanningFailure,
		},
		{
			name:      "error: failed to unmarshal N1QL results into target",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &struct{}{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.cluster.EXPECT().Query(gomock.Any(), gomock.Any()).Return(mocks.queryResult, nil),
					mocks.queryResult.EXPECT().Next().Return(true),
					mocks.queryResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					mocks.queryResult.EXPECT().Next().Return(false),
					mocks.queryResult.EXPECT().Err().Return(nil),
					mocks.queryResult.EXPECT().Close().Return(nil))
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: errFailedToUnmarshalN1QL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			err := client.Query(t.Context(), tt.statement, tt.params, tt.result)

			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
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
	provider := noop.NewTracerProvider()
	tracer := provider.Tracer("test")
	client.UseTracer(tracer)
	assert.Equal(t, tracer, client.tracer)
}

func TestClient_Close(t *testing.T) {
	mocks := newTestMocks(t)
	mocks.cluster.EXPECT().Close(&gocb.ClusterCloseOptions{}).Return(nil)

	client := &Client{
		cluster: mocks.cluster,
	}

	err := client.Close(&gocb.ClusterCloseOptions{})
	require.NoError(t, err)
}

func TestClient_HealthCheck(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(mocks *testMocks)
		wantStatus string
		wantErr    error
	}{
		{
			name: "success: cluster is up",
			setupMocks: func(mocks *testMocks) {
				mocks.cluster.EXPECT().Ping(nil).Return(&gocb.PingResult{}, nil)
			},
			wantStatus: "UP",
			wantErr:    nil,
		},
		{
			name: "error: cluster is down",
			setupMocks: func(mocks *testMocks) {
				mocks.cluster.EXPECT().Ping(nil).Return(nil, gocb.ErrUnambiguousTimeout)
			},
			wantStatus: "DOWN",
			wantErr:    errStatusDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			tt.setupMocks(mocks)

			client := &Client{
				cluster: mocks.cluster,
				config: &Config{
					Host:   "localhost",
					Bucket: "gofr",
				},
			}

			health, err := client.HealthCheck(t.Context())

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
			c := &Client{config: tt.config}
			got, err := c.generateCouchbaseURI()

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
	tests := []struct {
		name      string
		statement string
		params    map[string]any
		result    any
		setup     func(mocks *testMocks) *Client
		wantErr   error
	}{
		{
			name:      "success: Analytics query",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(mocks.queryResult, nil),
					mocks.queryResult.EXPECT().Next().Return(true),
					mocks.queryResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test_analytics"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					mocks.queryResult.EXPECT().Next().Return(false),
					mocks.queryResult.EXPECT().Err().Return(nil),
					mocks.queryResult.EXPECT().Close().Return(nil))
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
		},
		{
			name:      "error: failed to unmarshal Analytics query row",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &[]map[string]any{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(mocks.queryResult, nil),
					mocks.queryResult.EXPECT().Next().Return(true),
					mocks.queryResult.EXPECT().Row(gomock.Any()).Return(gocb.ErrDecodingFailure),
					mocks.queryResult.EXPECT().Close().Return(nil))
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: gocb.ErrDecodingFailure,
		},
		{
			name:      "error: failed to unmarshal analytics results into target",
			statement: "SELECT * FROM `bucket`",
			params:    nil,
			result:    &struct{}{},
			setup: func(mocks *testMocks) *Client {
				gomock.InOrder(mocks.cluster.EXPECT().AnalyticsQuery(gomock.Any(), gomock.Any()).Return(mocks.queryResult, nil),
					mocks.queryResult.EXPECT().Next().Return(true),
					mocks.queryResult.EXPECT().Row(gomock.Any()).DoAndReturn(func(arg any) error {
						data := `{"id": "1", "name": "test_analytics"}`
						return json.Unmarshal([]byte(data), arg)
					}),
					mocks.queryResult.EXPECT().Next().Return(false),
					mocks.queryResult.EXPECT().Err().Return(nil),
					mocks.queryResult.EXPECT().Close().Return(nil))
				mocks.metrics.EXPECT().RecordHistogram(gomock.Any(), "app_couchbase_stats", gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Debug(gomock.Any())
				mocks.logger.EXPECT().Logf(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mocks.logger.EXPECT().Errorf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				return &Client{
					cluster: mocks.cluster, config: &Config{}, logger: mocks.logger, metrics: mocks.metrics,
				}
			},
			wantErr: errFailedToUnmarshalAnalytics,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := newTestMocks(t)
			client := tt.setup(mocks)
			err := client.AnalyticsQuery(t.Context(), tt.statement, tt.params, tt.result)

			if tt.wantErr != nil {
				assert.ErrorContains(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
