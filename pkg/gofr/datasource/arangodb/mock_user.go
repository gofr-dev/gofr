package arangodb

import (
	"context"

	"github.com/arangodb/go-driver/v2/arangodb"
	"go.uber.org/mock/gomock"
)

// MockUser implements the complete arangodb.user interface.
type MockUser struct {
	ctrl   *gomock.Controller
	name   string
	active bool
}

func NewMockUser(ctrl *gomock.Controller) *MockUser {
	return &MockUser{
		ctrl:   ctrl,
		name:   "testUser",
		active: true,
	}
}

func (m *MockUser) Name() string   { return m.name }
func (m *MockUser) IsActive() bool { return m.active }
func (*MockUser) Extra(any) error  { return nil }

func (*MockUser) AccessibleDatabases(context.Context) (map[string]arangodb.Grant, error) {
	return nil, nil
}

func (*MockUser) AccessibleDatabasesFull(context.Context) (map[string]arangodb.DatabasePermissions, error) {
	return nil, nil
}

func (*MockUser) GetDatabaseAccess(context.Context, string) (arangodb.Grant, error) {
	return arangodb.GrantNone, nil
}

func (*MockUser) GetCollectionAccess(context.Context, string, string) (arangodb.Grant, error) {
	return arangodb.GrantNone, nil
}

func (*MockUser) SetDatabaseAccess(context.Context, string, arangodb.Grant) error {
	return nil
}

func (*MockUser) SetCollectionAccess(context.Context, string, string, arangodb.Grant) error {
	return nil
}

func (*MockUser) RemoveDatabaseAccess(context.Context, string) error {
	return nil
}

func (*MockUser) RemoveCollectionAccess(context.Context, string, string) error {
	return nil
}
