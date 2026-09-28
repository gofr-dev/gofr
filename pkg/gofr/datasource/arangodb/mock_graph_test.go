package arangodb

import (
	"context"
	"reflect"

	"github.com/arangodb/go-driver/v2/arangodb"
	"go.uber.org/mock/gomock"
)

// MockDatabaseGraph is a mock of DatabaseGraph interface.
type MockDatabaseGraph struct {
	ctrl     *gomock.Controller
	recorder *MockDatabaseGraphMockRecorder
}

// MockDatabaseGraphMockRecorder is the mock recorder for MockDatabaseGraph.
type MockDatabaseGraphMockRecorder struct {
	mock *MockDatabaseGraph
}

type EdgeDirection string
type GetEdgesOptions struct {
	// The direction of the edges. Allowed values are "in" and "out". If not set, edges in both directions are returned.
	Direction EdgeDirection `json:"direction,omitempty"`

	// Set this to true to allow the Coordinator to ask any shard replica for the data, not only the shard leader.
	// This may result array of collection names that is used to create SatelliteCollections for a (Disjoint) SmartGraph
	// using SatelliteCollections (Enterprise Edition only). Each array element must be a string and a valid
	// collection name. The collection type cannot be modified later.
	Satellites []string `json:"satellites,omitempty"`
}

type GraphsResponseReader interface {
	// Read returns next Graph. If no Graph left, shared.NoMoreDocumentsError returned
	Read() (arangodb.Graph, error)
}

// NewMockDatabaseGraph creates a new mock instance.
func NewMockDatabaseGraph(ctrl *gomock.Controller) *MockDatabaseGraph {
	mock := &MockDatabaseGraph{ctrl: ctrl}
	mock.recorder = &MockDatabaseGraphMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDatabaseGraph) EXPECT() *MockDatabaseGraphMockRecorder {
	return m.recorder
}

// CreateGraph mocks base method.
func (m *MockDatabaseGraph) CreateGraph(ctx context.Context, name string, graph *arangodb.GraphDefinition,
	options *arangodb.CreateGraphOptions) (arangodb.Graph, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGraph", ctx, name, graph, options)
	ret0, _ := ret[0].(arangodb.Graph)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// CreateGraph indicates an expected call of CreateGraph.
func (mr *MockDatabaseGraphMockRecorder) CreateGraph(ctx, name, graph, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGraph",
		reflect.TypeOf((*MockDatabaseGraph)(nil).CreateGraph), ctx, name, graph, options)
}

// GetEdges mocks base method.
func (m *MockDatabaseGraph) GetEdges(ctx context.Context, name, vertex string,
	options *GetEdgesOptions) ([]EdgeDetails, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEdges", ctx, name, vertex, options)
	ret0, _ := ret[0].([]EdgeDetails)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// GetEdges indicates an expected call of GetEdges.
func (mr *MockDatabaseGraphMockRecorder) GetEdges(ctx, name, vertex, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEdges",
		reflect.TypeOf((*MockDatabaseGraph)(nil).GetEdges), ctx, name, vertex, options)
}

// Graph mocks base method.
func (m *MockDatabaseGraph) Graph(ctx context.Context, name string,
	options *arangodb.GetGraphOptions) (arangodb.Graph, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Graph", ctx, name, options)
	ret0, _ := ret[0].(arangodb.Graph)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// Graph indicates an expected call of Graph.
func (mr *MockDatabaseGraphMockRecorder) Graph(ctx, name, options any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Graph",
		reflect.TypeOf((*MockDatabaseGraph)(nil).Graph), ctx, name, options)
}

// GraphExists mocks base method.
func (m *MockDatabaseGraph) GraphExists(ctx context.Context, name string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GraphExists", ctx, name)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// GraphExists indicates an expected call of GraphExists.
func (mr *MockDatabaseGraphMockRecorder) GraphExists(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GraphExists",
		reflect.TypeOf((*MockDatabaseGraph)(nil).GraphExists), ctx, name)
}

// Graphs mocks base method.
func (m *MockDatabaseGraph) Graphs(ctx context.Context) (GraphsResponseReader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Graphs", ctx)
	ret0, _ := ret[0].(GraphsResponseReader)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// Graphs indicates an expected call of Graphs.
func (mr *MockDatabaseGraphMockRecorder) Graphs(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Graphs",
		reflect.TypeOf((*MockDatabaseGraph)(nil).Graphs), ctx)
}

