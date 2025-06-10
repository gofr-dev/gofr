package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/container"
)

func pubsubTestSetup(t *testing.T) (migrator, *container.MockPubSubProvider, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	ds := Datasource{PubSub: mockContainer.PubSub}

	pubsubDB := pubsubDS{client: mockContainer.PubSub}
	migratorWithPubSub := pubsubDB.apply(&ds)

	mockContainer.PubSub = mocks.PubSub

	return migratorWithPubSub, mocks.PubSub, mockContainer
}

func Test_PubSubCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithPubSub, mockPubSub, mockContainer := pubsubTestSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"topic already exists", nil},
	}

	for i, tc := range testCases {
		mockPubSub.EXPECT().CreateTopic(context.Background(), pubsubMigrationTopic).Return(tc.err)

		err := migratorWithPubSub.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}
