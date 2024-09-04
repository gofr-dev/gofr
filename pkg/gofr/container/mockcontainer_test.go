package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/datasource"
)

func TestMockSQL_Select(t *testing.T) {
	ids := make([]string, 0)
	ids = append(ids, "1", "2")

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
