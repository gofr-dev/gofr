// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go
//
// Generated by this command:
//
//	mockgen -source=interface.go -destination=mock_interfaces.go -package=arango
//

// Package arango is a generated GoMock package.
package arango

import (
	 "context"
	"fmt"
	"github.com/arangodb/go-driver/v2/connection"
	reflect "reflect"
	"strings"

	arangodb "github.com/arangodb/go-driver/v2/arangodb"
	gomock "go.uber.org/mock/gomock"
)

// MockArangoMockRecorder is the mock recorder for MockArango.
type MockArangoMockRecorder struct {
	mock *MockArango
}

// NewMockArango creates a new mock instance.
func NewMockArango(ctrl *gomock.Controller) *MockArango {
	mock := &MockArango{ctrl: ctrl}
	mock.recorder = &MockArangoMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockArango) EXPECT() *MockArangoMockRecorder {
	return m.recorder
}


// MockArango is a mock of Arango interface.
type MockArango struct {
	ctrl     *gomock.Controller
	recorder *MockArangoMockRecorder
}

func (m *MockArango) Connection() connection.Connection {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Connection")
	return ret[0].(connection.Connection)
}

func (mr *MockArangoMockRecorder) Connection() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Connection", reflect.TypeOf((*MockArango)(nil).Connection))
}

func (m *MockArango) Get(ctx context.Context, output interface{}, urlParts ...string) (connection.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, output, urlParts)
	return ret[0].(connection.Response), ret[1].(error)
}

func (mr *MockArangoMockRecorder) Get(ctx, output interface{}, urlParts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockArango)(nil).Get), ctx, output, urlParts)
}

func (m *MockArango) Post(ctx context.Context, output, input interface{}, urlParts ...string) (connection.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Post", ctx, output, input, urlParts)
	return ret[0].(connection.Response), ret[1].(error)
}

func (mr *MockArangoMockRecorder) Post(ctx, output, input interface{}, urlParts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Post", reflect.TypeOf((*MockArango)(nil).Post), ctx, output, input, urlParts)
}

func (m *MockArango) Put(ctx context.Context, output, input interface{}, urlParts ...string) (connection.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", ctx, output, input, urlParts)
	return ret[0].(connection.Response), ret[1].(error)
}

func (mr *MockArangoMockRecorder) Put(ctx, output, input interface{}, urlParts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockArango)(nil).Put), ctx, output, input, urlParts)
}

func (m *MockArango) Delete(ctx context.Context, output interface{}, urlParts ...string) (connection.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx, output, urlParts)
	return ret[0].(connection.Response), ret[1].(error)
}

func (mr *MockArangoMockRecorder) Delete(ctx, output interface{}, urlParts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockArango)(nil).Delete), ctx, output, urlParts)
}

func (m *MockArango) Head(ctx context.Context, output interface{}, urlParts ...string) (connection.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Head", ctx, output, urlParts)
	return ret[0].(connection.Response), ret[1].(error)
}

func (mr *MockArangoMockRecorder) Head(ctx, output interface{}, urlParts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Head", reflect.TypeOf((*MockArango)(nil).Head), ctx, output, urlParts)
}

func (m *MockArango) Patch(ctx context.Context, output, input interface{}, urlParts ...string) (connection.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Patch", ctx, output, input, urlParts)
	return ret[0].(connection.Response), ret[1].(error)
}

func (mr *MockArangoMockRecorder) Patch(ctx, output, input interface{}, urlParts ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Patch", reflect.TypeOf((*MockArango)(nil).Patch), ctx, output, input, urlParts)
}

func (m *MockArango) CreateDatabase(ctx context.Context, name string, options *arangodb.CreateDatabaseOptions) (arangodb.Database, error) {
	db := NewMockDatabase(m.ctrl)
	if strings.Contains(name, "error") {
		return nil,fmt.Errorf("database not found")
	}

	return db,nil
}

