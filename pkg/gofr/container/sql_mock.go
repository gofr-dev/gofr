package container

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/logging"
)

type health datasource.Health

type dialect string

// expectedQuery stores the mock expectations till the method call.
type expectedQuery struct {
	queryWithArgs       []queryWithArgs
	expectedDialect     []dialect
	expectedHealthCheck []health
}

type queryWithArgs struct {
	queryText string
	arguments []interface{}
}

// mockSQL wraps go-mock-sql and expectations.
type mockSQL struct {
	sqlmock.Sqlmock
	*expectedQuery
}

// sqlMockDB wraps the go-mock-sql DB connection and expectations.
type sqlMockDB struct {
	*gofrSQL.DB
	*expectedQuery
	logger logging.Logger
}

func (m sqlMockDB) Select(_ context.Context, _ interface{}, query string, args ...interface{}) {
	if m.queryWithArgs == nil || len(m.queryWithArgs) == 0 {
		m.logger.Fatalf("Did not expect any calls for Select with query: %s", query)
	}

	lastIndex := len(m.queryWithArgs) - 1
	expectedText := m.queryWithArgs[lastIndex].queryText
	expectedArgs := m.queryWithArgs[lastIndex].arguments

	if expectedText != query {
		m.logger.Fatalf("expected query: %s, actual query: %s", query, expectedText)
	}

	if len(args) != len(expectedArgs) {
		m.logger.Fatalf("expected %d args, actual %d", len(expectedArgs), len(args))
	}

	for i := range args {
		if args[i] != expectedArgs[i] {
			m.logger.Fatalf("expected arg %d, actual arg %d", args[i], expectedArgs[i])
		}
	}

	m.queryWithArgs = m.queryWithArgs[:lastIndex]
}

func (m sqlMockDB) HealthCheck() *datasource.Health {
	if m.expectedHealthCheck == nil || len(m.expectedHealthCheck) == 0 {
		m.logger.Fatal("Did not expect any mock calls for HealthCheck")
	}

	lastIndex := len(m.expectedHealthCheck) - 1
	expectedString := m.expectedHealthCheck[lastIndex]
	d := datasource.Health(expectedString)

	m.expectedHealthCheck = m.expectedHealthCheck[:lastIndex]

	return &d
}

func (m sqlMockDB) Dialect() string {
	if m.expectedDialect == nil || len(m.expectedDialect) == 0 {
		m.logger.Fatal("Did not expect any mock calls for Dialect")
	}

	lastIndex := len(m.expectedDialect) - 1
	expectedString := m.expectedDialect[lastIndex]

	m.expectedDialect = m.expectedDialect[:lastIndex]

	return string(expectedString)
}

func (m sqlMockDB) finish(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		require.Empty(t, m.queryWithArgs, "Expected mock call to Select")
		require.Empty(t, m.expectedDialect, "Expected mock call to Dialect")
		require.Empty(t, m.expectedHealthCheck, "Expected mock call to HealthCheck")
	})
}

// ExpectSelect is not a direct method for mocking the Select method of SQL in go-mock-sql.
// Hence, it expects the user to already provide the populated data interface field,
// which can then be used within the functions implemented by the user.
func (m *mockSQL) ExpectSelect(_ context.Context, _ interface{}, query string, args ...interface{}) {
	sliceQueryWithArgs := make([]queryWithArgs, 0)

	if m.queryWithArgs == nil {
		m.queryWithArgs = sliceQueryWithArgs
	}

	qr := queryWithArgs{query, args}

	sliceQueryWithArgs = append(sliceQueryWithArgs, qr)
	m.queryWithArgs = append(sliceQueryWithArgs, m.queryWithArgs...)
}

func (m *mockSQL) ExpectHealthCheck() *health {
	healthCheckSlice := make([]health, 0)

	if m.expectedHealthCheck == nil {
		m.expectedHealthCheck = healthCheckSlice
	}

	hc := health{}

	healthCheckSlice = append(healthCheckSlice, hc)
	m.expectedHealthCheck = append(healthCheckSlice, m.expectedHealthCheck...)

	return &m.expectedHealthCheck[0]
}

func (d *health) WillReturnHealthCheck(dh *datasource.Health) {
	*d = health(*dh)
}

func (m *mockSQL) ExpectDialect() *dialect {
	dialectSlice := make([]dialect, 0)

	if m.expectedDialect == nil {
		m.expectedDialect = dialectSlice
	}

	d := dialect("")

	dialectSlice = append(dialectSlice, d)
	m.expectedDialect = append(dialectSlice, m.expectedDialect...)

	return &m.expectedDialect[0]
}

func (*mockSQL) NewResult(lastInsertID, rowsAffected int64) sql.Result {
	return sqlmock.NewResult(lastInsertID, rowsAffected)
}

func (d *dialect) WillReturnString(s string) {
	*d = dialect(s)
}
