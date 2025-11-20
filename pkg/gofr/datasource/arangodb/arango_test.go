package arangodb

import (
	"context"
	"errors"
	"testing"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/arangodb/go-driver/v2/arangodb/shared"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"
)

var (
	errUserNotFound     = errors.New("user not found")
	errDBNotFound       = errors.New("database not found")
	errDocumentNotFound = errors.New("document not found")
)

func setupDB(t *testing.T) (*Client, *MockClient, *MockUser, *MockLogger, *MockMetrics) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup
	mockLogger := NewMockLogger(ctrl)
	mockMetrics := NewMockMetrics(ctrl)
	mockArango := NewMockClient(ctrl)
	mockUser := NewMockUser(ctrl)

	mockLogger.EXPECT().Debugf(gomock.Any(), gomock.Any()).AnyTimes()

	config := Config{Host: "localhost", Port: 8527, User: "root", Password: "root"}
	client := New(config)
	client.UseLogger(mockLogger)
	client.UseMetrics(mockMetrics)
	client.UseTracer(otel.GetTracerProvider().Tracer("gofr-arangodb"))

	client.client = mockArango

	return client, mockArango, mockUser, mockLogger, mockMetrics
}

func Test_NewArangoClient_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	metrics := NewMockMetrics(ctrl)
	logger := NewMockLogger(ctrl)

	logger.EXPECT().Errorf("failed to verify connection: %v", gomock.Any())
	logger.EXPECT().Debugf(gomock.Any(), gomock.Any())

	client := New(Config{Host: "localhost", Port: 8529, Password: "root", User: "admin"})

	client.UseLogger(logger)
	client.UseMetrics(metrics)
	client.Connect()

	require.NotNil(t, client)
}

