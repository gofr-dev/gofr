// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go
//
// Generated by this command:
//
//	mockgen -source=interface.go -destination=mock_interface.go -package=migration
//

// Package migration is a generated GoMock package.
package migration

import (
	context "context"
	sql "database/sql"
	reflect "reflect"
	time "time"

	redis "github.com/redis/go-redis/v9"
	gomock "go.uber.org/mock/gomock"
	container "gofr.dev/pkg/gofr/container"
)

// MockRedis is a mock of Redis interface.
type MockRedis struct {
	ctrl     *gomock.Controller
	recorder *MockRedisMockRecorder
}

// MockRedisMockRecorder is the mock recorder for MockRedis.
type MockRedisMockRecorder struct {
	mock *MockRedis
}

// NewMockRedis creates a new mock instance.
func NewMockRedis(ctrl *gomock.Controller) *MockRedis {
	mock := &MockRedis{ctrl: ctrl}
	mock.recorder = &MockRedisMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRedis) EXPECT() *MockRedisMockRecorder {
	return m.recorder
}

// Del mocks base method.
func (m *MockRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.ctrl.T.Helper()
	varargs := []any{ctx}
	for _, a := range keys {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Del", varargs...)
	ret0, _ := ret[0].(*redis.IntCmd)
	return ret0
}

// Del indicates an expected call of Del.
func (mr *MockRedisMockRecorder) Del(ctx any, keys ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx}, keys...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Del", reflect.TypeOf((*MockRedis)(nil).Del), varargs...)
}

// Get mocks base method.
func (m *MockRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, key)
	ret0, _ := ret[0].(*redis.StringCmd)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *MockRedisMockRecorder) Get(ctx, key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockRedis)(nil).Get), ctx, key)
}

// Rename mocks base method.
func (m *MockRedis) Rename(ctx context.Context, key, newKey string) *redis.StatusCmd {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rename", ctx, key, newKey)
	ret0, _ := ret[0].(*redis.StatusCmd)
	return ret0
}

// Rename indicates an expected call of Rename.
func (mr *MockRedisMockRecorder) Rename(ctx, key, newKey any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rename", reflect.TypeOf((*MockRedis)(nil).Rename), ctx, key, newKey)
}

// Set mocks base method.
func (m *MockRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Set", ctx, key, value, expiration)
	ret0, _ := ret[0].(*redis.StatusCmd)
	return ret0
}

// Set indicates an expected call of Set.
func (mr *MockRedisMockRecorder) Set(ctx, key, value, expiration any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Set", reflect.TypeOf((*MockRedis)(nil).Set), ctx, key, value, expiration)
}

// MockSQL is a mock of SQL interface.
type MockSQL struct {
	ctrl     *gomock.Controller
	recorder *MockSQLMockRecorder
}

// MockSQLMockRecorder is the mock recorder for MockSQL.
type MockSQLMockRecorder struct {
	mock *MockSQL
}

// NewMockSQL creates a new mock instance.
func NewMockSQL(ctrl *gomock.Controller) *MockSQL {
	mock := &MockSQL{ctrl: ctrl}
	mock.recorder = &MockSQLMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSQL) EXPECT() *MockSQLMockRecorder {
	return m.recorder
}

// Exec mocks base method.
func (m *MockSQL) Exec(query string, args ...any) (sql.Result, error) {
	m.ctrl.T.Helper()
	varargs := []any{query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Exec", varargs...)
	ret0, _ := ret[0].(sql.Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Exec indicates an expected call of Exec.
func (mr *MockSQLMockRecorder) Exec(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockSQL)(nil).Exec), varargs...)
}

// ExecContext mocks base method.
func (m *MockSQL) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "ExecContext", varargs...)
	ret0, _ := ret[0].(sql.Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ExecContext indicates an expected call of ExecContext.
func (mr *MockSQLMockRecorder) ExecContext(ctx, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecContext", reflect.TypeOf((*MockSQL)(nil).ExecContext), varargs...)
}

// Query mocks base method.
func (m *MockSQL) Query(query string, args ...any) (*sql.Rows, error) {
	m.ctrl.T.Helper()
	varargs := []any{query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Query", varargs...)
	ret0, _ := ret[0].(*sql.Rows)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Query indicates an expected call of Query.
func (mr *MockSQLMockRecorder) Query(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockSQL)(nil).Query), varargs...)
}

// QueryRow mocks base method.
func (m *MockSQL) QueryRow(query string, args ...any) *sql.Row {
	m.ctrl.T.Helper()
	varargs := []any{query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "QueryRow", varargs...)
	ret0, _ := ret[0].(*sql.Row)
	return ret0
}

// QueryRow indicates an expected call of QueryRow.
func (mr *MockSQLMockRecorder) QueryRow(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryRow", reflect.TypeOf((*MockSQL)(nil).QueryRow), varargs...)
}

// QueryRowContext mocks base method.
func (m *MockSQL) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	m.ctrl.T.Helper()
	varargs := []any{ctx, query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "QueryRowContext", varargs...)
	ret0, _ := ret[0].(*sql.Row)
	return ret0
}

// QueryRowContext indicates an expected call of QueryRowContext.
func (mr *MockSQLMockRecorder) QueryRowContext(ctx, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryRowContext", reflect.TypeOf((*MockSQL)(nil).QueryRowContext), varargs...)
}

// MockPubSub is a mock of PubSub interface.
type MockPubSub struct {
	ctrl     *gomock.Controller
	recorder *MockPubSubMockRecorder
}

// MockPubSubMockRecorder is the mock recorder for MockPubSub.
type MockPubSubMockRecorder struct {
	mock *MockPubSub
}

