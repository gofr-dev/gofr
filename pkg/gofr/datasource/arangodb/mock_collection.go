// Code generated by MockGen. DO NOT EDIT.
// Source: collection.go
//
// Generated by this command:
//
//	mockgen -source=collection.go -destination=./mock_collection.go -package=arangodb
//

// Package arangodb is a generated GoMock package.
package arangodb

import (
	context "context"
	reflect "reflect"

	arangodb "github.com/arangodb/go-driver/v2/arangodb"
	gomock "go.uber.org/mock/gomock"
)

// MockCollection is a mock of Collection interface.
type MockCollection struct {
	ctrl     *gomock.Controller
	recorder *MockCollectionMockRecorder
}

// MockCollectionMockRecorder is the mock recorder for MockCollection.
type MockCollectionMockRecorder struct {
	mock *MockCollection
}

// NewMockCollection creates a new mock instance.
func NewMockCollection(ctrl *gomock.Controller) *MockCollection {
	mock := &MockCollection{ctrl: ctrl}
	mock.recorder = &MockCollectionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCollection) EXPECT() *MockCollectionMockRecorder {
	return m.recorder
}

// Count mocks base method.
func (m *MockCollection) Count(ctx context.Context) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Count", ctx)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Count indicates an expected call of Count.
func (mr *MockCollectionMockRecorder) Count(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Count", reflect.TypeOf((*MockCollection)(nil).Count), ctx)
}