func TestClient_Query_Success(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	dbName := "testDB"
	query := "FOR doc IN collection RETURN doc"
	bindVars := map[string]any{"key": "value"}

	var result []map[string]any

	expectedResult := []map[string]any{
		{"_key": "doc1", "value": "test1"},
		{"_key": "doc2", "value": "test2"},
	}

	test.MockArango.EXPECT().GetDatabase(test.Ctx, dbName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().Query(test.Ctx, query, &arangodb.QueryOptions{BindVars: bindVars}).
		Return(NewMockQueryCursor(test.Ctrl, expectedResult), nil)

	err := test.Client.Query(test.Ctx, dbName, query, bindVars, &result)

	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid config",
			config: Config{
				Host:     "localhost",
				Port:     8529,
				User:     "root",
				Password: "password",
			},
			expectErr: false,
		},
		{
			name: "Empty host",
			config: Config{
				Port:     8529,
				User:     "root",
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: host is empty",
		},
		{
			name: "Empty port",
			config: Config{
				Host:     "localhost",
				User:     "root",
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: port is empty",
		},
		{
			name: "Empty user",
			config: Config{
				Host:     "localhost",
				Port:     8529,
				Password: "password",
			},
			expectErr: true,
			errMsg:    "missing required field in config: user is empty",
		},
		{
			name: "Empty password",
			config: Config{
				Host: "localhost",
				Port: 8529,
				User: "root",
			},
			expectErr: true,
			errMsg:    "missing required field in config: password is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &Client{config: &tc.config}
			err := client.validateConfig()

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_HealthCheck_Success(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	expectedVersion := arangodb.VersionInfo{
		Version: "3.9.0",
		Server:  "arango",
	}

	test.MockArango.EXPECT().Version(test.Ctx).Return(expectedVersion, nil)

	health, err := test.Client.HealthCheck(test.Ctx)

	require.NoError(t, err)

	h, ok := health.(*Health)
	require.True(t, ok)

	require.Equal(t, "UP", h.Status)
	require.Equal(t, test.Client.endpoint, h.Details["endpoint"])
	require.Equal(t, expectedVersion.Version, h.Details["version"])
	require.Equal(t, expectedVersion.Server, h.Details["server"])
}

func TestClient_HealthCheck_Error(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	test.MockArango.EXPECT().Version(test.Ctx).Return(arangodb.VersionInfo{}, errStatusDown)

	health, err := test.Client.HealthCheck(test.Ctx)

	require.Error(t, err)
	require.Equal(t, errStatusDown, err)

	h, ok := health.(*Health)
	require.True(t, ok)

	require.Equal(t, "DOWN", h.Status)
	require.Equal(t, test.Client.endpoint, h.Details["endpoint"])
}

type MockQueryCursor struct {
	ctrl *gomock.Controller
	data []map[string]any
	idx  int
}

func NewMockQueryCursor(ctrl *gomock.Controller, data []map[string]any) *MockQueryCursor {
	return &MockQueryCursor{
		ctrl: ctrl,
		data: data,
		idx:  0,
	}
}

func (*MockQueryCursor) Close() error {
	return nil
}

func (*MockQueryCursor) CloseWithContext(_ context.Context) error {
	return nil
}

func (m *MockQueryCursor) HasMore() bool {
	return m.idx < len(m.data)
}

func (m *MockQueryCursor) ReadDocument(_ context.Context, document any) (arangodb.DocumentMeta, error) {
	if m.idx >= len(m.data) {
		return arangodb.DocumentMeta{}, shared.NoMoreDocumentsError{}
	}

	doc, ok := document.(*map[string]any)
	if !ok {
		return arangodb.DocumentMeta{}, errInvalidEdgeDocumentType
	}

	*doc = m.data[m.idx]
	meta := arangodb.DocumentMeta{}

	m.idx++

	return meta, nil
}

func (m *MockQueryCursor) Count() int64 {
	return int64(len(m.data))
}

func (*MockQueryCursor) Statistics() arangodb.CursorStats {
	return arangodb.CursorStats{}
}

func (*MockQueryCursor) Plan() arangodb.CursorPlan {
	return arangodb.CursorPlan{}
}

func TestClient_Query_WithBatchSizeAndFullCount(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	dbName := "testDB"
	query := "FOR doc IN collection RETURN doc"
	bindVars := map[string]any{"key": "value"}

	var result []map[string]any

	expectedResult := []map[string]any{
		{"_key": "doc1", "value": "v1"},
		{"_key": "doc2", "value": "v2"},
	}

	// Define QueryOptions with batchSize and fullCount
	queryOpts := map[string]any{
		"batchSize": 50,
		"options": map[string]any{
			"fullCount": true,
		},
	}

	test.MockArango.EXPECT().GetDatabase(test.Ctx, dbName, nil).
		Return(test.MockDB, nil)

	test.MockDB.EXPECT().
		Query(test.Ctx, query, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, opts *arangodb.QueryOptions) (arangodb.Cursor, error) {
			require.NotNil(t, opts)
			require.Equal(t, 50, opts.BatchSize)
			require.True(t, opts.Options.FullCount)
			require.Equal(t, bindVars, opts.BindVars)

			return NewMockQueryCursor(test.Ctrl, expectedResult), nil
		})

	err := test.Client.Query(test.Ctx, dbName, query, bindVars, &result, queryOpts)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestClient_Query_WithMaxPlans(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	dbName := "testDB"
	query := "FOR doc IN collection RETURN doc"
	bindVars := map[string]any{"key": "value"}

	var result []map[string]any

	expectedResult := []map[string]any{
		{"_key": "doc1", "value": "v1"},
	}

	// Define QueryOptions with maxPlans sub-option
	queryOpts := map[string]any{
		"options": map[string]any{
			"maxPlans": 5,
		},
	}

	test.MockArango.EXPECT().GetDatabase(test.Ctx, dbName, nil).
		Return(test.MockDB, nil)

	test.MockDB.EXPECT().
		Query(test.Ctx, query, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, opts *arangodb.QueryOptions) (arangodb.Cursor, error) {
			require.NotNil(t, opts)
			require.Equal(t, 5, opts.Options.MaxPlans)

			return NewMockQueryCursor(test.Ctrl, expectedResult), nil
		})

	err := test.Client.Query(test.Ctx, dbName, query, bindVars, &result, queryOpts)
	require.NoError(t, err)
	require.Equal(t, expectedResult, result)
}

func TestClient_Query_InvalidResultType(t *testing.T) {
	test := setupGraphTest(t)
	defer test.Ctrl.Finish()

	dbName := "testDB"
	query := "FOR doc IN collection RETURN doc"
	bindVars := map[string]any{"key": "value"}

	var result int // Incorrect type

	test.MockArango.EXPECT().GetDatabase(test.Ctx, dbName, nil).
		Return(test.MockDB, nil)
	test.MockDB.EXPECT().Query(test.Ctx, query, gomock.Any()).Return(NewMockQueryCursor(test.Ctrl, nil), nil)

	err := test.Client.Query(test.Ctx, dbName, query, bindVars, &result)
	require.Error(t, err)
	require.Equal(t, errInvalidResultType, err)
}