// MockGraphsResponseReader is a mock of GraphsResponseReader interface.
type MockGraphsResponseReader struct {
	ctrl     *gomock.Controller
	recorder *MockGraphsResponseReaderMockRecorder
}

// MockGraphsResponseReaderMockRecorder is the mock recorder for MockGraphsResponseReader.
type MockGraphsResponseReaderMockRecorder struct {
	mock *MockGraphsResponseReader
}

// NewMockGraphsResponseReader creates a new mock instance.
func NewMockGraphsResponseReader(ctrl *gomock.Controller) *MockGraphsResponseReader {
	mock := &MockGraphsResponseReader{ctrl: ctrl}
	mock.recorder = &MockGraphsResponseReaderMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGraphsResponseReader) EXPECT() *MockGraphsResponseReaderMockRecorder {
	return m.recorder
}

// Read mocks base method.
func (m *MockGraphsResponseReader) Read() (arangodb.Graph, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read")
	ret0, _ := ret[0].(arangodb.Graph)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockGraphsResponseReaderMockRecorder) Read() *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read",
		reflect.TypeOf((*MockGraphsResponseReader)(nil).Read))
}

// MockGraph is a mock of Graph interface.
type MockGraph struct {
	ctrl     *gomock.Controller
	recorder *MockGraphMockRecorder
}

// MockGraphMockRecorder is the mock recorder for MockGraph.
type MockGraphMockRecorder struct {
	mock *MockGraph
}

// NewMockGraph creates a new mock instance.
func NewMockGraph(ctrl *gomock.Controller) *MockGraph {
	mock := &MockGraph{ctrl: ctrl}
	mock.recorder = &MockGraphMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGraph) EXPECT() *MockGraphMockRecorder {
	return m.recorder
}

