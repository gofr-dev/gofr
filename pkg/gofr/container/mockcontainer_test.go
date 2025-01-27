package container

import (
	"bytes"
	"context"
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
	mock.HTTPService.EXPECT().Get(context.Background(), "fact", map[string]any{
		"max_length": 20,
	}).Return(result, nil)

	resp, err := mock.HTTPService.Get(context.Background(), "fact", map[string]any{
		"max_length": 20,
	})

	require.NoError(t, err)
	assert.Equal(t, resp, result)

	err = result.Body.Close()
	require.NoError(t, err)

	err = resp.Body.Close()
	require.NoError(t, err)
}

func TestExpectSelect_ValidCases(t *testing.T) {
	mockContainer, mock := NewMockContainer(t)

	t.Run("Test with string slice", func(t *testing.T) {
		var passedResultSlice, actualResultSlice []string

		expectedIDs := []string{"1", "2"}

		mock.SQL.ExpectSelect(context.Background(), &passedResultSlice, "SELECT id FROM users").ReturnsResponse(expectedIDs)

		mockContainer.SQL.Select(context.Background(), &actualResultSlice, "SELECT id FROM users")
		require.Equal(t, expectedIDs, actualResultSlice)
	})

	t.Run("Test with string slice with multiple expectations", func(t *testing.T) {
		var passedResultSlice, actualResultSlice, actualResultSlice2 []string

		expectedIDs := []string{"1", "2"}
		expectedIDs2 := []string{"1", "3"}

		mock.SQL.ExpectSelect(context.Background(), &passedResultSlice, "SELECT id FROM users").ReturnsResponse(expectedIDs)
		mock.SQL.ExpectSelect(context.Background(), &passedResultSlice, "SELECT id FROM users").ReturnsResponse(expectedIDs2)

		mockContainer.SQL.Select(context.Background(), &actualResultSlice, "SELECT id FROM users")
		mockContainer.SQL.Select(context.Background(), &actualResultSlice2, "SELECT id FROM users")

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

		mock.SQL.ExpectSelect(context.Background(), &passedUser, "SELECT * FROM users WHERE id = ?", 1).ReturnsResponse(expectedUser)

		mockContainer.SQL.Select(context.Background(), &actualUser, "SELECT * FROM users WHERE id = ?", 1)
		require.Equal(t, expectedUser, actualUser)
	})

	t.Run("Test with map", func(t *testing.T) {
		var passedSettings, actualSettings map[string]int

		expectedSettings := map[string]int{"a": 1, "b": 2}

		mock.SQL.ExpectSelect(context.Background(), &passedSettings, "SELECT * FROM settings").ReturnsResponse(expectedSettings)

		mockContainer.SQL.Select(context.Background(), &actualSettings, "SELECT * FROM settings")
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

		sqlMockWrapper.ExpectSelect(context.Background(), uninitializedVal, "SELECT * FROM test WHERE id=?", 1).ReturnsResponse(expectedVal)

		sqlDB.Select(context.Background(), &resultVal, "SELECT * FROM test WHERE id=?", 1)
		assert.Zero(t, resultVal)
	})

	t.Run("PointerValue_In_ReturnsResponse", func(t *testing.T) {
		mockLogger.EXPECT().Errorf("received different expectations: %q", gomock.Any())

		var uninitializedVal, resultVal int

		expectedVal := 123

		sqlMockWrapper.ExpectSelect(context.Background(), &uninitializedVal, "SELECT * FROM test WHERE id=?", 1).ReturnsResponse(&expectedVal)

		sqlDB.Select(context.Background(), &resultVal, "SELECT * FROM test WHERE id=?", 1)
		assert.Zero(t, resultVal)
	})

	t.Run("Type_Mismatch_Between_Expect_And_Response", func(t *testing.T) {
		mockLogger.EXPECT().Errorf("received different expectations: %q", gomock.Any())

		var expectedVal, resultVal []string

		sqlMockWrapper.ExpectSelect(context.Background(), &expectedVal, "SELECT * FROM test WHERE id=?", 1).ReturnsResponse(123)

		sqlDB.Select(context.Background(), &resultVal, "SELECT * FROM test WHERE id=?", 1)
		assert.Empty(t, resultVal)
	})

	t.Run("Select_Called_Without_Expectations", func(t *testing.T) {
		mockLogger.EXPECT().Errorf("did not expect any calls for Select with query: %q", gomock.Any())

		var val []string

		sqlDB.Select(context.Background(), &val, "SELECT * FROM test WHERE id=?", 1)
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
