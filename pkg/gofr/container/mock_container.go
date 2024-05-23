package container

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
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

type MockPubSub struct {
}

func (m *MockPubSub) CreateTopic(_ context.Context, _ string) error {
	return nil
}

func (m *MockPubSub) DeleteTopic(_ context.Context, _ string) error {
	return nil
}

func (m *MockPubSub) Health() datasource.Health {
	return datasource.Health{}
}

func (m *MockPubSub) Publish(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (m *MockPubSub) Subscribe(_ context.Context, _ string) (*pubsub.Message, error) {
	return nil, nil
}