func (mr *MockArangoMockRecorder) CreateDatabase(ctx, name, options interface{}) *gomock.Call {
	return mr.CreateDB(ctx, name)
}

func (m *MockArango) GetDatabase(ctx context.Context, name string, options *arangodb.GetDatabaseOptions) (arangodb.Database, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDatabase", ctx, name, options)
	return ret[0].(arangodb.Database), ret[1].(error)
}

func (mr *MockArangoMockRecorder) GetDatabase(ctx, name, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDatabase", reflect.TypeOf((*MockArango)(nil).GetDatabase), ctx, name, options)
}

func (m *MockArango) DatabaseExists(ctx context.Context, name string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) AccessibleDatabases(ctx context.Context) ([]arangodb.Database, error) {
	//TODO implement me
	panic("implement me")
}


func (m *MockArango) UserExists(ctx context.Context, name string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) Users(ctx context.Context) ([]arangodb.User, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) ReplaceUser(ctx context.Context, name string, options *arangodb.UserOptions) (arangodb.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceUser", ctx, name, options)
	return ret[0].(arangodb.User), ret[1].(error)
}

// Mock recorder method for setting up expectations
func (mr *MockArangoMockRecorder) ReplaceUser(ctx, name, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceUser", reflect.TypeOf((*MockArango)(nil).ReplaceUser), ctx, name, options)
}

func (m *MockArango) UpdateUser(ctx context.Context, name string, options *arangodb.UserOptions) (arangodb.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateUser", ctx, name, options)
	return ret[0].(arangodb.User), ret[1].(error)
}

// Mock recorder method for setting up expectations
func (mr *MockArangoMockRecorder) UpdateUser(ctx, name, options interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateUser", reflect.TypeOf((*MockArango)(nil).UpdateUser), ctx, name, options)
}


func (m *MockArango) RemoveUser(ctx context.Context, name string) error {
	return m.DropUser(ctx, name)
}

func (m *MockArango) VersionWithOptions(ctx context.Context, opts *arangodb.GetVersionOptions) (arangodb.VersionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VersionWithOptions", ctx, opts)
	return ret[0].(arangodb.VersionInfo), ret[1].(error)
}

// Mock recorder method for setting up expectations
func (mr *MockArangoMockRecorder) VersionWithOptions(ctx, opts interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VersionWithOptions", reflect.TypeOf((*MockArango)(nil).VersionWithOptions), ctx, opts)
}


func (m *MockArango) ServerRole(ctx context.Context) (arangodb.ServerRole, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) ServerID(ctx context.Context) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) GetLogLevels(ctx context.Context, opts *arangodb.LogLevelsGetOptions) (arangodb.LogLevels, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) SetLogLevels(ctx context.Context, logLevels arangodb.LogLevels, opts *arangodb.LogLevelsSetOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) BackupCreate(ctx context.Context, opt *arangodb.BackupCreateOptions) (arangodb.BackupResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) BackupRestore(ctx context.Context, id string) (arangodb.BackupRestoreResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) BackupDelete(ctx context.Context, id string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) BackupList(ctx context.Context, opt *arangodb.BackupListOptions) (arangodb.ListBackupsResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) BackupUpload(ctx context.Context, backupId string, remoteRepository string, config interface{}) (arangodb.TransferMonitor, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) BackupDownload(ctx context.Context, backupId string, remoteRepository string, config interface{}) (arangodb.TransferMonitor, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) TransferMonitor(jobId string, transferType arangodb.TransferType) (arangodb.TransferMonitor, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) GetLicense(ctx context.Context) (arangodb.License, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) SetLicense(ctx context.Context, license string, force bool) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) Health(ctx context.Context) (arangodb.ClusterHealth, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) DatabaseInventory(ctx context.Context, dbName string) (arangodb.DatabaseInventory, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) MoveShard(ctx context.Context, col arangodb.Collection, shard arangodb.ShardID, fromServer, toServer arangodb.ServerID) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) CleanOutServer(ctx context.Context, serverID arangodb.ServerID) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) ResignServer(ctx context.Context, serverID arangodb.ServerID) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) NumberOfServers(ctx context.Context) (arangodb.NumberOfServersResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) IsCleanedOut(ctx context.Context, serverID arangodb.ServerID) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) RemoveServer(ctx context.Context, serverID arangodb.ServerID) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) ServerMode(ctx context.Context) (arangodb.ServerMode, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) SetServerMode(ctx context.Context, mode arangodb.ServerMode) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) CheckAvailability(ctx context.Context, serverEndpoint string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) AsyncJobList(ctx context.Context, jobType arangodb.AsyncJobStatusType, opts *arangodb.AsyncJobListOptions) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) AsyncJobStatus(ctx context.Context, jobID string) (arangodb.AsyncJobStatusType, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) AsyncJobCancel(ctx context.Context, jobID string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockArango) AsyncJobDelete(ctx context.Context, deleteType arangodb.AsyncJobDeleteType, opts *arangodb.AsyncJobDeleteOptions) (bool, error) {
	//TODO implement me
	panic("implement me")
}

