package arangodb

import driver "github.com/arangodb/go-driver/v2/arangodb"

// The mocks in this package are generated from go-driver's own interfaces, but nothing in the
// package assigns one to the interface it mocks — the driver types reach this code only through
// gofr's local interfaces in interface.go, so `go build` and `go vet` stay green when a mock and
// the driver drift apart. mock_collection.go carried a v2.1.6 shape while go.mod was on v2.3.1
// and was missing Collection.EnsureVectorIndex, which only surfaced as a gomock failure inside
// the tests that used it (#3824).
//
// These assertions cost nothing at runtime and turn the next driver bump into a build failure
// naming the mock that needs regenerating. MockDatabaseGraph is absent deliberately: it mocks a
// gofr-local interface, not a driver one.
var (
	_ driver.Client                 = (*MockClient)(nil)
	_ driver.Database               = (*MockDatabase)(nil)
	_ driver.Collection             = (*MockCollection)(nil)
	_ driver.User                   = (*MockUser)(nil)
	_ driver.Graph                  = (*MockGraph)(nil)
	_ driver.GraphVertexCollections = (*MockGraphVertexCollections)(nil)
	_ driver.GraphEdgesDefinition   = (*MockGraphEdgesDefinition)(nil)
	_ driver.GraphsResponseReader   = (*MockGraphsResponseReader)(nil)
)