// CreateEdgeDefinition mocks base method.
func (m *MockGraph) CreateEdgeDefinition(ctx context.Context, collection string, from, to []string,
	opts *arangodb.CreateEdgeDefinitionOptions) (arangodb.CreateEdgeDefinitionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateEdgeDefinition", ctx, collection, from, to, opts)
	ret0, _ := ret[0].(arangodb.CreateEdgeDefinitionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// CreateEdgeDefinition indicates an expected call of CreateEdgeDefinition.
func (mr *MockGraphMockRecorder) CreateEdgeDefinition(ctx, collection, from, to, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateEdgeDefinition",
		reflect.TypeOf((*MockGraph)(nil).CreateEdgeDefinition), ctx, collection, from, to, opts)
}

// CreateVertexCollection mocks base method.
func (m *MockGraph) CreateVertexCollection(ctx context.Context, name string,
	opts *arangodb.CreateVertexCollectionOptions) (arangodb.CreateVertexCollectionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateVertexCollection", ctx, name, opts)
	ret0, _ := ret[0].(arangodb.CreateVertexCollectionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// CreateVertexCollection indicates an expected call of CreateVertexCollection.
func (mr *MockGraphMockRecorder) CreateVertexCollection(ctx, name, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateVertexCollection",
		reflect.TypeOf((*MockGraph)(nil).CreateVertexCollection), ctx, name, opts)
}

// DeleteEdgeDefinition mocks base method.
func (m *MockGraph) DeleteEdgeDefinition(ctx context.Context, collection string,
	opts *arangodb.DeleteEdgeDefinitionOptions) (arangodb.DeleteEdgeDefinitionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteEdgeDefinition", ctx, collection, opts)
	ret0, _ := ret[0].(arangodb.DeleteEdgeDefinitionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// DeleteEdgeDefinition indicates an expected call of DeleteEdgeDefinition.
func (mr *MockGraphMockRecorder) DeleteEdgeDefinition(ctx, collection, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteEdgeDefinition",
		reflect.TypeOf((*MockGraph)(nil).DeleteEdgeDefinition), ctx, collection, opts)
}

// DeleteVertexCollection mocks base method.
func (m *MockGraph) DeleteVertexCollection(ctx context.Context, name string,
	opts *arangodb.DeleteVertexCollectionOptions) (arangodb.DeleteVertexCollectionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteVertexCollection", ctx, name, opts)
	ret0, _ := ret[0].(arangodb.DeleteVertexCollectionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// DeleteVertexCollection indicates an expected call of DeleteVertexCollection.
func (mr *MockGraphMockRecorder) DeleteVertexCollection(ctx, name, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteVertexCollection",
		reflect.TypeOf((*MockGraph)(nil).DeleteVertexCollection), ctx, name, opts)
}

// EdgeDefinition mocks base method.
func (m *MockGraph) EdgeDefinition(ctx context.Context, collection string) (arangodb.Edge, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EdgeDefinition", ctx, collection)
	ret0, _ := ret[0].(arangodb.Edge)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// EdgeDefinition indicates an expected call of EdgeDefinition.
func (mr *MockGraphMockRecorder) EdgeDefinition(ctx, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EdgeDefinition",
		reflect.TypeOf((*MockGraph)(nil).EdgeDefinition), ctx, collection)
}

// EdgeDefinitionExists mocks base method.
func (m *MockGraph) EdgeDefinitionExists(ctx context.Context, collection string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EdgeDefinitionExists", ctx, collection)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// EdgeDefinitionExists indicates an expected call of EdgeDefinitionExists.
func (mr *MockGraphMockRecorder) EdgeDefinitionExists(ctx, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EdgeDefinitionExists",
		reflect.TypeOf((*MockGraph)(nil).EdgeDefinitionExists), ctx, collection)
}

// EdgeDefinitions mocks base method.
func (m *MockGraph) EdgeDefinitions() []arangodb.EdgeDefinition {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EdgeDefinitions")
	ret0, _ := ret[0].([]arangodb.EdgeDefinition)

	return ret0
}

// EdgeDefinitions indicates an expected call of EdgeDefinitions.
func (mr *MockGraphMockRecorder) EdgeDefinitions() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EdgeDefinitions", reflect.TypeOf((*MockGraph)(nil).EdgeDefinitions))
}

// GetEdgeDefinitions mocks base method.
func (m *MockGraph) GetEdgeDefinitions(ctx context.Context) ([]arangodb.Edge, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEdgeDefinitions", ctx)
	ret0, _ := ret[0].([]arangodb.Edge)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// GetEdgeDefinitions indicates an expected call of GetEdgeDefinitions.
func (mr *MockGraphMockRecorder) GetEdgeDefinitions(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEdgeDefinitions", reflect.TypeOf((*MockGraph)(nil).GetEdgeDefinitions), ctx)
}

// IsDisjoint mocks base method.
func (m *MockGraph) IsDisjoint() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsDisjoint")
	ret0, _ := ret[0].(bool)

	return ret0
}

// IsDisjoint indicates an expected call of IsDisjoint.
func (mr *MockGraphMockRecorder) IsDisjoint() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDisjoint", reflect.TypeOf((*MockGraph)(nil).IsDisjoint))
}

// IsSatellite mocks base method.
func (m *MockGraph) IsSatellite() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSatellite")
	ret0, _ := ret[0].(bool)

	return ret0
}

// IsSatellite indicates an expected call of IsSatellite.
func (mr *MockGraphMockRecorder) IsSatellite() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSatellite", reflect.TypeOf((*MockGraph)(nil).IsSatellite))
}

// IsSmart mocks base method.
func (m *MockGraph) IsSmart() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsSmart")
	ret0, _ := ret[0].(bool)

	return ret0
}

// IsSmart indicates an expected call of IsSmart.
func (mr *MockGraphMockRecorder) IsSmart() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsSmart", reflect.TypeOf((*MockGraph)(nil).IsSmart))
}

// Name mocks base method.
func (m *MockGraph) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)

	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockGraphMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockGraph)(nil).Name))
}

// NumberOfShards mocks base method.
func (m *MockGraph) NumberOfShards() *int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NumberOfShards")
	ret0, _ := ret[0].(*int)

	return ret0
}

// NumberOfShards indicates an expected call of NumberOfShards.
func (mr *MockGraphMockRecorder) NumberOfShards() *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NumberOfShards",
		reflect.TypeOf((*MockGraph)(nil).NumberOfShards))
}

// OrphanCollections mocks base method.
func (m *MockGraph) OrphanCollections() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OrphanCollections")
	ret0, _ := ret[0].([]string)

	return ret0
}

