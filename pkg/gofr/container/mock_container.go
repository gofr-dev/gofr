package container

import (
	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"
	"gofr.dev/pkg/gofr/logging"
	"testing"
)

type MockContainer struct {
	Logger logging.Logger

	appName    string
	appVersion string

	//Redis *redis.Redis
	SQL     DBInterface
	SQLMock sqlmock.Sqlmock
}

func NewMockContainer(t *testing.T) MockContainer {
	container := MockContainer{}
	container.SQL = NewMockDBInterface(gomock.NewController(t))
	container.Logger = logging.NewLogger(logging.DEBUG)

	_, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("SQL mock not initialized %v", err)
	}

	container.SQLMock = mock

	return container
}
