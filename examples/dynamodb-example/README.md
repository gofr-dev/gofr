# DynamoDB Integration with GoFr

This example demonstrates the integration of DynamoDB as a key-value store in GoFr using Uber mocks for testing.

## Implementation Summary

### 1. Documentation Added
- **File**: `docs/datasources/dynamodb/page.md`
- **Content**: Complete documentation with interface definition, examples, configuration options, and local development setup

### 2. Docker Container Added
- **File**: `CONTRIBUTING.md`
- **Addition**: DynamoDB Local container command for development
```bash
docker run --name dynamodb-local -d -p 8000:8000 amazon/dynamodb-local
```

### 3. GoFr Integration Files Modified

#### `pkg/gofr/container/datasources.go`
- Added `DynamoDB` interface with Get, Set, Delete, and HealthCheck methods
- Added `DynamoDBProvider` interface extending DynamoDB with provider methods

#### `pkg/gofr/container/container.go`
- Added `DynamoDB DynamoDB` field to the Container struct

#### `pkg/gofr/external_db.go`
- Added `AddDynamoDB(db container.DynamoDBProvider)` method
- Integrated logging, metrics, and tracing support

#### `pkg/gofr/container/mock_container.go`
- Added DynamoDB mock to the Mocks struct
- Integrated DynamoDB mock initialization in NewMockContainer

### 4. Uber Mocks Implementation

The implementation uses Uber mocks (go.uber.org/mock/mockgen) for comprehensive testing:

#### Mock Generation
- Mocks are automatically generated using `go generate` in the container package
- Generated file: `mock_datasources.go` contains `MockDynamoDB` and `MockDynamoDBMockRecorder`

#### Test Examples
- **File**: `mock_test.go`
- **Tests**:
  - `TestDynamoDBMockOperations`: Demonstrates successful operations (Set, Get, Delete, HealthCheck)
  - `TestDynamoDBMockErrorHandling`: Demonstrates error scenarios

## Test Results

### Successful Operations Test
```
=== RUN   TestDynamoDBMockOperations
    mock_test.go:90: ✅ All DynamoDB operations completed successfully with Uber mocks
    mock_test.go:91: 📊 Get result: map[created:1234567890 email:john@example.com name:John Doe timestamp:2023-01-01T00:00:00Z]
    mock_test.go:92: 🏥 Health check result: map[details:map[region:us-east-1 table:gofr-test-table] status:UP]
--- PASS: TestDynamoDBMockOperations (0.00s)
```

### Error Handling Test
```
=== RUN   TestDynamoDBMockErrorHandling
    mock_test.go:128: ✅ Error handling tests completed successfully with Uber mocks
    mock_test.go:129: ❌ Get error: context deadline exceeded
--- PASS: TestDynamoDBMockErrorHandling (0.00s)
```

## Key Features

### 1. Interface-Driven Design
- Clean separation between interface definition and implementation
- Easy to mock and test
- Follows GoFr's established patterns

### 2. Observability Support
- **Logging**: Integrated with GoFr's logging system
- **Metrics**: Histogram metrics for operation duration
- **Tracing**: OpenTelemetry tracing support

### 3. Uber Mocks Integration
- Automatic mock generation using `go generate`
- Comprehensive test coverage
- Error scenario testing
- Easy to extend and maintain

### 4. Local Development Support
- Docker container for local DynamoDB
- Configurable endpoints
- Health check functionality

## Usage Example

```go
// Create DynamoDB client
db := dynamodb.New(dynamodb.Configs{
    Table:            "my-table",
    Region:           "us-east-1",
    Endpoint:         "http://localhost:8000", // Local DynamoDB
    PartitionKeyName: "pk",
})

// Add to GoFr app
app.AddDynamoDB(db)

// Use in handlers
func MyHandler(ctx *gofr.Context) (any, error) {
    // Set data
    err := ctx.DynamoDB.Set(ctx, "key", map[string]any{"value": "data"})
    
    // Get data
    result, err := ctx.DynamoDB.Get(ctx, "key")
    
    // Delete data
    err = ctx.DynamoDB.Delete(ctx, "key")
    
    // Health check
    health, err := ctx.DynamoDB.HealthCheck(ctx)
    
    return result, nil
}
```

## Testing with Uber Mocks

```go
func TestDynamoDBOperations(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    _, mocks := container.NewMockContainer(t)
    
    // Set up expectations
    mocks.DynamoDB.EXPECT().
        Set(gomock.Any(), "test-key", gomock.Any()).
        Return(nil).
        Times(1)
    
    // Test operations
    err := mocks.DynamoDB.Set(ctx, "test-key", data)
    // ... test assertions
}
```

## Files Modified/Created

1. `docs/datasources/dynamodb/page.md` - Documentation
2. `CONTRIBUTING.md` - Docker container addition
3. `pkg/gofr/container/datasources.go` - Interface definitions
4. `pkg/gofr/container/container.go` - Container struct
5. `pkg/gofr/external_db.go` - AddDynamoDB method
6. `pkg/gofr/container/mock_container.go` - Mock integration
7. `pkg/gofr/container/mock_datasources.go` - Generated mocks
8. `examples/dynamodb-example/mock_test.go` - Test examples

## Next Steps

1. **Integration Testing**: Test with real DynamoDB Local instance
2. **Performance Testing**: Benchmark operations with metrics
3. **Documentation**: Add to main GoFr documentation site
4. **Examples**: Create more comprehensive usage examples
5. **CI/CD**: Add DynamoDB tests to CI pipeline

The implementation follows GoFr's established patterns and provides a solid foundation for DynamoDB integration with comprehensive testing using Uber mocks.

