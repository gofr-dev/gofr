package scylladb

import (
	"reflect"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/mock/gomock"
)

type casRow struct {
	ID   int
	Name string `db:"user_name"`
}

// closedGocqlSession returns a gocql session that is already closed, so every operation on it
// fails fast with gocql.ErrSessionClosed without needing a running cluster.
func closedGocqlSession() *scyllaSession {
	sess := &gocql.Session{}
	sess.Close()

	return &scyllaSession{session: sess}
}

func Test_ScyllaSessionClosedOperations(t *testing.T) {
	tests := []struct {
		desc      string
		operation func(s *scyllaSession) error
		expErr    error
	}{
		{
			desc:      "query exec",
			operation: func(s *scyllaSession) error { return s.Query("INSERT INTO users (id) VALUES (?)", 1).Exec() },
			expErr:    gocql.ErrSessionClosed,
		},
		{
			desc: "query map scan cas",
			operation: func(s *scyllaSession) error {
				_, err := s.Query("UPDATE users SET name = ? WHERE id = ? IF EXISTS", "a", 1).MapScanCAS(map[string]any{})

				return err
			},
			expErr: gocql.ErrSessionClosed,
		},
		{
			desc: "query scan cas",
			operation: func(s *scyllaSession) error {
				var name string

				_, err := s.Query("UPDATE users SET name = ? WHERE id = ? IF EXISTS", "a", 1).ScanCAS(&name)

				return err
			},
			expErr: gocql.ErrSessionClosed,
		},
		{
			desc: "execute batch",
			operation: func(s *scyllaSession) error {
				b := s.newBatch(gocql.LoggedBatch)
				b.Query("INSERT INTO users (id) VALUES (?)", 1)

				return s.executeBatch(b)
			},
			expErr: gocql.ErrSessionClosed,
		},
		{
			desc: "execute batch cas",
			operation: func(s *scyllaSession) error {
				b := s.newBatch(gocql.LoggedBatch)
				b.Query("INSERT INTO users (id) VALUES (?) IF NOT EXISTS", 1)

				_, err := s.executeBatchCAS(b)

				return err
			},
			expErr: gocql.ErrSessionClosed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.operation(closedGocqlSession())

			require.ErrorIs(t, err, tc.expErr)
		})
	}
}

func Test_ScyllaIteratorOnClosedSession(t *testing.T) {
	tests := []struct {
		desc       string
		stmt       string
		expColumns []gocql.ColumnInfo
		expScan    bool
		expNumRows int
	}{
		{
			desc: "iterator of failed query has no rows",
			stmt: "SELECT id FROM users",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var id int

			iter := closedGocqlSession().Query(tc.stmt).Iter()

			assert.Equal(t, tc.expColumns, iter.Columns())
			assert.Equal(t, tc.expScan, iter.Scan(&id))
			assert.Equal(t, tc.expNumRows, iter.NumRows())
		})
	}
}

func Test_ScyllaBatchQuery(t *testing.T) {
	tests := []struct {
		desc    string
		stmts   []string
		expSize int
	}{
		{desc: "single statement", stmts: []string{"INSERT INTO users (id) VALUES (?)"}, expSize: 1},
		{desc: "multiple statements", stmts: []string{"INSERT INTO users (id) VALUES (?)", "DELETE FROM users WHERE id = ?"},
			expSize: 2},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			b := closedGocqlSession().newBatch(gocql.UnloggedBatch)

			for _, stmt := range tc.stmts {
				b.Query(stmt, 1)
			}

			assert.Equal(t, tc.expSize, b.getBatch().Size())
			assert.Equal(t, gocql.UnloggedBatch, b.getBatch().Type)
		})
	}
}

