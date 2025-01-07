package container

import (
	"context"
	"database/sql"
	"reflect"
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
	value     any
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

func (m sqlMockDB) Select(_ context.Context, value any, query string, args ...interface{}) {
	if len(m.queryWithArgs) == 0 {
		m.logger.Errorf("did not expect any calls for Select with query: %q", query)
		return
	}

	lastIndex := len(m.queryWithArgs) - 1

	defer func() {
		m.queryWithArgs = m.queryWithArgs[:lastIndex]
	}()

	expectedText := m.queryWithArgs[lastIndex].queryText
	expectedArgs := m.queryWithArgs[lastIndex].arguments

	valueType := reflect.TypeOf(value)
	if valueType.Kind() != reflect.Ptr {
		m.logger.Errorf("expected a pointer type: %q", value)
		return
	}

	if m.queryWithArgs[lastIndex].value == nil {
		m.logger.Errorf("received different expectations: %q", query)
		return
	}

	v := reflect.ValueOf(value)

	if v.Kind() == reflect.Ptr && !v.IsNil() {
		tobechanged := v.Elem()
		tobechanged.Set(reflect.ValueOf(m.queryWithArgs[lastIndex].value))
	}

	if expectedText != query {
		m.logger.Errorf("expected query: %q, actual query: %q", query, expectedText)
		return
	}

	if len(args) != len(expectedArgs) {
		m.logger.Errorf("expected %d args, actual %d", len(expectedArgs), len(args))
		return
	}

	for i := range args {
		if args[i] != expectedArgs[i] {
			m.logger.Errorf("expected arg %d, actual arg %d", args[i], expectedArgs[i])
			return
		}
	}
}

func (m sqlMockDB) HealthCheck() *datasource.Health {
	if len(m.expectedHealthCheck) == 0 {
		m.logger.Error("Did not expect any mock calls for HealthCheck")
		return nil
	}

	lastIndex := len(m.expectedHealthCheck) - 1
	expectedString := m.expectedHealthCheck[lastIndex]
	d := datasource.Health(expectedString)

	m.expectedHealthCheck = m.expectedHealthCheck[:lastIndex]

	return &d
}

func (m sqlMockDB) Dialect() string {
	if len(m.expectedDialect) == 0 {
		m.logger.Error("Did not expect any mock calls for Dialect")
		return ""
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
func (m *mockSQL) ExpectSelect(_ context.Context, value any, query string, args ...interface{}) *queryWithArgs {
	sliceQueryWithArgs := make([]queryWithArgs, 0)

	if m.queryWithArgs == nil {
		m.queryWithArgs = sliceQueryWithArgs
	}

	qr := queryWithArgs{queryText: query, arguments: args}

	fieldType := reflect.TypeOf(value)
	if fieldType.Kind() == reflect.Ptr {
		qr.value = value
	}

	sliceQueryWithArgs = append(sliceQueryWithArgs, qr)
	m.queryWithArgs = append(sliceQueryWithArgs, m.queryWithArgs...)

	return &m.queryWithArgs[0]
}

func (q *queryWithArgs) ReturnsResponse(value any) {
	fieldType := reflect.TypeOf(q.value)
	if fieldType == nil {
		return
	}

	valueType := reflect.TypeOf(value)

	fieldType = fieldType.Elem()

	q.value = nil
	if fieldType == valueType {
		q.value = value
	}
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
