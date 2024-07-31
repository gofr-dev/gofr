// Code generated by MockGen. DO NOT EDIT.
// Source: interfaces.go
//
// Generated by this command:
//
//	mockgen -source=interfaces.go -destination=mock_interfaces.go -package=cassandra
//

// Package cassandra is a generated GoMock package.
package cassandra

import (
	reflect "reflect"

	gocql "github.com/gocql/gocql"
	gomock "go.uber.org/mock/gomock"
)

// MockclusterConfig is a mock of clusterConfig interface.
type MockclusterConfig struct {
	ctrl     *gomock.Controller
	recorder *MockclusterConfigMockRecorder
}

// MockclusterConfigMockRecorder is the mock recorder for MockclusterConfig.
type MockclusterConfigMockRecorder struct {
	mock *MockclusterConfig
}

// NewMockclusterConfig creates a new mock instance.
func NewMockclusterConfig(ctrl *gomock.Controller) *MockclusterConfig {
	mock := &MockclusterConfig{ctrl: ctrl}
	mock.recorder = &MockclusterConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockclusterConfig) EXPECT() *MockclusterConfigMockRecorder {
	return m.recorder
}

// createSession mocks base method.
func (m *MockclusterConfig) createSession() (session, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "createSession")
	ret0, _ := ret[0].(session)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// createSession indicates an expected call of createSession.
func (mr *MockclusterConfigMockRecorder) createSession() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "createSession", reflect.TypeOf((*MockclusterConfig)(nil).createSession))
}

// Mocksession is a mock of session interface.
type Mocksession struct {
	ctrl     *gomock.Controller
	recorder *MocksessionMockRecorder
}

// MocksessionMockRecorder is the mock recorder for Mocksession.
type MocksessionMockRecorder struct {
	mock *Mocksession
}

// NewMocksession creates a new mock instance.
func NewMocksession(ctrl *gomock.Controller) *Mocksession {
	mock := &Mocksession{ctrl: ctrl}
	mock.recorder = &MocksessionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mocksession) EXPECT() *MocksessionMockRecorder {
	return m.recorder
}

// executeBatch mocks base method.
func (m *Mocksession) executeBatch(batch batch) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "executeBatch", batch)
	ret0, _ := ret[0].(error)
	return ret0
}

// executeBatch indicates an expected call of executeBatch.
func (mr *MocksessionMockRecorder) executeBatch(batch any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "executeBatch", reflect.TypeOf((*Mocksession)(nil).executeBatch), batch)
}

// executeBatchCAS mocks base method.
func (m *Mocksession) executeBatchCAS(b batch) (bool, iterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "executeBatchCAS", b)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(iterator)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// executeBatchCAS indicates an expected call of executeBatchCAS.
func (mr *MocksessionMockRecorder) executeBatchCAS(b any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "executeBatchCAS", reflect.TypeOf((*Mocksession)(nil).executeBatchCAS), b)
}

// newBatch mocks base method.
func (m *Mocksession) newBatch(batchtype gocql.BatchType) batch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "newBatch", batchtype)
	ret0, _ := ret[0].(batch)
	return ret0
}

// newBatch indicates an expected call of newBatch.
func (mr *MocksessionMockRecorder) newBatch(batchtype any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "newBatch", reflect.TypeOf((*Mocksession)(nil).newBatch), batchtype)
}

// query mocks base method.
func (m *Mocksession) query(stmt string, values ...any) query {
	m.ctrl.T.Helper()
	varargs := []any{stmt}
	for _, a := range values {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "query", varargs...)
	ret0, _ := ret[0].(query)
	return ret0
}

// query indicates an expected call of query.
func (mr *MocksessionMockRecorder) query(stmt any, values ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{stmt}, values...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "query", reflect.TypeOf((*Mocksession)(nil).query), varargs...)
}

// Mockquery is a mock of query interface.
type Mockquery struct {
	ctrl     *gomock.Controller
	recorder *MockqueryMockRecorder
}

// MockqueryMockRecorder is the mock recorder for Mockquery.
type MockqueryMockRecorder struct {
	mock *Mockquery
}

// NewMockquery creates a new mock instance.
func NewMockquery(ctrl *gomock.Controller) *Mockquery {
	mock := &Mockquery{ctrl: ctrl}
	mock.recorder = &MockqueryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockquery) EXPECT() *MockqueryMockRecorder {
	return m.recorder
}

