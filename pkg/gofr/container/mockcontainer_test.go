package container

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/sql"
)

func Test_HttpServiceMock(t *testing.T) {
	test := struct {
		desc        string
		path        string
		statusCode  int
		expectedRes string
	}{

		desc:        "simple service handler",
		path:        "/fact",
		expectedRes: `{"data":{"fact":"Cats have 3 eyelids.","length":20}}` + "\n",
		statusCode:  200,
	}

	httpservices := []string{"cat-facts", "cat-facts1", "cat-facts2"}

	_, mock := NewMockContainer(t, WithMockHTTPService(httpservices...))

	res := httptest.NewRecorder()
	res.Body = bytes.NewBufferString(`{"fact":"Cats have 3 eyelids.","length":20}` + "\n")
	res.Code = test.statusCode
	result := res.Result()

	// Setting mock expectations
	mock.HTTPService.EXPECT().Get(t.Context(), "fact", map[string]any{
		"max_length": 20,
	}).Return(result, nil)

	resp, err := mock.HTTPService.Get(t.Context(), "fact", map[string]any{
		"max_length": 20,
	})

	require.NoError(t, err)
	assert.Equal(t, resp, result)

	err = result.Body.Close()
	require.NoError(t, err)

	err = resp.Body.Close()
	require.NoError(t, err)
}

