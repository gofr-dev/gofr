package container

import (
	"testing"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

type Mocks struct {
	Redis *MockRedis
	SQL   *MockDB
}

func NewMockContainer(t *testing.T) (*Container, Mocks) {
	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	sqlMock := NewMockDB(gomock.NewController(t))
	container.SQL = sqlMock

	redisMock := NewMockRedis(gomock.NewController(t))
	container.Redis = redisMock

	mocks := Mocks{Redis: redisMock, SQL: sqlMock}

	return container, mocks
}
