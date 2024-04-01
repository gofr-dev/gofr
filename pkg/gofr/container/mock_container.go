package container

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v9"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

type Mocks struct {
	Redis redismock.ClientMock
	SQL   sqlmock.Sqlmock
}

func NewMockContainer(t *testing.T) (*Container, Mocks) {
	container := &Container{}
	container.Logger = logging.NewLogger(logging.DEBUG)

	container.SQL = NewMockDBInterface(gomock.NewController(t))
	container.Redis = NewMockRedisInterface(gomock.NewController(t))

	_, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("SQL mock not initialized %v", err)
	}

	_, redisMock := redismock.NewClientMock()

	mocks := Mocks{redisMock, sqlMock}

	return container, mocks
}
