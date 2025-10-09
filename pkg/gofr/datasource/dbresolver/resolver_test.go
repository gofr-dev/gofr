package dbresolver

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

var errTestReplicaFailed = errors.New("replica connection failed")

const (
	healthStatusUP   = "UP"
	healthStatusDOWN = "DOWN"
)

// MockResult is a mock of sql.Result interface.
type MockResult struct {
	ctrl     *gomock.Controller
	recorder *MockResultMockRecorder
}

// MockResultMockRecorder is the mock recorder for MockResult.
type MockResultMockRecorder struct {
	mock *MockResult
}

// NewMockResult creates a new mock instance.
func NewMockResult(ctrl *gomock.Controller) *MockResult {
	mock := &MockResult{ctrl: ctrl}
	mock.recorder = &MockResultMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResult) EXPECT() *MockResultMockRecorder {
	return m.recorder
}

// LastInsertId mocks base method.
func (m *MockResult) LastInsertId() (int64, error) {
	ret := m.ctrl.Call(m, "LastInsertId")

	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// LastInsertId indicates an expected call of LastInsertId.
func (mr *MockResultMockRecorder) LastInsertId() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LastInsertId", reflect.TypeOf((*MockResult)(nil).LastInsertId))
}

// RowsAffected mocks base method.
func (m *MockResult) RowsAffected() (int64, error) {
	ret := m.ctrl.Call(m, "RowsAffected")
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// RowsAffected indicates an expected call of RowsAffected.
func (mr *MockResultMockRecorder) RowsAffected() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RowsAffected", reflect.TypeOf((*MockResult)(nil).RowsAffected))
}