func Test_ScyllaClusterConfigCreateSessionError(t *testing.T) {
	tests := []struct {
		desc   string
		hosts  []string
		expErr error
	}{
		{desc: "no hosts configured", hosts: nil, expErr: gocql.ErrNoHosts},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := &scyllaClusterConfig{clusterConfig: gocql.NewCluster(tc.hosts...)}

			sess, err := cfg.createSession()

			require.ErrorIs(t, err, tc.expErr)
			assert.Nil(t, sess)
		})
	}
}

func Test_ClientGetFields(t *testing.T) {
	tests := []struct {
		desc     string
		columns  []string
		expTypes []any
	}{
		{
			desc:     "all columns map to struct fields",
			columns:  []string{"id", "user_name"},
			expTypes: []any{new(int), new(string)},
		},
		{
			desc:     "unknown column gets a generic placeholder",
			columns:  []string{"id", "unknown"},
			expTypes: []any{new(int), new(any)},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var row casRow

			v := reflect.ValueOf(&row).Elem()
			c := Client{}

			fields := c.getFields(tc.columns, c.getFieldNameIndex(v), v)

			require.Len(t, fields, len(tc.expTypes))
			assert.Same(t, &row.ID, fields[0])

			for i, expType := range tc.expTypes {
				assert.IsType(t, expType, fields[i])
			}
		})
	}
}

func Test_ClientTraceAndOperationStats(t *testing.T) {
	tests := []struct {
		desc   string
		method string
		query  string
	}{
		{desc: "query with tracer", method: "query", query: "SELECT * FROM users"},
		{desc: "exec with tracer", method: "exec", query: "INSERT INTO users (id) VALUES (1)"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockLogger := NewMockLogger(ctrl)
			mockMetrics := NewMockMetrics(ctrl)

			client := New(Config{Host: "host1", Keyspace: "my_keyspace"})
			client.UseLogger(mockLogger)
			client.UseMetrics(mockMetrics)
			client.UseTracer(noop.NewTracerProvider().Tracer("test"))
			client.scylla.query = NewMockquery(ctrl)

			ql := &QueryLog{Operation: tc.method, Query: tc.query, Keyspace: "my_keyspace"}

			mockLogger.EXPECT().Debug(ql)
			mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_scylla_stats", gomock.Any(), "hostname", "host1",
				"keyspace", "my_keyspace")

			span := client.addTrace(t.Context(), tc.method, tc.query)
			require.NotNil(t, span)

			// A start time one second in the past must be reflected in the recorded duration (microseconds).
			client.sendOperationStats(ql, time.Now().Add(-time.Second), tc.method, span)

			assert.Nil(t, client.scylla.query)
			assert.GreaterOrEqual(t, ql.Duration, time.Second.Microseconds())
		})
	}
}

func Test_ClientRowsToStructCAS(t *testing.T) {
	tests := []struct {
		desc       string
		row        map[string]any
		scanErr    error
		expApplied bool
		expRow     casRow
		expErr     error
	}{
		{
			desc:       "matching columns are set",
			row:        map[string]any{"id": 1, "user_name": "alice"},
			expApplied: true,
			expRow:     casRow{ID: 1, Name: "alice"},
		},
		{
			desc:       "mismatched types and unknown columns are ignored",
			row:        map[string]any{"id": "one", "unknown": 5, "user_name": "bob"},
			expApplied: false,
			expRow:     casRow{Name: "bob"},
		},
		{
			desc:    "map scan cas error",
			scanErr: errMock,
			expErr:  errMock,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockQuery := NewMockquery(ctrl)

			mockQuery.EXPECT().MapScanCAS(gomock.Any()).DoAndReturn(func(dest map[string]any) (bool, error) {
				for k, v := range tc.row {
					dest[k] = v
				}

				return tc.expApplied, tc.scanErr
			})

			var row casRow

			client := New(Config{})

			applied, err := client.rowsToStructCAS(mockQuery, reflect.ValueOf(&row))

			require.ErrorIs(t, err, tc.expErr)
			assert.Equal(t, tc.expApplied, applied)
			assert.Equal(t, tc.expRow, row)
		})
	}
}
