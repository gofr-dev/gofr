package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
)

// TestMockSQL_Select tests the successful operation of SQL mocking for SELECT statements.
// It checks that the mock expectations are correctly set and that the SQL database function
// is called as expected.
//
// Additional test scenarios to consider:
// 1. Missing Initialization of Mock Expectations**:
//   - This can be tested by commenting out the `ExpectSelect` call.
//
// 2. Missing Call to SQL Function:
//   - This can be tested by commenting out the actual SQL database function call.
//
// Note: Both scenarios mentioned above will trigger a fatal error that terminates the process.
// Explicit tests for these scenarios are not included because they result in an abrupt process
// termination, which is handled by the fatal function.
func TestMockSQL_Select(t *testing.T) {
	ids := []string{"1", "2"}

	mockContainer, mock := NewMockContainer(t)

	mock.SQL.ExpectSelect(context.Background(), ids, "select quantity from items where id =?", 123)
	mock.SQL.ExpectSelect(context.Background(), ids, "select quantity from items where id =?", 132)

	mockContainer.SQL.Select(context.Background(), &ids, "select quantity from items where id =?", 123)
	mockContainer.SQL.Select(context.Background(), &ids, "select quantity from items where id =?", 132)
}

func TestMockSQL_Dialect(t *testing.T) {
	mockContainer, mock := NewMockContainer(t)

	mock.SQL.ExpectDialect().WillReturnString("abcd")

	h := mockContainer.SQL.Dialect()

	assert.Equal(t, "abcd", h)
}

func TestMockSQL_HealthCheck(t *testing.T) {
	mockContainer, mock := NewMockContainer(t)

	expectedHealth := &datasource.Health{
		Status:  "up",
		Details: map[string]interface{}{"uptime": 1234567}}

	mock.SQL.ExpectHealthCheck().WillReturnHealthCheck(expectedHealth)

	resultHealth := mockContainer.SQL.HealthCheck()

	assert.Equal(t, expectedHealth, resultHealth)
}