// Connect mocks base method.
func (m *MockArango) Connect() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Connect")
}

// Connect indicates an expected call of Connect.
func (mr *MockArangoMockRecorder) Connect() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Connect", reflect.TypeOf((*MockArango)(nil).Connect))
}

// CreateCollection mocks base method.
func (m *MockArango) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateCollection", ctx, database, collection, isEdge)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateCollection indicates an expected call of CreateCollection.
func (mr *MockArangoMockRecorder) CreateCollection(ctx, database, collection, isEdge any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateCollection", reflect.TypeOf((*MockArango)(nil).CreateCollection), ctx, database, collection, isEdge)
}

// CreateDB mocks base method.
func (m *MockArango) CreateDB(ctx context.Context, database string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDB", ctx, database)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateDB indicates an expected call of CreateDB.
func (mr *MockArangoMockRecorder) CreateDB(ctx, database any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDB", reflect.TypeOf((*MockArango)(nil).CreateDB), ctx, database)
}

// CreateDocument mocks base method.
func (m *MockArango) CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDocument", ctx, dbName, collectionName, document)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDocument indicates an expected call of CreateDocument.
func (mr *MockArangoMockRecorder) CreateDocument(ctx, dbName, collectionName, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDocument", reflect.TypeOf((*MockArango)(nil).CreateDocument), ctx, dbName, collectionName, document)
}

// CreateEdgeDocument mocks base method.
func (m *MockArango) CreateEdgeDocument(ctx context.Context, dbName, collectionName, from, to string, document any) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateEdgeDocument", ctx, dbName, collectionName, from, to, document)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateEdgeDocument indicates an expected call of CreateEdgeDocument.
func (mr *MockArangoMockRecorder) CreateEdgeDocument(ctx, dbName, collectionName, from, to, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateEdgeDocument", reflect.TypeOf((*MockArango)(nil).CreateEdgeDocument), ctx, dbName, collectionName, from, to, document)
}

// CreateGraph mocks base method.
func (m *MockArango) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGraph", ctx, database, graph, edgeDefinitions)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateGraph indicates an expected call of CreateGraph.
func (mr *MockArangoMockRecorder) CreateGraph(ctx, database, graph, edgeDefinitions any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGraph", reflect.TypeOf((*MockArango)(nil).CreateGraph), ctx, database, graph, edgeDefinitions)
}

func (m *MockArango) CreateUser(ctx context. Context, name string, options *arangodb.UserOptions) (arangodb.User, error){
	return nil,m.AddUser(ctx, name, options)
}

// CreateUser mocks base method.
func (m *MockArango) AddUser(ctx context.Context, username string, options any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateUser", ctx, username, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateUser indicates an expected call of CreateUser.
func (mr *MockArangoMockRecorder) AddUser(ctx, username, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateUser", reflect.TypeOf((*MockArango)(nil).CreateUser), ctx, username, options)
}

// Database mocks base method.
func (m *MockArango) Database(ctx context.Context, name string) (arangodb.Database, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Database", ctx, name)
	ret0, _ := ret[0].(arangodb.Database)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Database indicates an expected call of Database.
func (mr *MockArangoMockRecorder) Database(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Database", reflect.TypeOf((*MockArango)(nil).Database), ctx, name)
}

// Databases mocks base method.
func (m *MockArango) Databases(ctx context.Context) ([]arangodb.Database, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Databases", ctx)
	ret0, _ := ret[0].([]arangodb.Database)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Databases indicates an expected call of Databases.
func (mr *MockArangoMockRecorder) Databases(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Databases", reflect.TypeOf((*MockArango)(nil).Databases), ctx)
}

// DeleteDocument mocks base method.
func (m *MockArango) DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDocument", ctx, dbName, collectionName, documentID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteDocument indicates an expected call of DeleteDocument.
func (mr *MockArangoMockRecorder) DeleteDocument(ctx, dbName, collectionName, documentID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDocument", reflect.TypeOf((*MockArango)(nil).DeleteDocument), ctx, dbName, collectionName, documentID)
}

// DropCollection mocks base method.
func (m *MockArango) DropCollection(ctx context.Context, database, collection string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropCollection", ctx, database, collection)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropCollection indicates an expected call of DropCollection.
func (mr *MockArangoMockRecorder) DropCollection(ctx, database, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropCollection", reflect.TypeOf((*MockArango)(nil).DropCollection), ctx, database, collection)
}

// DropDB mocks base method.
func (m *MockArango) DropDB(ctx context.Context, database string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropDB", ctx, database)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropDB indicates an expected call of DropDB.
func (mr *MockArangoMockRecorder) DropDB(ctx, database any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropDB", reflect.TypeOf((*MockArango)(nil).DropDB), ctx, database)
}

// DropGraph mocks base method.
func (m *MockArango) DropGraph(ctx context.Context, database, graph string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropGraph", ctx, database, graph)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropGraph indicates an expected call of DropGraph.
func (mr *MockArangoMockRecorder) DropGraph(ctx, database, graph any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropGraph", reflect.TypeOf((*MockArango)(nil).DropGraph), ctx, database, graph)
}

// DropUser mocks base method.
func (m *MockArango) DropUser(ctx context.Context, username string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DropUser", ctx, username)
	ret0, _ := ret[0].(error)
	return ret0
}

// DropUser indicates an expected call of DropUser.
func (mr *MockArangoMockRecorder) DropUser(ctx, username any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DropUser", reflect.TypeOf((*MockArango)(nil).DropUser), ctx, username)
}

// GetDocument mocks base method.
func (m *MockArango) GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDocument", ctx, dbName, collectionName, documentID, result)
	ret0, _ := ret[0].(error)
	return ret0
}

// GetDocument indicates an expected call of GetDocument.
func (mr *MockArangoMockRecorder) GetDocument(ctx, dbName, collectionName, documentID, result any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDocument", reflect.TypeOf((*MockArango)(nil).GetDocument), ctx, dbName, collectionName, documentID, result)
}

// GrantCollection mocks base method.
func (m *MockArango) GrantCollection(ctx context.Context, database, collection, username, permission string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GrantCollection", ctx, database, collection, username, permission)
	ret0, _ := ret[0].(error)
	return ret0
}

// GrantCollection indicates an expected call of GrantCollection.
func (mr *MockArangoMockRecorder) GrantCollection(ctx, database, collection, username, permission any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GrantCollection", reflect.TypeOf((*MockArango)(nil).GrantCollection), ctx, database, collection, username, permission)
}

// GrantDB mocks base method.
func (m *MockArango) GrantDB(ctx context.Context, database, username, permission string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GrantDB", ctx, database, username, permission)
	ret0, _ := ret[0].(error)
	return ret0
}

// GrantDB indicates an expected call of GrantDB.
func (mr *MockArangoMockRecorder) GrantDB(ctx, database, username, permission any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GrantDB", reflect.TypeOf((*MockArango)(nil).GrantDB), ctx, database, username, permission)
}

// HealthCheck mocks base method.
func (m *MockArango) HealthCheck(ctx context.Context) (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HealthCheck", ctx)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// HealthCheck indicates an expected call of HealthCheck.
func (mr *MockArangoMockRecorder) HealthCheck(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthCheck", reflect.TypeOf((*MockArango)(nil).HealthCheck), ctx)
}

// ListCollections mocks base method.
func (m *MockArango) ListCollections(ctx context.Context, database string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListCollections", ctx, database)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListCollections indicates an expected call of ListCollections.
func (mr *MockArangoMockRecorder) ListCollections(ctx, database any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListCollections", reflect.TypeOf((*MockArango)(nil).ListCollections), ctx, database)
}

// ListDBs mocks base method.
func (m *MockArango) ListDBs(ctx context.Context) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListDBs", ctx)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListDBs indicates an expected call of ListDBs.
func (mr *MockArangoMockRecorder) ListDBs(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListDBs", reflect.TypeOf((*MockArango)(nil).ListDBs), ctx)
}

// ListGraphs mocks base method.
func (m *MockArango) ListGraphs(ctx context.Context, database string) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListGraphs", ctx, database)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListGraphs indicates an expected call of ListGraphs.
func (mr *MockArangoMockRecorder) ListGraphs(ctx, database any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListGraphs", reflect.TypeOf((*MockArango)(nil).ListGraphs), ctx, database)
}

// Query mocks base method.
func (m *MockArango) Query(ctx context.Context, dbName, query string, bindVars map[string]any, result any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Query", ctx, dbName, query, bindVars, result)
	ret0, _ := ret[0].(error)
	return ret0
}

// Query indicates an expected call of Query.
func (mr *MockArangoMockRecorder) Query(ctx, dbName, query, bindVars, result any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockArango)(nil).Query), ctx, dbName, query, bindVars, result)
}

// TruncateCollection mocks base method.
func (m *MockArango) TruncateCollection(ctx context.Context, database, collection string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TruncateCollection", ctx, database, collection)
	ret0, _ := ret[0].(error)
	return ret0
}

// TruncateCollection indicates an expected call of TruncateCollection.
func (mr *MockArangoMockRecorder) TruncateCollection(ctx, database, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TruncateCollection", reflect.TypeOf((*MockArango)(nil).TruncateCollection), ctx, database, collection)
}

// UpdateDocument mocks base method.
func (m *MockArango) UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDocument", ctx, dbName, collectionName, documentID, document)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateDocument indicates an expected call of UpdateDocument.
func (mr *MockArangoMockRecorder) UpdateDocument(ctx, dbName, collectionName, documentID, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDocument", reflect.TypeOf((*MockArango)(nil).UpdateDocument), ctx, dbName, collectionName, documentID, document)
}

// User mocks base method.
func (m *MockArango) User(ctx context.Context, username string) (arangodb.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "User", ctx, username)
	ret0, _ := ret[0].(arangodb.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// User indicates an expected call of User.
func (mr *MockArangoMockRecorder) User(ctx, username any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "User", reflect.TypeOf((*MockArango)(nil).User), ctx, username)
}

// Version mocks base method.
func (m *MockArango) Version(ctx context.Context) (arangodb.VersionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Version", ctx)
	ret0, _ := ret[0].(arangodb.VersionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Version indicates an expected call of Version.
func (mr *MockArangoMockRecorder) Version(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Version", reflect.TypeOf((*MockArango)(nil).Version), ctx)
}
