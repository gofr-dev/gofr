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
func (m *MockClickhouse) HealthCheck(ctx context.Context) (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck", ctx)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockClickhouseMockRecorder) HealthCheck(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockClickhouse)(nil).HealthCheck), ctx)
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

// MockCassandra is a mock of Cassandra interface.
type MockCassandra struct {
	ctrl     *gomock.Controller
	recorder *MockCassandraMockRecorder
}

// MockCassandraMockRecorder is the mock recorder for MockCassandra.
type MockCassandraMockRecorder struct {
	mock *MockCassandra
}

// NewMockCassandra creates a new mock instance.
func NewMockCassandra(ctrl *gomock.Controller) *MockCassandra {
	mock := &MockCassandra{ctrl: ctrl}
	mock.recorder = &MockCassandraMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCassandra) EXPECT() *MockCassandraMockRecorder {
	return m.recorder
}

// BatchQuery mocks base method.
func (m *MockCassandra) BatchQuery(name, stmt string, values ...any) error {
	m.ctrl.T.Helper()
	varargs := []any{name, stmt}
	for _, a := range values {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "BatchQuery", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// BatchQuery indicates an expected call of BatchQuery.
func (mr *MockCassandraMockRecorder) BatchQuery(name, stmt any, values ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{name, stmt}, values...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BatchQuery", reflect.TypeOf((*MockCassandra)(nil).BatchQuery), varargs...)
}

// Exec mocks base method.
func (m *MockCassandra) Exec(query string, args ...any) error {
	m.ctrl.T.Helper()
	varargs := []any{query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Exec", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Exec indicates an expected call of Exec.
func (mr *MockCassandraMockRecorder) Exec(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockCassandra)(nil).Exec), varargs...)
}

// ExecuteBatch mocks base method.
func (m *MockCassandra) ExecuteBatch(name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExecuteBatch", name)
	ret0, _ := ret[0].(error)
	return ret0
}

// ExecuteBatch indicates an expected call of ExecuteBatch.
func (mr *MockCassandraMockRecorder) ExecuteBatch(name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecuteBatch", reflect.TypeOf((*MockCassandra)(nil).ExecuteBatch), name)
}

// HealthCheck mocks base method.
func (m *MockCassandra) HealthCheck(ctx context.Context) (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck", ctx)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockCassandraMockRecorder) HealthCheck(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockCassandra)(nil).HealthCheck), ctx)
}

// NewBatch mocks base method.
func (m *MockCassandra) NewBatch(name string, batchType int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewBatch", name, batchType)
	ret0, _ := ret[0].(error)
	return ret0
}

// NewBatch indicates an expected call of NewBatch.
func (mr *MockCassandraMockRecorder) NewBatch(name, batchType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewBatch", reflect.TypeOf((*MockCassandra)(nil).NewBatch), name, batchType)
}

// MockMongo is a mock of Mongo interface.
type MockMongo struct {
	ctrl     *gomock.Controller
	recorder *MockMongoMockRecorder
}

// MockMongoMockRecorder is the mock recorder for MockMongo.
type MockMongoMockRecorder struct {
	mock *MockMongo
}

// NewMockMongo creates a new mock instance.
func NewMockMongo(ctrl *gomock.Controller) *MockMongo {
	mock := &MockMongo{ctrl: ctrl}
	mock.recorder = &MockMongoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMongo) EXPECT() *MockMongoMockRecorder {
	return m.recorder
}

// CreateCollection mocks base method.
func (m *MockMongo) CreateCollection(ctx context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateCollection", ctx, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateCollection indicates an expected call of CreateCollection.
func (mr *MockMongoMockRecorder) CreateCollection(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCollection", reflect.TypeOf((*MockMongo)(nil).CreateCollection), ctx, name)
}

// DeleteMany mocks base method.
func (m *MockMongo) DeleteMany(ctx context.Context, collection string, filter any) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteMany", ctx, collection, filter)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteMany indicates an expected call of DeleteMany.
func (mr *MockMongoMockRecorder) DeleteMany(ctx, collection, filter any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteMany", reflect.TypeOf((*MockMongo)(nil).DeleteMany), ctx, collection, filter)
}

// DeleteOne mocks base method.
func (m *MockMongo) DeleteOne(ctx context.Context, collection string, filter any) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteOne", ctx, collection, filter)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteOne indicates an expected call of DeleteOne.
func (mr *MockMongoMockRecorder) DeleteOne(ctx, collection, filter any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteOne", reflect.TypeOf((*MockMongo)(nil).DeleteOne), ctx, collection, filter)
}

// Drop mocks base method.
func (m *MockMongo) Drop(ctx context.Context, collection string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Drop", ctx, collection)
	ret0, _ := ret[0].(error)
	return ret0
}

// Drop indicates an expected call of Drop.
func (mr *MockMongoMockRecorder) Drop(ctx, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Drop", reflect.TypeOf((*MockMongo)(nil).Drop), ctx, collection)
}

// Find mocks base method.
func (m *MockMongo) Find(ctx context.Context, collection string, filter, results any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Find", ctx, collection, filter, results)
	ret0, _ := ret[0].(error)
	return ret0
}

// Find indicates an expected call of Find.
func (mr *MockMongoMockRecorder) Find(ctx, collection, filter, results any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Find", reflect.TypeOf((*MockMongo)(nil).Find), ctx, collection, filter, results)
}

// FindOne mocks base method.
func (m *MockMongo) FindOne(ctx context.Context, collection string, filter, result any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindOne", ctx, collection, filter, result)
	ret0, _ := ret[0].(error)
	return ret0
}

// FindOne indicates an expected call of FindOne.
func (mr *MockMongoMockRecorder) FindOne(ctx, collection, filter, result any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindOne", reflect.TypeOf((*MockMongo)(nil).FindOne), ctx, collection, filter, result)
}

// InsertMany mocks base method.
func (m *MockMongo) InsertMany(ctx context.Context, collection string, documents []any) ([]any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InsertMany", ctx, collection, documents)
	ret0, _ := ret[0].([]any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InsertMany indicates an expected call of InsertMany.
func (mr *MockMongoMockRecorder) InsertMany(ctx, collection, documents any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InsertMany", reflect.TypeOf((*MockMongo)(nil).InsertMany), ctx, collection, documents)
}

// InsertOne mocks base method.
func (m *MockMongo) InsertOne(ctx context.Context, collection string, document any) (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InsertOne", ctx, collection, document)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InsertOne indicates an expected call of InsertOne.
func (mr *MockMongoMockRecorder) InsertOne(ctx, collection, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InsertOne", reflect.TypeOf((*MockMongo)(nil).InsertOne), ctx, collection, document)
}

// StartSession mocks base method.
func (m *MockMongo) StartSession() (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartSession")
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartSession indicates an expected call of StartSession.
func (mr *MockMongoMockRecorder) StartSession() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartSession", reflect.TypeOf((*MockMongo)(nil).StartSession))
}

// UpdateByID mocks base method.
func (m *MockMongo) UpdateByID(ctx context.Context, collection string, id, update any) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateByID", ctx, collection, id, update)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateByID indicates an expected call of UpdateByID.
func (mr *MockMongoMockRecorder) UpdateByID(ctx, collection, id, update any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateByID", reflect.TypeOf((*MockMongo)(nil).UpdateByID), ctx, collection, id, update)
}

// UpdateMany mocks base method.
func (m *MockMongo) UpdateMany(ctx context.Context, collection string, filter, update any) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateMany", ctx, collection, filter, update)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateMany indicates an expected call of UpdateMany.
func (mr *MockMongoMockRecorder) UpdateMany(ctx, collection, filter, update any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMany", reflect.TypeOf((*MockMongo)(nil).UpdateMany), ctx, collection, filter, update)
}

// UpdateOne mocks base method.
func (m *MockMongo) UpdateOne(ctx context.Context, collection string, filter, update any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateOne", ctx, collection, filter, update)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateOne indicates an expected call of UpdateOne.
func (mr *MockMongoMockRecorder) UpdateOne(ctx, collection, filter, update any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateOne", reflect.TypeOf((*MockMongo)(nil).UpdateOne), ctx, collection, filter, update)
}

// MockArangoDB is a mock of ArangoDB interface.
type MockArangoDB struct {
	ctrl     *gomock.Controller
	recorder *MockArangoDBMockRecorder
}

// MockArangoDBMockRecorder is the mock recorder for MockArangoDB.
type MockArangoDBMockRecorder struct {
	mock *MockArangoDB
}

// NewMockArangoDB creates a new mock instance.
func NewMockArangoDB(ctrl *gomock.Controller) *MockArangoDB {
	mock := &MockArangoDB{ctrl: ctrl}
	mock.recorder = &MockArangoDBMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockArangoDB) EXPECT() *MockArangoDBMockRecorder {
	return m.recorder
}

// CreateCollection mocks base method.
func (m *MockArangoDB) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateCollection", ctx, database, collection, isEdge)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateCollection indicates an expected call of CreateCollection.
func (mr *MockArangoDBMockRecorder) CreateCollection(ctx, database, collection, isEdge any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCollection", reflect.TypeOf((*MockArangoDB)(nil).CreateCollection), ctx, database, collection, isEdge)
}

// CreateDB mocks base method.
func (m *MockArangoDB) CreateDB(ctx context.Context, database string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDB", ctx, database)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateDB indicates an expected call of CreateDB.
func (mr *MockArangoDBMockRecorder) CreateDB(ctx, database any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDB", reflect.TypeOf((*MockArangoDB)(nil).CreateDB), ctx, database)
}

// CreateGraph mocks base method.
func (m *MockArangoDB) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGraph", ctx, database, graph, edgeDefinitions)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateGraph indicates an expected call of CreateGraph.
func (mr *MockArangoDBMockRecorder) CreateGraph(ctx, database, graph, edgeDefinitions any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGraph", reflect.TypeOf((*MockArangoDB)(nil).CreateGraph), ctx, database, graph, edgeDefinitions)
}

// DropCollection mocks base method.
func (m *MockArangoDB) DropCollection(ctx context.Context, database, collection string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropCollection", ctx, database, collection)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropCollection indicates an expected call of DropCollection.
func (mr *MockArangoDBMockRecorder) DropCollection(ctx, database, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropCollection", reflect.TypeOf((*MockArangoDB)(nil).DropCollection), ctx, database, collection)
}

// DropDB mocks base method.
func (m *MockArangoDB) DropDB(ctx context.Context, database string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropDB", ctx, database)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropDB indicates an expected call of DropDB.
func (mr *MockArangoDBMockRecorder) DropDB(ctx, database any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropDB", reflect.TypeOf((*MockArangoDB)(nil).DropDB), ctx, database)
}

// DropGraph mocks base method.
func (m *MockArangoDB) DropGraph(ctx context.Context, database, graph string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropGraph", ctx, database, graph)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropGraph indicates an expected call of DropGraph.
func (mr *MockArangoDBMockRecorder) DropGraph(ctx, database, graph any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropGraph", reflect.TypeOf((*MockArangoDB)(nil).DropGraph), ctx, database, graph)
}

// Exists mocks base method.
func (m *MockArangoDB) Exists(ctx context.Context, name, resourceType string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Exists", ctx, name, resourceType)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Exists indicates an expected call of Exists.
func (mr *MockArangoDBMockRecorder) Exists(ctx, name, resourceType any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exists", reflect.TypeOf((*MockArangoDB)(nil).Exists), ctx, name, resourceType)
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