// OrphanCollections indicates an expected call of OrphanCollections.
func (mr *MockGraphMockRecorder) OrphanCollections() *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OrphanCollections",
		reflect.TypeOf((*MockGraph)(nil).OrphanCollections))
}

// Remove mocks base method.
func (m *MockGraph) Remove(ctx context.Context, opts *arangodb.RemoveGraphOptions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", ctx, opts)
	ret0, _ := ret[0].(error)

	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockGraphMockRecorder) Remove(ctx, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockGraph)(nil).Remove), ctx, opts)
}

// ReplaceEdgeDefinition mocks base method.
func (m *MockGraph) ReplaceEdgeDefinition(ctx context.Context, collection string, from, to []string,
	opts *arangodb.ReplaceEdgeOptions) (arangodb.ReplaceEdgeDefinitionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceEdgeDefinition", ctx, collection, from, to, opts)
	ret0, _ := ret[0].(arangodb.ReplaceEdgeDefinitionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// ReplaceEdgeDefinition indicates an expected call of ReplaceEdgeDefinition.
func (mr *MockGraphMockRecorder) ReplaceEdgeDefinition(ctx, collection, from, to, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceEdgeDefinition",
		reflect.TypeOf((*MockGraph)(nil).ReplaceEdgeDefinition), ctx, collection, from, to, opts)
}

// ReplicationFactor mocks base method.
func (m *MockGraph) ReplicationFactor() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplicationFactor")
	ret0, _ := ret[0].(int)

	return ret0
}

// ReplicationFactor indicates an expected call of ReplicationFactor.
func (mr *MockGraphMockRecorder) ReplicationFactor() *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplicationFactor",
		reflect.TypeOf((*MockGraph)(nil).ReplicationFactor))
}

// SmartGraphAttribute mocks base method.
func (m *MockGraph) SmartGraphAttribute() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SmartGraphAttribute")
	ret0, _ := ret[0].(string)

	return ret0
}

// SmartGraphAttribute indicates an expected call of SmartGraphAttribute.
func (mr *MockGraphMockRecorder) SmartGraphAttribute() *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SmartGraphAttribute",
		reflect.TypeOf((*MockGraph)(nil).SmartGraphAttribute))
}

// VertexCollection mocks base method.
func (m *MockGraph) VertexCollection(ctx context.Context, name string) (arangodb.VertexCollection, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VertexCollection", ctx, name)
	ret0, _ := ret[0].(arangodb.VertexCollection)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// VertexCollection indicates an expected call of VertexCollection.
func (mr *MockGraphMockRecorder) VertexCollection(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VertexCollection",
		reflect.TypeOf((*MockGraph)(nil).VertexCollection), ctx, name)
}

// VertexCollectionExists mocks base method.
func (m *MockGraph) VertexCollectionExists(ctx context.Context, name string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VertexCollectionExists", ctx, name)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// VertexCollectionExists indicates an expected call of VertexCollectionExists.
func (mr *MockGraphMockRecorder) VertexCollectionExists(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VertexCollectionExists",
		reflect.TypeOf((*MockGraph)(nil).VertexCollectionExists), ctx, name)
}

// VertexCollections mocks base method.
func (m *MockGraph) VertexCollections(ctx context.Context) ([]arangodb.VertexCollection, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VertexCollections", ctx)
	ret0, _ := ret[0].([]arangodb.VertexCollection)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// VertexCollections indicates an expected call of VertexCollections.
func (mr *MockGraphMockRecorder) VertexCollections(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VertexCollections",
		reflect.TypeOf((*MockGraph)(nil).VertexCollections), ctx)
}

// WriteConcern mocks base method.
func (m *MockGraph) WriteConcern() *int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteConcern")
	ret0, _ := ret[0].(*int)

	return ret0
}

// WriteConcern indicates an expected call of WriteConcern.
func (mr *MockGraphMockRecorder) WriteConcern() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteConcern", reflect.TypeOf((*MockGraph)(nil).WriteConcern))
}

// MockGraphVertexCollections is a mock of GraphVertexCollections interface.
type MockGraphVertexCollections struct {
	ctrl     *gomock.Controller
	recorder *MockGraphVertexCollectionsMockRecorder
}

// MockGraphVertexCollectionsMockRecorder is the mock recorder for MockGraphVertexCollections.
type MockGraphVertexCollectionsMockRecorder struct {
	mock *MockGraphVertexCollections
}