// NewMockPubSub creates a new mock instance.
func NewMockPubSub(ctrl *gomock.Controller) *MockPubSub {
	mock := &MockPubSub{ctrl: ctrl}
	mock.recorder = &MockPubSubMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPubSub) EXPECT() *MockPubSubMockRecorder {
	return m.recorder
}

// CreateTopic mocks base method.
func (m *MockPubSub) CreateTopic(context context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTopic", context, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateTopic indicates an expected call of CreateTopic.
func (mr *MockPubSubMockRecorder) CreateTopic(context, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTopic", reflect.TypeOf((*MockPubSub)(nil).CreateTopic), context, name)
}

// DeleteTopic mocks base method.
func (m *MockPubSub) DeleteTopic(context context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteTopic", context, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteTopic indicates an expected call of DeleteTopic.
func (mr *MockPubSubMockRecorder) DeleteTopic(context, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteTopic", reflect.TypeOf((*MockPubSub)(nil).DeleteTopic), context, name)
}

// MockClickhouse is a mock of Clickhouse interface.
type MockClickhouse struct {
	ctrl     *gomock.Controller
	recorder *MockClickhouseMockRecorder
}

// MockClickhouseMockRecorder is the mock recorder for MockClickhouse.
type MockClickhouseMockRecorder struct {
	mock *MockClickhouse
}

// NewMockClickhouse creates a new mock instance.
func NewMockClickhouse(ctrl *gomock.Controller) *MockClickhouse {
	mock := &MockClickhouse{ctrl: ctrl}
	mock.recorder = &MockClickhouseMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClickhouse) EXPECT() *MockClickhouseMockRecorder {
	return m.recorder
}

// AsyncInsert mocks base method.
func (m *MockClickhouse) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, query, wait}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "AsyncInsert", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// AsyncInsert indicates an expected call of AsyncInsert.
func (mr *MockClickhouseMockRecorder) AsyncInsert(ctx, query, wait any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query, wait}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AsyncInsert", reflect.TypeOf((*MockClickhouse)(nil).AsyncInsert), varargs...)
}

// Exec mocks base method.
func (m *MockClickhouse) Exec(ctx context.Context, query string, args ...any) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Exec", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Exec indicates an expected call of Exec.
func (mr *MockClickhouseMockRecorder) Exec(ctx, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockClickhouse)(nil).Exec), varargs...)
}

// HealthCheck mocks base method.
func (m *MockClickhouse) HealthCheck() any {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck")
	ret0, _ := ret[0].(any)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockClickhouseMockRecorder) HealthCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockClickhouse)(nil).HealthCheck))
}

// Select mocks base method.
func (m *MockClickhouse) Select(ctx context.Context, dest any, query string, args ...any) error {
	m.ctrl.T.Helper()
	varargs := []any{ctx, dest, query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Select", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Select indicates an expected call of Select.
func (mr *MockClickhouseMockRecorder) Select(ctx, dest, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, dest, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Select", reflect.TypeOf((*MockClickhouse)(nil).Select), varargs...)
}

// Mockmigrator is a mock of migrator interface.
type Mockmigrator struct {
	ctrl     *gomock.Controller
	recorder *MockmigratorMockRecorder
}

// MockmigratorMockRecorder is the mock recorder for Mockmigrator.
type MockmigratorMockRecorder struct {
	mock *Mockmigrator
}

// NewMockmigrator creates a new mock instance.
func NewMockmigrator(ctrl *gomock.Controller) *Mockmigrator {
	mock := &Mockmigrator{ctrl: ctrl}
	mock.recorder = &MockmigratorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockmigrator) EXPECT() *MockmigratorMockRecorder {
	return m.recorder
}

// beginTransaction mocks base method.
func (m *Mockmigrator) beginTransaction(c *container.Container) transactionData {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "beginTransaction", c)
	ret0, _ := ret[0].(transactionData)
	return ret0
}

// beginTransaction indicates an expected call of beginTransaction.
func (mr *MockmigratorMockRecorder) beginTransaction(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "beginTransaction", reflect.TypeOf((*Mockmigrator)(nil).beginTransaction), c)
}

// checkAndCreateMigrationTable mocks base method.
func (m *Mockmigrator) checkAndCreateMigrationTable(c *container.Container) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "checkAndCreateMigrationTable", c)
	ret0, _ := ret[0].(error)
	return ret0
}

// checkAndCreateMigrationTable indicates an expected call of checkAndCreateMigrationTable.
func (mr *MockmigratorMockRecorder) checkAndCreateMigrationTable(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "checkAndCreateMigrationTable", reflect.TypeOf((*Mockmigrator)(nil).checkAndCreateMigrationTable), c)
}

// commitMigration mocks base method.
func (m *Mockmigrator) commitMigration(c *container.Container, data transactionData) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "commitMigration", c, data)
	ret0, _ := ret[0].(error)
	return ret0
}

// commitMigration indicates an expected call of commitMigration.
func (mr *MockmigratorMockRecorder) commitMigration(c, data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "commitMigration", reflect.TypeOf((*Mockmigrator)(nil).commitMigration), c, data)
}

// getLastMigration mocks base method.
func (m *Mockmigrator) getLastMigration(c *container.Container) int64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getLastMigration", c)
	ret0, _ := ret[0].(int64)
	return ret0
}

// getLastMigration indicates an expected call of getLastMigration.
func (mr *MockmigratorMockRecorder) getLastMigration(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getLastMigration", reflect.TypeOf((*Mockmigrator)(nil).getLastMigration), c)
}

// rollback mocks base method.
func (m *Mockmigrator) rollback(c *container.Container, data transactionData) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "rollback", c, data)
}

// rollback indicates an expected call of rollback.
func (mr *MockmigratorMockRecorder) rollback(c, data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "rollback", reflect.TypeOf((*Mockmigrator)(nil).rollback), c, data)
}
