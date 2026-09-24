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
	arguments []any
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

func emptyExpectation(m *sqlMockDB) {
	if len(m.queryWithArgs) > 0 {
		m.queryWithArgs = m.queryWithArgs[1:]
	}
}

func (m sqlMockDB) Select(_ context.Context, value any, query string, args ...any) {
	if len(m.queryWithArgs) == 0 {
		m.logger.Errorf("did not expect any calls for Select with query: %q", query)
		return
	}

	defer emptyExpectation(&m)

	expected := m.queryWithArgs[0]

	if !m.matchesCall(expected, query, args) {
		return
	}

	m.assignResponse(expected.value, value, query)
}

// matchesCall reports whether the actual query and args match the expectation, reporting the first mismatch.
func (m sqlMockDB) matchesCall(expected queryWithArgs, query string, args []any) bool {
	if expected.queryText != query {
		m.logger.Errorf("expected query: %q, actual query: %q", expected.queryText, query)
		return false
	}

	if len(args) != len(expected.arguments) {
		m.logger.Errorf("expected %d args, actual %d", len(expected.arguments), len(args))
		return false
	}

	for i, arg := range args {
		if !reflect.DeepEqual(expected.arguments[i], arg) {
			m.logger.Errorf("expected arg %d: %v (%T), actual: %v (%T)", i, expected.arguments[i], expected.arguments[i], arg, arg)
			return false
		}
	}

	return true
}

// assignResponse copies the canned response into dest once the call has matched.
func (m sqlMockDB) assignResponse(response, dest any, query string) {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Pointer || destValue.IsNil() {
		m.logger.Errorf("expected a non-nil pointer, actual %T", dest)
		return
	}

	if response == nil {
		m.logger.Errorf("received different expectations: %q", query)
		return
	}

	responseValue := reflect.ValueOf(response)
	if !responseValue.Type().AssignableTo(destValue.Elem().Type()) {
		m.logger.Errorf("cannot assign response of type %T to destination of type %T", response, dest)
		return
	}

	destValue.Elem().Set(responseValue)
}

func (m sqlMockDB) HealthCheck() *datasource.Health {
	if len(m.expectedHealthCheck) == 0 {
		m.logger.Error("Did not expect any mock calls for HealthCheck")
		return nil
	}

	expectedString := m.expectedHealthCheck[0]
	d := datasource.Health(expectedString)

	if len(m.expectedHealthCheck) > 0 {
		m.expectedHealthCheck = m.expectedHealthCheck[1:]
	}

	return &d
}

func (m sqlMockDB) Dialect() string {
	if len(m.expectedDialect) == 0 {
		m.logger.Error("Did not expect any mock calls for Dialect")
		return ""
	}

	expectedString := m.expectedDialect[0]

	if len(m.expectedDialect) > 0 {
		m.expectedDialect = m.expectedDialect[1:]
	}

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
func (m *mockSQL) ExpectSelect(_ context.Context, value any, query string, args ...any) *queryWithArgs {
	qr := queryWithArgs{queryText: query, arguments: args}

	if reflect.ValueOf(value).Kind() == reflect.Pointer {
		qr.value = value
	}

	m.queryWithArgs = append(m.queryWithArgs, qr)

	return &m.queryWithArgs[len(m.queryWithArgs)-1]
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
	hc := health{}

	m.expectedHealthCheck = append(m.expectedHealthCheck, hc)

	return &m.expectedHealthCheck[len(m.expectedHealthCheck)-1]
}

func (d *health) WillReturnHealthCheck(dh *datasource.Health) {
	*d = health(*dh)
}

func (m *mockSQL) ExpectDialect() *dialect {
	d := dialect("")

	m.expectedDialect = append(m.expectedDialect, d)

	return &m.expectedDialect[len(m.expectedDialect)-1]
}

func (*mockSQL) NewResult(lastInsertID, rowsAffected int64) sql.Result {
	return sqlmock.NewResult(lastInsertID, rowsAffected)
}

func (d *dialect) WillReturnString(s string) {
	*d = dialect(s)
}