func TestResolver_Query_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := mocks.Resolver.Query(readQuery)

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_Query_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	writeQuery := "INSERT INTO users (name) VALUES (?)"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryContext(gomock.Any(), writeQuery, args[0]).Return(expectedRows, nil)

	rows, err := mocks.Resolver.Query(writeQuery, args[0])

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_Query_FallbackToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := mocks.Resolver.Query(readQuery)

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_QueryContext_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(t.Context(), readQuery)

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_QueryContext_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	writeQuery := "INSERT INTO users (name) VALUES (?)"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryContext(gomock.Any(), writeQuery, args[0]).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(t.Context(), writeQuery, args[0])

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_QueryRow_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	readQuery := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().QueryRowContext(gomock.Any(), readQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRow(readQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_QueryRow_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	writeQuery := "INSERT INTO users (name) VALUES (?) RETURNING id"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryRowContext(gomock.Any(), writeQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRow(writeQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_QueryRowContext_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	readQuery := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().QueryRowContext(gomock.Any(), readQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRowContext(t.Context(), readQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_QueryRowContext_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRow := &sql.Row{}
	writeQuery := "INSERT INTO users (name) VALUES (?) RETURNING id"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().QueryRowContext(gomock.Any(), writeQuery, args[0]).Return(expectedRow)

	row := mocks.Resolver.QueryRowContext(t.Context(), writeQuery, args[0])

	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
}

func TestResolver_ExecContext_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedResult := NewMockResult(mocks.Ctrl)
	writeQuery := "UPDATE users SET name = ? WHERE id = ?"
	args := []any{"new_name", 1}

	mocks.Primary.EXPECT().ExecContext(gomock.Any(), writeQuery, args[0], args[1]).Return(expectedResult, nil)

	result, err := mocks.Resolver.ExecContext(t.Context(), writeQuery, args[0], args[1])

	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestResolver_Exec_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedResult := NewMockResult(mocks.Ctrl)
	writeQuery := "UPDATE users SET name = ? WHERE id = ?"
	args := []any{"new_name", 1}

	mocks.Primary.EXPECT().ExecContext(gomock.Any(), writeQuery, args[0], args[1]).Return(expectedResult, nil)

	result, err := mocks.Resolver.Exec(writeQuery, args[0], args[1])

	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestResolver_Select_ReadGoesToReplica(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	data := &struct{ Name string }{}
	readQuery := "SELECT name FROM users WHERE id = ?"
	args := []any{1}

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().Select(gomock.Any(), data, readQuery, args[0])

	mocks.Resolver.Select(t.Context(), data, readQuery, args[0])
}

func TestResolver_Select_WriteGoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	data := &struct{ ID int64 }{}
	writeQuery := "INSERT INTO users (name) VALUES (?) RETURNING id"
	args := []any{"test_user"}

	mocks.Primary.EXPECT().Select(gomock.Any(), data, writeQuery, args[0])

	mocks.Resolver.Select(t.Context(), data, writeQuery, args[0])
}

func TestResolver_Prepare_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedStmt := &sql.Stmt{}
	query := "SELECT * FROM users WHERE id = ?"

	mocks.Primary.EXPECT().Prepare(query).Return(expectedStmt, nil)

	stmt, err := mocks.Resolver.Prepare(query)

	require.NoError(t, err)
	assert.Equal(t, expectedStmt, stmt)
}

func TestResolver_Begin_GoesToPrimary(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedTx := &gofrSQL.Tx{}

	mocks.Primary.EXPECT().Begin().Return(expectedTx, nil)

	tx, err := mocks.Resolver.Begin()

	require.NoError(t, err)
	assert.Equal(t, expectedTx, tx)
}

func TestResolver_Dialect(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedDialect := "mysql"
	mocks.Primary.EXPECT().Dialect().Return(expectedDialect)

	dialect := mocks.Resolver.Dialect()
	assert.Equal(t, expectedDialect, dialect)
}

func TestResolver_Close(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	// Test primary close
	mocks.Primary.EXPECT().Close().Return(nil)

	// Test replicas close
	for _, replica := range mocks.MockReplicas {
		replica.EXPECT().Close().Return(nil)
	}

	err := mocks.Resolver.Close()
	assert.NoError(t, err)
}

func TestResolver_Close_WithError(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	// Primary returns error
	mocks.Primary.EXPECT().Close().Return(errTestReplicaFailed)

	// Replicas don't error
	for _, replica := range mocks.MockReplicas {
		replica.EXPECT().Close().Return(nil)
	}

	err := mocks.Resolver.Close()
	assert.Equal(t, errTestReplicaFailed, err)
}

func TestResolver_QueryContext_WithFallback(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	expectedRows := &sql.Rows{}
	readQuery := "SELECT * FROM users"

	mocks.Strategy.EXPECT().Choose(gomock.Any()).Return(mocks.MockReplicas[0], nil)
	mocks.MockReplicas[0].EXPECT().QueryContext(gomock.Any(), readQuery).Return(nil, errTestReplicaFailed)
	mocks.Primary.EXPECT().QueryContext(gomock.Any(), readQuery).Return(expectedRows, nil)

	rows, err := mocks.Resolver.QueryContext(t.Context(), readQuery)

	require.NoError(t, err)
	assert.NoError(t, rows.Err())
	assert.Equal(t, expectedRows, rows)
}

func TestResolver_HealthCheck(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	primaryHealth := &datasource.Health{
		Status: healthStatusUP,
		Details: map[string]any{
			"connections": 5,
		},
	}

	replica1Health := &datasource.Health{
		Status: healthStatusUP,
		Details: map[string]any{
			"connections": 3,
		},
	}

	replica2Health := &datasource.Health{
		Status: healthStatusDOWN,
		Details: map[string]any{
			"error": "connection refused",
		},
	}

	mocks.Primary.EXPECT().HealthCheck().Return(primaryHealth)
	mocks.MockReplicas[0].EXPECT().HealthCheck().Return(replica1Health)
	mocks.MockReplicas[1].EXPECT().HealthCheck().Return(replica2Health)

	health := mocks.Resolver.HealthCheck()

	assert.Equal(t, healthStatusUP, health.Status)
	assert.Equal(t, primaryHealth, health.Details["primary"])

	replicas := health.Details["replicas"].([]any)
	assert.Len(t, replicas, 2)

	stats := health.Details["stats"].(map[string]any)
	assert.Contains(t, stats, "primaryReads")
	assert.Contains(t, stats, "replicaReads")
	assert.Contains(t, stats, "totalQueries")
}

func TestResolver_AddTrace(t *testing.T) {
	mocks := setupMocks(t, true)
	defer mocks.Ctrl.Finish()

	method := "query"
	query := "SELECT * FROM users"

	_, span := mocks.Resolver.addTrace(t.Context(), method, query)

	assert.NotNil(t, span)
}