// Test_HttpServiceMockWithServiceName verifies that WithMockHTTPService works correctly.
// when service names are provided, and that mocks.HTTPService matches the service in container.
func Test_HttpServiceMockWithServiceName(t *testing.T) {
	serviceName := "test-service"
	container, mocks := NewMockContainer(t, WithMockHTTPService(serviceName))

	// Verify that the service is registered in the container
	serviceFromContainer := container.GetHTTPService(serviceName)
	require.NotNil(t, serviceFromContainer, "Service should be registered in container")

	// Verify the service is in the HTTPServices map
	mock, exists := mocks.HTTPServices[serviceName]
	require.True(t, exists, "Service should be in HTTPServices map")
	assert.Equal(t, mock, serviceFromContainer, "Service from container should match the mock in HTTPServices map")

	// Verify backward compatibility: mocks.HTTPService should be the same as the service mock
	assert.Equal(t, mocks.HTTPService, serviceFromContainer,
		"mocks.HTTPService (backward compatibility) should be the same instance as container.Services[serviceName]")
	assert.Equal(t, mocks.HTTPService, mock,
		"mocks.HTTPService should be the same as the mock in HTTPServices map")

	// Test that we can set expectations on mocks.HTTPService and they work for the service in container
	mockResp := httptest.NewRecorder()
	mockResp.Body = bytes.NewBufferString(`{"data":"test"}`)
	mockResp.Code = 200
	result := mockResp.Result()

	// Set expectation on mocks.HTTPService (backward compatibility)
	mocks.HTTPService.EXPECT().Get(
		gomock.Any(), // Use gomock.Any() for context to avoid context mismatch
		"test-path",
		gomock.Any(), // Use gomock.Any() for queryParams
	).Return(result, nil)

	// Call the service from container - should match the expectation
	resp, err := serviceFromContainer.Get(context.Background(), "test-path", map[string]any{
		"key": "value",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	err = result.Body.Close()
	require.NoError(t, err)

	err = resp.Body.Close()
	require.NoError(t, err)
}

// Test_HttpServiceMockMultipleServices verifies that multiple services have separate mock instances.
func Test_HttpServiceMockMultipleServices(t *testing.T) {
	serviceNames := []string{"service1", "service2", "service3"}
	container, mocks := NewMockContainer(t, WithMockHTTPService(serviceNames...))

	// Verify all services are registered and have separate mock instances
	for _, name := range serviceNames {
		service := container.GetHTTPService(name)
		require.NotNil(t, service, "Service %s should be registered", name)
		
		// Verify the service is in the HTTPServices map
		mock, exists := mocks.HTTPServices[name]
		require.True(t, exists, "Service %s should be in HTTPServices map", name)
		assert.Equal(t, mock, service, "Service %s should match the mock in HTTPServices map", name)
		
		// Verify each service has a different mock instance (use pointer comparison)
		for _, otherName := range serviceNames {
			if name != otherName {
				otherMock := mocks.HTTPServices[otherName]
				// Use fmt.Sprintf to compare pointers, as assert.NotEqual might do value comparison
				mockPtr := fmt.Sprintf("%p", mock)
				otherMockPtr := fmt.Sprintf("%p", otherMock)
				assert.NotEqual(t, mockPtr, otherMockPtr, "Service %s and %s should have different mock instances (pointers)", name, otherName)
			}
		}
	}

	// Test that different services can have different expectations
	mockResp1 := httptest.NewRecorder()
	mockResp1.Body = bytes.NewBufferString(`{"data":"service1"}`)
	mockResp1.Code = 200
	result1 := mockResp1.Result()

	mockResp2 := httptest.NewRecorder()
	mockResp2.Body = bytes.NewBufferString(`{"data":"service2"}`)
	mockResp2.Code = 200
	result2 := mockResp2.Result()

	mockResp3 := httptest.NewRecorder()
	mockResp3.Body = bytes.NewBufferString(`{"data":"service3"}`)
	mockResp3.Code = 200
	result3 := mockResp3.Result()

	// Set different expectations for each service
	mocks.HTTPServices["service1"].EXPECT().Get(
		gomock.Any(),
		"/service1/path",
		gomock.Any(),
	).Return(result1, nil)

	mocks.HTTPServices["service2"].EXPECT().Get(
		gomock.Any(),
		"/service2/path",
		gomock.Any(),
	).Return(result2, nil)

	mocks.HTTPServices["service3"].EXPECT().Get(
		gomock.Any(),
		"/service3/path",
		gomock.Any(),
	).Return(result3, nil)

	// Call each service with their specific paths
	service1 := container.GetHTTPService("service1")
	resp1, err1 := service1.Get(context.Background(), "/service1/path", map[string]any{})
	require.NoError(t, err1)
	assert.NotNil(t, resp1)
	assert.Equal(t, 200, resp1.StatusCode)
	resp1.Body.Close()

	service2 := container.GetHTTPService("service2")
	resp2, err2 := service2.Get(context.Background(), "/service2/path", map[string]any{})
	require.NoError(t, err2)
	assert.NotNil(t, resp2)
	assert.Equal(t, 200, resp2.StatusCode)
	resp2.Body.Close()

	service3 := container.GetHTTPService("service3")
	resp3, err3 := service3.Get(context.Background(), "/service3/path", map[string]any{})
	require.NoError(t, err3)
	assert.NotNil(t, resp3)
	assert.Equal(t, 200, resp3.StatusCode)
	resp3.Body.Close()

	result1.Body.Close()
	result2.Body.Close()
	result3.Body.Close()
}

func TestExpectSelect_ValidCases(t *testing.T) {
	mockContainer, mock := NewMockContainer(t)

	t.Run("Test with string slice", func(t *testing.T) {
		var passedResultSlice, actualResultSlice []string

		expectedIDs := []string{"1", "2"}

		mock.SQL.ExpectSelect(t.Context(), &passedResultSlice, "SELECT id FROM users").ReturnsResponse(expectedIDs)

		mockContainer.SQL.Select(t.Context(), &actualResultSlice, "SELECT id FROM users")
		require.Equal(t, expectedIDs, actualResultSlice)
	})

	t.Run("Test with string slice with multiple expectations", func(t *testing.T) {
		var passedResultSlice, actualResultSlice, actualResultSlice2 []string

		expectedIDs := []string{"1", "2"}
		expectedIDs2 := []string{"1", "3"}

		mock.SQL.ExpectSelect(t.Context(), &passedResultSlice, "SELECT id FROM users").ReturnsResponse(expectedIDs)
		mock.SQL.ExpectSelect(t.Context(), &passedResultSlice, "SELECT id FROM users").ReturnsResponse(expectedIDs2)

		mockContainer.SQL.Select(t.Context(), &actualResultSlice, "SELECT id FROM users")
		mockContainer.SQL.Select(t.Context(), &actualResultSlice2, "SELECT id FROM users")

		require.Equal(t, expectedIDs, actualResultSlice)
		require.Equal(t, expectedIDs2, actualResultSlice2)
	})

	t.Run("Test with struct", func(t *testing.T) {
		type User struct {
			ID   int
			Name string
		}

		var passedUser, actualUser User

		expectedUser := User{ID: 1, Name: "John"}

		mock.SQL.ExpectSelect(t.Context(), &passedUser, "SELECT * FROM users WHERE id = ?", 1).ReturnsResponse(expectedUser)

		mockContainer.SQL.Select(t.Context(), &actualUser, "SELECT * FROM users WHERE id = ?", 1)
		require.Equal(t, expectedUser, actualUser)
	})

	t.Run("Test with map", func(t *testing.T) {
		var passedSettings, actualSettings map[string]int

		expectedSettings := map[string]int{"a": 1, "b": 2}

		mock.SQL.ExpectSelect(t.Context(), &passedSettings, "SELECT * FROM settings").ReturnsResponse(expectedSettings)

		mockContainer.SQL.Select(t.Context(), &actualSettings, "SELECT * FROM settings")
		require.Equal(t, expectedSettings, actualSettings)
	})
}

func TestExpectSelect_ErrorCases(t *testing.T) {
	mockDB, sqlMock, _ := sql.NewSQLMocks(t)
	ctrl := gomock.NewController(t)
	expectation := expectedQuery{}
	mockLogger := NewMockLogger(ctrl)
	sqlMockWrapper := &mockSQL{sqlMock, &expectation}
	sqlDB := &sqlMockDB{mockDB, &expectation, mockLogger}
	sqlDB.finish(t)

	t.Run("NonPointer_Value_In_ExpectSelect", func(t *testing.T) {
		mockLogger.EXPECT().Errorf(gomock.Any(), gomock.Any())

		var uninitializedVal, resultVal int

		expectedVal := 123

		sqlMockWrapper.ExpectSelect(t.Context(), uninitializedVal, "SELECT * FROM test WHERE id=?", 1).ReturnsResponse(expectedVal)

		sqlDB.Select(t.Context(), &resultVal, "SELECT * FROM test WHERE id=?", 1)
		assert.Zero(t, resultVal)
	})

	t.Run("PointerValue_In_ReturnsResponse", func(t *testing.T) {
		mockLogger.EXPECT().Errorf("received different expectations: %q", gomock.Any())

		var uninitializedVal, resultVal int

		expectedVal := 123

		sqlMockWrapper.ExpectSelect(t.Context(), &uninitializedVal, "SELECT * FROM test WHERE id=?", 1).ReturnsResponse(&expectedVal)

		sqlDB.Select(t.Context(), &resultVal, "SELECT * FROM test WHERE id=?", 1)
		assert.Zero(t, resultVal)
	})

	t.Run("Type_Mismatch_Between_Expect_And_Response", func(t *testing.T) {
		mockLogger.EXPECT().Errorf("received different expectations: %q", gomock.Any())

		var expectedVal, resultVal []string

		sqlMockWrapper.ExpectSelect(t.Context(), &expectedVal, "SELECT * FROM test WHERE id=?", 1).ReturnsResponse(123)

		sqlDB.Select(t.Context(), &resultVal, "SELECT * FROM test WHERE id=?", 1)
		assert.Empty(t, resultVal)
	})

	t.Run("Select_Called_Without_Expectations", func(t *testing.T) {
		mockLogger.EXPECT().Errorf("did not expect any calls for Select with query: %q", gomock.Any())

		var val []string

		sqlDB.Select(t.Context(), &val, "SELECT * FROM test WHERE id=?", 1)
		assert.Empty(t, val)
	})
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
		Details: map[string]any{"uptime": 1234567}}

	mock.SQL.ExpectHealthCheck().WillReturnHealthCheck(expectedHealth)

	resultHealth := mockContainer.SQL.HealthCheck()

	assert.Equal(t, expectedHealth, resultHealth)
}
