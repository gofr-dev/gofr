// Code generated by MockGen. DO NOT EDIT.
// Source: datasources.go
//
// Generated by this command:
//
//	mockgen -source=datasources.go -package=container -destination=mock_datasources.go
//

// Package container is a generated GoMock package.
package container

import (
	context "context"
	sql "database/sql"
	driver "database/sql/driver"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	datasource "gofr.dev/pkg/gofr/datasource"
	sql0 "gofr.dev/pkg/gofr/datasource/sql"
)

// MockDBInterface is a mock of DBInterface interface.
type MockDBInterface struct {
	ctrl     *gomock.Controller
	recorder *MockDBInterfaceMockRecorder
}

// MockDBInterfaceMockRecorder is the mock recorder for MockDBInterface.
type MockDBInterfaceMockRecorder struct {
	mock *MockDBInterface
}

// NewMockDBInterface creates a new mock instance.
func NewMockDBInterface(ctrl *gomock.Controller) *MockDBInterface {
	mock := &MockDBInterface{ctrl: ctrl}
	mock.recorder = &MockDBInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDBInterface) EXPECT() *MockDBInterfaceMockRecorder {
	return m.recorder
}

// Begin mocks base method.
func (m *MockDBInterface) Begin() (*sql0.Tx, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Begin")
	ret0, _ := ret[0].(*sql0.Tx)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Begin indicates an expected call of Begin.
func (mr *MockDBInterfaceMockRecorder) Begin() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Begin", reflect.TypeOf((*MockDBInterface)(nil).Begin))
}

// Driver mocks base method.
func (m *MockDBInterface) Driver() driver.Driver {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Driver")
	ret0, _ := ret[0].(driver.Driver)
	return ret0
}

// Driver indicates an expected call of Driver.
func (mr *MockDBInterfaceMockRecorder) Driver() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Driver", reflect.TypeOf((*MockDBInterface)(nil).Driver))
}

// Exec mocks base method.
func (m *MockDBInterface) Exec(query string, args ...any) (sql.Result, error) {
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
func (mr *MockDBInterfaceMockRecorder) Exec(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Exec", reflect.TypeOf((*MockDBInterface)(nil).Exec), varargs...)
}

// ExecContext mocks base method.
func (m *MockDBInterface) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
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
func (mr *MockDBInterfaceMockRecorder) ExecContext(ctx, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExecContext", reflect.TypeOf((*MockDBInterface)(nil).ExecContext), varargs...)
}

// HealthCheck mocks base method.
func (m *MockDBInterface) HealthCheck() *datasource.Health {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck")
	ret0, _ := ret[0].(*datasource.Health)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockDBInterfaceMockRecorder) HealthCheck() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockDBInterface)(nil).HealthCheck))
}

// Prepare mocks base method.
func (m *MockDBInterface) Prepare(query string) (*sql.Stmt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Prepare", query)
	ret0, _ := ret[0].(*sql.Stmt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Prepare indicates an expected call of Prepare.
func (mr *MockDBInterfaceMockRecorder) Prepare(query any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Prepare", reflect.TypeOf((*MockDBInterface)(nil).Prepare), query)
}

// Query mocks base method.
func (m *MockDBInterface) Query(query string, args ...any) (*sql.Rows, error) {
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
func (mr *MockDBInterfaceMockRecorder) Query(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockDBInterface)(nil).Query), varargs...)
}

// QueryContext mocks base method.
func (m *MockDBInterface) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "QueryContext", varargs...)
	ret0, _ := ret[0].(*sql.Rows)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryContext indicates an expected call of QueryContext.
func (mr *MockDBInterfaceMockRecorder) QueryContext(ctx, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryContext", reflect.TypeOf((*MockDBInterface)(nil).QueryContext), varargs...)
}

// QueryRow mocks base method.
func (m *MockDBInterface) QueryRow(query string, args ...any) *sql.Row {
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
func (mr *MockDBInterfaceMockRecorder) QueryRow(query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryRow", reflect.TypeOf((*MockDBInterface)(nil).QueryRow), varargs...)
}

// QueryRowContext mocks base method.
func (m *MockDBInterface) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
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
func (mr *MockDBInterfaceMockRecorder) QueryRowContext(ctx, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryRowContext", reflect.TypeOf((*MockDBInterface)(nil).QueryRowContext), varargs...)
}

// Select mocks base method.
func (m *MockDBInterface) Select(ctx context.Context, data any, query string, args ...any) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, data, query}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Select", varargs...)
}

// Select indicates an expected call of Select.
func (mr *MockDBInterfaceMockRecorder) Select(ctx, data, query any, args ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, data, query}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Select", reflect.TypeOf((*MockDBInterface)(nil).Select), varargs...)
}
