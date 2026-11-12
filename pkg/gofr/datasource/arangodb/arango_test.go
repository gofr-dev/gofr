package arangodb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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
	errConnRefused      = errors.New("connection refused")
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

	test.MockArango.EXPECT().Version(test.Ctx).Return(arangodb.VersionInfo{}, errConnRefused)

	health, err := test.Client.HealthCheck(test.Ctx)

	require.ErrorIs(t, err, errStatusDown)

	h, ok := health.(*Health)
	require.True(t, ok)

	require.Equal(t, statusDown, h.Status)
	require.Equal(t, test.Client.endpoint, h.Details["endpoint"])
	require.Equal(t, errConnRefused.Error(), h.Details["error"])
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

// errReadCursor is a query cursor whose reads always fail.
type errReadCursor struct {
	*MockQueryCursor
}

func (errReadCursor) ReadDocument(context.Context, any) (arangodb.DocumentMeta, error) {
	return arangodb.DocumentMeta{}, errDocumentNotFound
}

func TestClient_Query_Errors(t *testing.T) {
	const query = "FOR doc IN collection RETURN doc"

	tests := []struct {
		desc       string
		options    []map[string]any
		setupMocks func(m *arangoMocks)
		expErr     string
	}{
		{
			desc: "database lookup fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(nil, errDBNotFound)
			},
			expErr: errDBNotFound.Error(),
		},
		{
			desc:    "options cannot be marshaled",
			options: []map[string]any{{"batchSize": func() {}}},
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
			},
			expErr: "unsupported type",
		},
		{
			desc:    "options do not match query options",
			options: []map[string]any{{"batchSize": "fifty"}},
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
			},
			expErr: "cannot unmarshal",
		},
		{
			desc: "query fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().Query(gomock.Any(), query, gomock.Any()).Return(nil, errStatusDown)
			},
			expErr: errStatusDown.Error(),
		},
		{
			desc: "reading a document fails",
			setupMocks: func(m *arangoMocks) {
				m.arango.EXPECT().GetDatabase(gomock.Any(), "testDB", nil).Return(m.db, nil)
				m.db.EXPECT().Query(gomock.Any(), query, gomock.Any()).
					Return(errReadCursor{NewMockQueryCursor(nil, nil)}, nil)
			},
			expErr: errDocumentNotFound.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			client, m := newArangoTestClient(t)
			tc.setupMocks(m)

			var result []map[string]any

			err := client.Query(t.Context(), "testDB", query, nil, &result, tc.options...)

			require.ErrorContains(t, err, tc.expErr)
			require.Empty(t, result)
		})
	}
}