// exec mocks base method.
func (m *Mockquery) exec() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "exec")
	ret0, _ := ret[0].(error)
	return ret0
}

// exec indicates an expected call of exec.
func (mr *MockqueryMockRecorder) exec() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "exec", reflect.TypeOf((*Mockquery)(nil).exec))
}

// iter mocks base method.
func (m *Mockquery) iter() iterator {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "iter")
	ret0, _ := ret[0].(iterator)
	return ret0
}

// iter indicates an expected call of iter.
func (mr *MockqueryMockRecorder) iter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "iter", reflect.TypeOf((*Mockquery)(nil).iter))
}

// mapScanCAS mocks base method.
func (m *Mockquery) mapScanCAS(dest map[string]any) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "mapScanCAS", dest)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// mapScanCAS indicates an expected call of mapScanCAS.
func (mr *MockqueryMockRecorder) mapScanCAS(dest any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "mapScanCAS", reflect.TypeOf((*Mockquery)(nil).mapScanCAS), dest)
}

// scanCAS mocks base method.
func (m *Mockquery) scanCAS(dest ...any) (bool, error) {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range dest {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "scanCAS", varargs...)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// scanCAS indicates an expected call of scanCAS.
func (mr *MockqueryMockRecorder) scanCAS(dest ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "scanCAS", reflect.TypeOf((*Mockquery)(nil).scanCAS), dest...)
}

// Mockbatch is a mock of batch interface.
type Mockbatch struct {
	ctrl     *gomock.Controller
	recorder *MockbatchMockRecorder
}

// MockbatchMockRecorder is the mock recorder for Mockbatch.
type MockbatchMockRecorder struct {
	mock *Mockbatch
}

// NewMockbatch creates a new mock instance.
func NewMockbatch(ctrl *gomock.Controller) *Mockbatch {
	mock := &Mockbatch{ctrl: ctrl}
	mock.recorder = &MockbatchMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockbatch) EXPECT() *MockbatchMockRecorder {
	return m.recorder
}

// Query mocks base method.
func (m *Mockbatch) Query(stmt string, args ...any) {
	m.ctrl.T.Helper()
	varargs := []any{stmt}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Query", varargs...)
}

// Query indicates an expected call of Query.
func (mr *MockbatchMockRecorder) Query(stmt any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{stmt}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*Mockbatch)(nil).Query), varargs...)
}

// getBatch mocks base method.
func (m *Mockbatch) getBatch() *gocql.Batch {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "getBatch")
	ret0, _ := ret[0].(*gocql.Batch)
	return ret0
}

// getBatch indicates an expected call of getBatch.
func (mr *MockbatchMockRecorder) getBatch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "getBatch", reflect.TypeOf((*Mockbatch)(nil).getBatch))
}

// Mockiterator is a mock of iterator interface.
type Mockiterator struct {
	ctrl     *gomock.Controller
	recorder *MockiteratorMockRecorder
}

// MockiteratorMockRecorder is the mock recorder for Mockiterator.
type MockiteratorMockRecorder struct {
	mock *Mockiterator
}

// NewMockiterator creates a new mock instance.
func NewMockiterator(ctrl *gomock.Controller) *Mockiterator {
	mock := &Mockiterator{ctrl: ctrl}
	mock.recorder = &MockiteratorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockiterator) EXPECT() *MockiteratorMockRecorder {
	return m.recorder
}

// columns mocks base method.
func (m *Mockiterator) columns() []gocql.ColumnInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "columns")
	ret0, _ := ret[0].([]gocql.ColumnInfo)
	return ret0
}

// columns indicates an expected call of columns.
func (mr *MockiteratorMockRecorder) columns() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "columns", reflect.TypeOf((*Mockiterator)(nil).columns))
}

// numRows mocks base method.
func (m *Mockiterator) numRows() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "numRows")
	ret0, _ := ret[0].(int)
	return ret0
}

// numRows indicates an expected call of numRows.
func (mr *MockiteratorMockRecorder) numRows() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "numRows", reflect.TypeOf((*Mockiterator)(nil).numRows))
}

// scan mocks base method.
func (m *Mockiterator) scan(dest ...any) bool {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range dest {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "scan", varargs...)
	ret0, _ := ret[0].(bool)
	return ret0
}

// scan indicates an expected call of scan.
func (mr *MockiteratorMockRecorder) scan(dest ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "scan", reflect.TypeOf((*Mockiterator)(nil).scan), dest...)
}