// NewMockGraphVertexCollections creates a new mock instance.
func NewMockGraphVertexCollections(ctrl *gomock.Controller) *MockGraphVertexCollections {
	mock := &MockGraphVertexCollections{ctrl: ctrl}
	mock.recorder = &MockGraphVertexCollectionsMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGraphVertexCollections) EXPECT() *MockGraphVertexCollectionsMockRecorder {
	return m.recorder
}

// CreateVertexCollection mocks base method.
func (m *MockGraphVertexCollections) CreateVertexCollection(ctx context.Context, name string,
	opts *arangodb.CreateVertexCollectionOptions) (arangodb.CreateVertexCollectionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateVertexCollection", ctx, name, opts)
	ret0, _ := ret[0].(arangodb.CreateVertexCollectionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// CreateVertexCollection indicates an expected call of CreateVertexCollection.
func (mr *MockGraphVertexCollectionsMockRecorder) CreateVertexCollection(ctx, name, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateVertexCollection",
		reflect.TypeOf((*MockGraphVertexCollections)(nil).CreateVertexCollection), ctx, name, opts)
}

// DeleteVertexCollection mocks base method.
func (m *MockGraphVertexCollections) DeleteVertexCollection(ctx context.Context, name string,
	opts *arangodb.DeleteVertexCollectionOptions) (arangodb.DeleteVertexCollectionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteVertexCollection", ctx, name, opts)
	ret0, _ := ret[0].(arangodb.DeleteVertexCollectionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// DeleteVertexCollection indicates an expected call of DeleteVertexCollection.
func (mr *MockGraphVertexCollectionsMockRecorder) DeleteVertexCollection(ctx, name, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock,
		"DeleteVertexCollection", reflect.TypeOf((*MockGraphVertexCollections)(nil).DeleteVertexCollection), ctx, name, opts)
}

// VertexCollection mocks base method.
func (m *MockGraphVertexCollections) VertexCollection(ctx context.Context, name string) (arangodb.VertexCollection, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VertexCollection", ctx, name)
	ret0, _ := ret[0].(arangodb.VertexCollection)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// VertexCollection indicates an expected call of VertexCollection.
func (mr *MockGraphVertexCollectionsMockRecorder) VertexCollection(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock,
		"VertexCollection", reflect.TypeOf((*MockGraphVertexCollections)(nil).VertexCollection), ctx, name)
}

// VertexCollectionExists mocks base method.
func (m *MockGraphVertexCollections) VertexCollectionExists(ctx context.Context, name string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VertexCollectionExists", ctx, name)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// VertexCollectionExists indicates an expected call of VertexCollectionExists.
func (mr *MockGraphVertexCollectionsMockRecorder) VertexCollectionExists(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock,
		"VertexCollectionExists", reflect.TypeOf((*MockGraphVertexCollections)(nil).VertexCollectionExists), ctx, name)
}

// VertexCollections mocks base method.
func (m *MockGraphVertexCollections) VertexCollections(ctx context.Context) ([]arangodb.VertexCollection, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VertexCollections", ctx)
	ret0, _ := ret[0].([]arangodb.VertexCollection)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// VertexCollections indicates an expected call of VertexCollections.
func (mr *MockGraphVertexCollectionsMockRecorder) VertexCollections(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VertexCollections",
		reflect.TypeOf((*MockGraphVertexCollections)(nil).VertexCollections), ctx)
}

// MockGraphEdgesDefinition is a mock of GraphEdgesDefinition interface.
type MockGraphEdgesDefinition struct {
	ctrl     *gomock.Controller
	recorder *MockGraphEdgesDefinitionMockRecorder
}

// MockGraphEdgesDefinitionMockRecorder is the mock recorder for MockGraphEdgesDefinition.
type MockGraphEdgesDefinitionMockRecorder struct {
	mock *MockGraphEdgesDefinition
}

// NewMockGraphEdgesDefinition creates a new mock instance.
func NewMockGraphEdgesDefinition(ctrl *gomock.Controller) *MockGraphEdgesDefinition {
	mock := &MockGraphEdgesDefinition{ctrl: ctrl}
	mock.recorder = &MockGraphEdgesDefinitionMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGraphEdgesDefinition) EXPECT() *MockGraphEdgesDefinitionMockRecorder {
	return m.recorder
}

// CreateEdgeDefinition mocks base method.
func (m *MockGraphEdgesDefinition) CreateEdgeDefinition(ctx context.Context, collection string, from, to []string,
	opts *arangodb.CreateEdgeDefinitionOptions) (arangodb.CreateEdgeDefinitionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateEdgeDefinition", ctx, collection, from, to, opts)
	ret0, _ := ret[0].(arangodb.CreateEdgeDefinitionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// CreateEdgeDefinition indicates an expected call of CreateEdgeDefinition.
func (mr *MockGraphEdgesDefinitionMockRecorder) CreateEdgeDefinition(ctx, collection, from, to, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateEdgeDefinition",
		reflect.TypeOf((*MockGraphEdgesDefinition)(nil).CreateEdgeDefinition), ctx, collection, from, to, opts)
}

// DeleteEdgeDefinition mocks base method.
func (m *MockGraphEdgesDefinition) DeleteEdgeDefinition(ctx context.Context, collection string,
	opts *arangodb.DeleteEdgeDefinitionOptions) (arangodb.DeleteEdgeDefinitionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteEdgeDefinition", ctx, collection, opts)
	ret0, _ := ret[0].(arangodb.DeleteEdgeDefinitionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// DeleteEdgeDefinition indicates an expected call of DeleteEdgeDefinition.
func (mr *MockGraphEdgesDefinitionMockRecorder) DeleteEdgeDefinition(ctx, collection, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteEdgeDefinition",
		reflect.TypeOf((*MockGraphEdgesDefinition)(nil).DeleteEdgeDefinition), ctx, collection, opts)
}

// EdgeDefinition mocks base method.
func (m *MockGraphEdgesDefinition) EdgeDefinition(ctx context.Context, collection string) (arangodb.Edge, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EdgeDefinition", ctx, collection)
	ret0, _ := ret[0].(arangodb.Edge)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// EdgeDefinition indicates an expected call of EdgeDefinition.
func (mr *MockGraphEdgesDefinitionMockRecorder) EdgeDefinition(ctx, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EdgeDefinition",
		reflect.TypeOf((*MockGraphEdgesDefinition)(nil).EdgeDefinition), ctx, collection)
}

// EdgeDefinitionExists mocks base method.
func (m *MockGraphEdgesDefinition) EdgeDefinitionExists(ctx context.Context, collection string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EdgeDefinitionExists", ctx, collection)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// EdgeDefinitionExists indicates an expected call of EdgeDefinitionExists.
func (mr *MockGraphEdgesDefinitionMockRecorder) EdgeDefinitionExists(ctx, collection any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EdgeDefinitionExists",
		reflect.TypeOf((*MockGraphEdgesDefinition)(nil).EdgeDefinitionExists), ctx, collection)
}

// GetEdgeDefinitions mocks base method.
func (m *MockGraphEdgesDefinition) GetEdgeDefinitions(ctx context.Context) ([]arangodb.Edge, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEdgeDefinitions", ctx)
	ret0, _ := ret[0].([]arangodb.Edge)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// GetEdgeDefinitions indicates an expected call of GetEdgeDefinitions.
func (mr *MockGraphEdgesDefinitionMockRecorder) GetEdgeDefinitions(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEdgeDefinitions",
		reflect.TypeOf((*MockGraphEdgesDefinition)(nil).GetEdgeDefinitions), ctx)
}

// ReplaceEdgeDefinition mocks base method.
func (m *MockGraphEdgesDefinition) ReplaceEdgeDefinition(ctx context.Context, collection string,
	from, to []string, opts *arangodb.ReplaceEdgeOptions) (arangodb.ReplaceEdgeDefinitionResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReplaceEdgeDefinition", ctx, collection, from, to, opts)
	ret0, _ := ret[0].(arangodb.ReplaceEdgeDefinitionResponse)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// ReplaceEdgeDefinition indicates an expected call of ReplaceEdgeDefinition.
func (mr *MockGraphEdgesDefinitionMockRecorder) ReplaceEdgeDefinition(ctx, collection, from, to, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReplaceEdgeDefinition",
		reflect.TypeOf((*MockGraphEdgesDefinition)(nil).ReplaceEdgeDefinition), ctx, collection, from, to, opts)
}