func TestClient_Connect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"server":"arango","version":"3.11.0"}`))
	}))
	t.Cleanup(srv.Close)

	downSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(downSrv.Close)

	host, port := hostPort(t, srv.URL)
	downHost, downPort := hostPort(t, downSrv.URL)

	tests := []struct {
		desc         string
		config       Config
		setupMocks   func(l *MockLogger, m *MockMetrics)
		expEndpoint  string
		expClientSet bool
	}{
		{
			desc:   "invalid config registers metrics and leaves client nil",
			config: Config{Host: "localhost"},
			setupMocks: func(l *MockLogger, m *MockMetrics) {
				m.EXPECT().NewHistogram("app_arango_stats", gomock.Any(), gomock.Any())
				l.EXPECT().Errorf("config validation error: %v", gomock.Any())
			},
		},
		{
			desc:   "server unreachable still registers metrics",
			config: Config{Host: downHost, Port: downPort, User: "root", Password: "root"},
			setupMocks: func(l *MockLogger, m *MockMetrics) {
				m.EXPECT().NewHistogram("app_arango_stats", gomock.Any(), gomock.Any())
				l.EXPECT().Debugf("connecting to ArangoDB at %s", "http://"+downSrv.Listener.Addr().String())
				l.EXPECT().Errorf("failed to verify connection: %v", gomock.Any())
			},
			expEndpoint:  "http://" + downSrv.Listener.Addr().String(),
			expClientSet: true,
		},
		{
			desc:   "connects and registers metrics",
			config: Config{Host: host, Port: port, User: "root", Password: "root"},
			setupMocks: func(l *MockLogger, m *MockMetrics) {
				m.EXPECT().NewHistogram("app_arango_stats", gomock.Any(), gomock.Any())
				l.EXPECT().Debugf("connecting to ArangoDB at %s", "http://"+srv.Listener.Addr().String())
				l.EXPECT().Logf("Connected to ArangoDB successfully at %s", "http://"+srv.Listener.Addr().String())
			},
			expEndpoint:  "http://" + srv.Listener.Addr().String(),
			expClientSet: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			tc.setupMocks(mockLogger, mockMetrics)

			client := New(tc.config)
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			client.Connect()

			require.Equal(t, tc.expEndpoint, client.endpoint)
			require.Equal(t, tc.expClientSet, client.client != nil)
		})
	}
}

func hostPort(t *testing.T, rawURL string) (host string, port int) {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	port, err = strconv.Atoi(u.Port())
	require.NoError(t, err)

	return u.Hostname(), port
}

func TestClient_NotConnected(t *testing.T) {
	tests := []struct {
		desc string
		call func(ctx context.Context, c *Client) error
	}{
		{"Query", func(ctx context.Context, c *Client) error {
			var res []map[string]any
			return c.Query(ctx, "db", "RETURN 1", nil, &res)
		}},
		{"CreateDB", func(ctx context.Context, c *Client) error { return c.CreateDB(ctx, "db") }},
		{"DropDB", func(ctx context.Context, c *Client) error { return c.DropDB(ctx, "db") }},
		{"CreateCollection", func(ctx context.Context, c *Client) error { return c.CreateCollection(ctx, "db", "col", false) }},
		{"DropCollection", func(ctx context.Context, c *Client) error { return c.DropCollection(ctx, "db", "col") }},
		{"CreateGraph", func(ctx context.Context, c *Client) error { return c.CreateGraph(ctx, "db", "g", &EdgeDefinition{}) }},
		{"DropGraph", func(ctx context.Context, c *Client) error { return c.DropGraph(ctx, "db", "g") }},
		{"GetEdges", func(ctx context.Context, c *Client) error {
			return c.GetEdges(ctx, "db", "g", "edges", "persons/1", &EdgeDetails{})
		}},
		{"CreateDocument", func(ctx context.Context, c *Client) error {
			_, err := c.CreateDocument(ctx, "db", "col", map[string]any{})
			return err
		}},
		{"GetDocument", func(ctx context.Context, c *Client) error {
			return c.GetDocument(ctx, "db", "col", "id", &map[string]any{})
		}},
		{"UpdateDocument", func(ctx context.Context, c *Client) error {
			return c.UpdateDocument(ctx, "db", "col", "id", map[string]any{})
		}},
		{"DeleteDocument", func(ctx context.Context, c *Client) error { return c.DeleteDocument(ctx, "db", "col", "id") }},
		{"createUser", func(ctx context.Context, c *Client) error { return c.createUser(ctx, "u", UserOptions{}) }},
		{"dropUser", func(ctx context.Context, c *Client) error { return c.dropUser(ctx, "u") }},
		{"grantDB", func(ctx context.Context, c *Client) error { return c.grantDB(ctx, "db", "u", "rw") }},
		{"grantCollection", func(ctx context.Context, c *Client) error {
			return c.grantCollection(ctx, "db", "col", "u", "rw")
		}},
		{"user", func(ctx context.Context, c *Client) error {
			_, err := c.user(ctx, "u")
			return err
		}},
		{"database", func(ctx context.Context, c *Client) error {
			_, err := c.database(ctx, "db")
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_arango_stats", gomock.Any(),
				gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			client := New(Config{})
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)

			err := tc.call(t.Context(), client)

			require.ErrorIs(t, err, errNotConnected)
		})
	}
}

func TestClient_HealthCheck_NotConnected(t *testing.T) {
	client := New(Config{})

	health, err := client.HealthCheck(t.Context())

	require.ErrorIs(t, err, errNotConnected)
	require.Equal(t, &Health{Status: statusDown, Details: map[string]any{
		"endpoint": "",
		"error":    errNotConnected.Error(),
	}}, health)
}