// CreateDocument mocks base method.
func (m *MockCollection) CreateDocument(ctx context.Context, document any) (arangodb.CollectionDocumentCreateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDocument", ctx, document)
	ret0, _ := ret[0].(arangodb.CollectionDocumentCreateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDocument indicates an expected call of CreateDocument.
func (mr *MockCollectionMockRecorder) CreateDocument(ctx, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDocument", reflect.TypeOf((*MockCollection)(nil).CreateDocument), ctx, document)
}

// CreateDocumentWithOptions mocks base method.
func (m *MockCollection) CreateDocumentWithOptions(ctx context.Context, document any, options *arangodb.CollectionDocumentCreateOptions) (arangodb.CollectionDocumentCreateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDocumentWithOptions", ctx, document, options)
	ret0, _ := ret[0].(arangodb.CollectionDocumentCreateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDocumentWithOptions indicates an expected call of CreateDocumentWithOptions.
func (mr *MockCollectionMockRecorder) CreateDocumentWithOptions(ctx, document, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDocumentWithOptions", reflect.TypeOf((*MockCollection)(nil).CreateDocumentWithOptions), ctx, document, options)
}

// CreateDocuments mocks base method.
func (m *MockCollection) CreateDocuments(ctx context.Context, documents any) (arangodb.CollectionDocumentCreateResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDocuments", ctx, documents)
	ret0, _ := ret[0].(arangodb.CollectionDocumentCreateResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDocuments indicates an expected call of CreateDocuments.
func (mr *MockCollectionMockRecorder) CreateDocuments(ctx, documents any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDocuments", reflect.TypeOf((*MockCollection)(nil).CreateDocuments), ctx, documents)
}

// CreateDocumentsWithOptions mocks base method.
func (m *MockCollection) CreateDocumentsWithOptions(ctx context.Context, documents any, opts *arangodb.CollectionDocumentCreateOptions) (arangodb.CollectionDocumentCreateResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDocumentsWithOptions", ctx, documents, opts)
	ret0, _ := ret[0].(arangodb.CollectionDocumentCreateResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateDocumentsWithOptions indicates an expected call of CreateDocumentsWithOptions.
func (mr *MockCollectionMockRecorder) CreateDocumentsWithOptions(ctx, documents, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDocumentsWithOptions", reflect.TypeOf((*MockCollection)(nil).CreateDocumentsWithOptions), ctx, documents, opts)
}

// Database mocks base method.
func (m *MockCollection) Database() arangodb.Database {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "database")
	ret0, _ := ret[0].(arangodb.Database)
	return ret0
}

// Database indicates an expected call of Database.
func (mr *MockCollectionMockRecorder) Database() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "database", reflect.TypeOf((*MockCollection)(nil).Database))
}

// DeleteDocument mocks base method.
func (m *MockCollection) DeleteDocument(ctx context.Context, key string) (arangodb.CollectionDocumentDeleteResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDocument", ctx, key)
	ret0, _ := ret[0].(arangodb.CollectionDocumentDeleteResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteDocument indicates an expected call of DeleteDocument.
func (mr *MockCollectionMockRecorder) DeleteDocument(ctx, key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDocument", reflect.TypeOf((*MockCollection)(nil).DeleteDocument), ctx, key)
}

// DeleteDocumentWithOptions mocks base method.
func (m *MockCollection) DeleteDocumentWithOptions(ctx context.Context, key string, opts *arangodb.CollectionDocumentDeleteOptions) (arangodb.CollectionDocumentDeleteResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDocumentWithOptions", ctx, key, opts)
	ret0, _ := ret[0].(arangodb.CollectionDocumentDeleteResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteDocumentWithOptions indicates an expected call of DeleteDocumentWithOptions.
func (mr *MockCollectionMockRecorder) DeleteDocumentWithOptions(ctx, key, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDocumentWithOptions", reflect.TypeOf((*MockCollection)(nil).DeleteDocumentWithOptions), ctx, key, opts)
}

// DeleteDocuments mocks base method.
func (m *MockCollection) DeleteDocuments(ctx context.Context, keys []string) (arangodb.CollectionDocumentDeleteResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDocuments", ctx, keys)
	ret0, _ := ret[0].(arangodb.CollectionDocumentDeleteResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteDocuments indicates an expected call of DeleteDocuments.
func (mr *MockCollectionMockRecorder) DeleteDocuments(ctx, keys any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDocuments", reflect.TypeOf((*MockCollection)(nil).DeleteDocuments), ctx, keys)
}

// DeleteDocumentsWithOptions mocks base method.
func (m *MockCollection) DeleteDocumentsWithOptions(ctx context.Context, documents any, opts *arangodb.CollectionDocumentDeleteOptions) (arangodb.CollectionDocumentDeleteResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteDocumentsWithOptions", ctx, documents, opts)
	ret0, _ := ret[0].(arangodb.CollectionDocumentDeleteResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteDocumentsWithOptions indicates an expected call of DeleteDocumentsWithOptions.
func (mr *MockCollectionMockRecorder) DeleteDocumentsWithOptions(ctx, documents, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDocumentsWithOptions", reflect.TypeOf((*MockCollection)(nil).DeleteDocumentsWithOptions), ctx, documents, opts)
}

// DeleteIndex mocks base method.
func (m *MockCollection) DeleteIndex(ctx context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteIndex", ctx, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteIndex indicates an expected call of DeleteIndex.
func (mr *MockCollectionMockRecorder) DeleteIndex(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteIndex", reflect.TypeOf((*MockCollection)(nil).DeleteIndex), ctx, name)
}

// DeleteIndexByID mocks base method.
func (m *MockCollection) DeleteIndexByID(ctx context.Context, id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteIndexByID", ctx, id)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteIndexByID indicates an expected call of DeleteIndexByID.
func (mr *MockCollectionMockRecorder) DeleteIndexByID(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteIndexByID", reflect.TypeOf((*MockCollection)(nil).DeleteIndexByID), ctx, id)
}

// DocumentExists mocks base method.
func (m *MockCollection) DocumentExists(ctx context.Context, key string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DocumentExists", ctx, key)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DocumentExists indicates an expected call of DocumentExists.
func (mr *MockCollectionMockRecorder) DocumentExists(ctx, key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DocumentExists", reflect.TypeOf((*MockCollection)(nil).DocumentExists), ctx, key)
}

// EnsureGeoIndex mocks base method.
func (m *MockCollection) EnsureGeoIndex(ctx context.Context, fields []string, options *arangodb.CreateGeoIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureGeoIndex", ctx, fields, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsureGeoIndex indicates an expected call of EnsureGeoIndex.
func (mr *MockCollectionMockRecorder) EnsureGeoIndex(ctx, fields, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureGeoIndex", reflect.TypeOf((*MockCollection)(nil).EnsureGeoIndex), ctx, fields, options)
}

// EnsureInvertedIndex mocks base method.
func (m *MockCollection) EnsureInvertedIndex(ctx context.Context, options *arangodb.InvertedIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureInvertedIndex", ctx, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsureInvertedIndex indicates an expected call of EnsureInvertedIndex.
func (mr *MockCollectionMockRecorder) EnsureInvertedIndex(ctx, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureInvertedIndex", reflect.TypeOf((*MockCollection)(nil).EnsureInvertedIndex), ctx, options)
}

// EnsureMDIIndex mocks base method.
func (m *MockCollection) EnsureMDIIndex(ctx context.Context, fields []string, options *arangodb.CreateMDIIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureMDIIndex", ctx, fields, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsureMDIIndex indicates an expected call of EnsureMDIIndex.
func (mr *MockCollectionMockRecorder) EnsureMDIIndex(ctx, fields, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureMDIIndex", reflect.TypeOf((*MockCollection)(nil).EnsureMDIIndex), ctx, fields, options)
}

// EnsureMDIPrefixedIndex mocks base method.
func (m *MockCollection) EnsureMDIPrefixedIndex(ctx context.Context, fields []string, options *arangodb.CreateMDIPrefixedIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureMDIPrefixedIndex", ctx, fields, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsureMDIPrefixedIndex indicates an expected call of EnsureMDIPrefixedIndex.
func (mr *MockCollectionMockRecorder) EnsureMDIPrefixedIndex(ctx, fields, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureMDIPrefixedIndex", reflect.TypeOf((*MockCollection)(nil).EnsureMDIPrefixedIndex), ctx, fields, options)
}

// EnsurePersistentIndex mocks base method.
func (m *MockCollection) EnsurePersistentIndex(ctx context.Context, fields []string, options *arangodb.CreatePersistentIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsurePersistentIndex", ctx, fields, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsurePersistentIndex indicates an expected call of EnsurePersistentIndex.
func (mr *MockCollectionMockRecorder) EnsurePersistentIndex(ctx, fields, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsurePersistentIndex", reflect.TypeOf((*MockCollection)(nil).EnsurePersistentIndex), ctx, fields, options)
}

// EnsureTTLIndex mocks base method.
func (m *MockCollection) EnsureTTLIndex(ctx context.Context, fields []string, expireAfter int, options *arangodb.CreateTTLIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureTTLIndex", ctx, fields, expireAfter, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsureTTLIndex indicates an expected call of EnsureTTLIndex.
func (mr *MockCollectionMockRecorder) EnsureTTLIndex(ctx, fields, expireAfter, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureTTLIndex", reflect.TypeOf((*MockCollection)(nil).EnsureTTLIndex), ctx, fields, expireAfter, options)
}

// EnsureZKDIndex mocks base method.
func (m *MockCollection) EnsureZKDIndex(ctx context.Context, fields []string, options *arangodb.CreateZKDIndexOptions) (arangodb.IndexResponse, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnsureZKDIndex", ctx, fields, options)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// EnsureZKDIndex indicates an expected call of EnsureZKDIndex.
func (mr *MockCollectionMockRecorder) EnsureZKDIndex(ctx, fields, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnsureZKDIndex", reflect.TypeOf((*MockCollection)(nil).EnsureZKDIndex), ctx, fields, options)
}

// Index mocks base method.
func (m *MockCollection) Index(ctx context.Context, name string) (arangodb.IndexResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Index", ctx, name)
	ret0, _ := ret[0].(arangodb.IndexResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Index indicates an expected call of Index.
func (mr *MockCollectionMockRecorder) Index(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Index", reflect.TypeOf((*MockCollection)(nil).Index), ctx, name)
}

// IndexExists mocks base method.
func (m *MockCollection) IndexExists(ctx context.Context, name string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IndexExists", ctx, name)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IndexExists indicates an expected call of IndexExists.
func (mr *MockCollectionMockRecorder) IndexExists(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IndexExists", reflect.TypeOf((*MockCollection)(nil).IndexExists), ctx, name)
}

// Indexes mocks base method.
func (m *MockCollection) Indexes(ctx context.Context) ([]arangodb.IndexResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Indexes", ctx)
	ret0, _ := ret[0].([]arangodb.IndexResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Indexes indicates an expected call of Indexes.
func (mr *MockCollectionMockRecorder) Indexes(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Indexes", reflect.TypeOf((*MockCollection)(nil).Indexes), ctx)
}

// Name mocks base method.
func (m *MockCollection) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockCollectionMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockCollection)(nil).Name))
}

// Properties mocks base method.
func (m *MockCollection) Properties(ctx context.Context) (arangodb.CollectionProperties, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Properties", ctx)
	ret0, _ := ret[0].(arangodb.CollectionProperties)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Properties indicates an expected call of Properties.
func (mr *MockCollectionMockRecorder) Properties(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Properties", reflect.TypeOf((*MockCollection)(nil).Properties), ctx)
}

// ReadDocument mocks base method.
func (m *MockCollection) ReadDocument(ctx context.Context, key string, result any) (arangodb.DocumentMeta, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDocument", ctx, key, result)
	ret0, _ := ret[0].(arangodb.DocumentMeta)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDocument indicates an expected call of ReadDocument.
func (mr *MockCollectionMockRecorder) ReadDocument(ctx, key, result any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDocument", reflect.TypeOf((*MockCollection)(nil).ReadDocument), ctx, key, result)
}

// ReadDocumentWithOptions mocks base method.
func (m *MockCollection) ReadDocumentWithOptions(ctx context.Context, key string, result any, opts *arangodb.CollectionDocumentReadOptions) (arangodb.DocumentMeta, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDocumentWithOptions", ctx, key, result, opts)
	ret0, _ := ret[0].(arangodb.DocumentMeta)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDocumentWithOptions indicates an expected call of ReadDocumentWithOptions.
func (mr *MockCollectionMockRecorder) ReadDocumentWithOptions(ctx, key, result, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDocumentWithOptions", reflect.TypeOf((*MockCollection)(nil).ReadDocumentWithOptions), ctx, key, result, opts)
}

// ReadDocuments mocks base method.
func (m *MockCollection) ReadDocuments(ctx context.Context, keys []string) (arangodb.CollectionDocumentReadResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDocuments", ctx, keys)
	ret0, _ := ret[0].(arangodb.CollectionDocumentReadResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDocuments indicates an expected call of ReadDocuments.
func (mr *MockCollectionMockRecorder) ReadDocuments(ctx, keys any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDocuments", reflect.TypeOf((*MockCollection)(nil).ReadDocuments), ctx, keys)
}

// ReadDocumentsWithOptions mocks base method.
func (m *MockCollection) ReadDocumentsWithOptions(ctx context.Context, documents any, opts *arangodb.CollectionDocumentReadOptions) (arangodb.CollectionDocumentReadResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadDocumentsWithOptions", ctx, documents, opts)
	ret0, _ := ret[0].(arangodb.CollectionDocumentReadResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadDocumentsWithOptions indicates an expected call of ReadDocumentsWithOptions.
func (mr *MockCollectionMockRecorder) ReadDocumentsWithOptions(ctx, documents, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadDocumentsWithOptions", reflect.TypeOf((*MockCollection)(nil).ReadDocumentsWithOptions), ctx, documents, opts)
}

// Remove mocks base method.
func (m *MockCollection) Remove(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockCollectionMockRecorder) Remove(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockCollection)(nil).Remove), ctx)
}

// RemoveWithOptions mocks base method.
func (m *MockCollection) RemoveWithOptions(ctx context.Context, opts *arangodb.RemoveCollectionOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveWithOptions", ctx, opts)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveWithOptions indicates an expected call of RemoveWithOptions.
func (mr *MockCollectionMockRecorder) RemoveWithOptions(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveWithOptions", reflect.TypeOf((*MockCollection)(nil).RemoveWithOptions), ctx, opts)
}

// ReplaceDocument mocks base method.
func (m *MockCollection) ReplaceDocument(ctx context.Context, key string, document any) (arangodb.CollectionDocumentReplaceResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceDocument", ctx, key, document)
	ret0, _ := ret[0].(arangodb.CollectionDocumentReplaceResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReplaceDocument indicates an expected call of ReplaceDocument.
func (mr *MockCollectionMockRecorder) ReplaceDocument(ctx, key, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceDocument", reflect.TypeOf((*MockCollection)(nil).ReplaceDocument), ctx, key, document)
}

// ReplaceDocumentWithOptions mocks base method.
func (m *MockCollection) ReplaceDocumentWithOptions(ctx context.Context, key string, document any, options *arangodb.CollectionDocumentReplaceOptions) (arangodb.CollectionDocumentReplaceResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceDocumentWithOptions", ctx, key, document, options)
	ret0, _ := ret[0].(arangodb.CollectionDocumentReplaceResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReplaceDocumentWithOptions indicates an expected call of ReplaceDocumentWithOptions.
func (mr *MockCollectionMockRecorder) ReplaceDocumentWithOptions(ctx, key, document, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceDocumentWithOptions", reflect.TypeOf((*MockCollection)(nil).ReplaceDocumentWithOptions), ctx, key, document, options)
}

// ReplaceDocuments mocks base method.
func (m *MockCollection) ReplaceDocuments(ctx context.Context, documents any) (arangodb.CollectionDocumentReplaceResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceDocuments", ctx, documents)
	ret0, _ := ret[0].(arangodb.CollectionDocumentReplaceResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReplaceDocuments indicates an expected call of ReplaceDocuments.
func (mr *MockCollectionMockRecorder) ReplaceDocuments(ctx, documents any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceDocuments", reflect.TypeOf((*MockCollection)(nil).ReplaceDocuments), ctx, documents)
}

// ReplaceDocumentsWithOptions mocks base method.
func (m *MockCollection) ReplaceDocumentsWithOptions(ctx context.Context, documents any, opts *arangodb.CollectionDocumentReplaceOptions) (arangodb.CollectionDocumentReplaceResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceDocumentsWithOptions", ctx, documents, opts)
	ret0, _ := ret[0].(arangodb.CollectionDocumentReplaceResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReplaceDocumentsWithOptions indicates an expected call of ReplaceDocumentsWithOptions.
func (mr *MockCollectionMockRecorder) ReplaceDocumentsWithOptions(ctx, documents, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceDocumentsWithOptions", reflect.TypeOf((*MockCollection)(nil).ReplaceDocumentsWithOptions), ctx, documents, opts)
}

// SetProperties mocks base method.
func (m *MockCollection) SetProperties(ctx context.Context, options arangodb.SetCollectionPropertiesOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetProperties", ctx, options)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetProperties indicates an expected call of SetProperties.
func (mr *MockCollectionMockRecorder) SetProperties(ctx, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProperties", reflect.TypeOf((*MockCollection)(nil).SetProperties), ctx, options)
}

// Shards mocks base method.
func (m *MockCollection) Shards(ctx context.Context, details bool) (arangodb.CollectionShards, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Shards", ctx, details)
	ret0, _ := ret[0].(arangodb.CollectionShards)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Shards indicates an expected call of Shards.
func (mr *MockCollectionMockRecorder) Shards(ctx, details any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Shards", reflect.TypeOf((*MockCollection)(nil).Shards), ctx, details)
}

// Truncate mocks base method.
func (m *MockCollection) Truncate(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Truncate", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Truncate indicates an expected call of Truncate.
func (mr *MockCollectionMockRecorder) Truncate(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Truncate", reflect.TypeOf((*MockCollection)(nil).Truncate), ctx)
}

// UpdateDocument mocks base method.
func (m *MockCollection) UpdateDocument(ctx context.Context, key string, document any) (arangodb.CollectionDocumentUpdateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDocument", ctx, key, document)
	ret0, _ := ret[0].(arangodb.CollectionDocumentUpdateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateDocument indicates an expected call of UpdateDocument.
func (mr *MockCollectionMockRecorder) UpdateDocument(ctx, key, document any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDocument", reflect.TypeOf((*MockCollection)(nil).UpdateDocument), ctx, key, document)
}

// UpdateDocumentWithOptions mocks base method.
func (m *MockCollection) UpdateDocumentWithOptions(ctx context.Context, key string, document any, options *arangodb.CollectionDocumentUpdateOptions) (arangodb.CollectionDocumentUpdateResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDocumentWithOptions", ctx, key, document, options)
	ret0, _ := ret[0].(arangodb.CollectionDocumentUpdateResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateDocumentWithOptions indicates an expected call of UpdateDocumentWithOptions.
func (mr *MockCollectionMockRecorder) UpdateDocumentWithOptions(ctx, key, document, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDocumentWithOptions", reflect.TypeOf((*MockCollection)(nil).UpdateDocumentWithOptions), ctx, key, document, options)
}

// UpdateDocuments mocks base method.
func (m *MockCollection) UpdateDocuments(ctx context.Context, documents any) (arangodb.CollectionDocumentUpdateResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDocuments", ctx, documents)
	ret0, _ := ret[0].(arangodb.CollectionDocumentUpdateResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateDocuments indicates an expected call of UpdateDocuments.
func (mr *MockCollectionMockRecorder) UpdateDocuments(ctx, documents any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDocuments", reflect.TypeOf((*MockCollection)(nil).UpdateDocuments), ctx, documents)
}

// UpdateDocumentsWithOptions mocks base method.
func (m *MockCollection) UpdateDocumentsWithOptions(ctx context.Context, documents any, opts *arangodb.CollectionDocumentUpdateOptions) (arangodb.CollectionDocumentUpdateResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateDocumentsWithOptions", ctx, documents, opts)
	ret0, _ := ret[0].(arangodb.CollectionDocumentUpdateResponseReader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateDocumentsWithOptions indicates an expected call of UpdateDocumentsWithOptions.
func (mr *MockCollectionMockRecorder) UpdateDocumentsWithOptions(ctx, documents, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDocumentsWithOptions", reflect.TypeOf((*MockCollection)(nil).UpdateDocumentsWithOptions), ctx, documents, opts)
}
