package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/container"
)

var errMongoConn = errors.New("error connecting to mongo")

func mongoSetup(t *testing.T) (migrator, *container.MockMongo, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockMongo := mocks.Mongo

	ds := Datasource{Mongo: mockContainer.Mongo}

	mongoDB := mongoDS{Mongo: mockMongo}
	migratorWithMongo := mongoDB.apply(&ds)

	mockContainer.Mongo = mockMongo

	return migratorWithMongo, mockMongo, mockContainer
}

func Test_MongoCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithMongo, mockMongo, mockContainer := mongoSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", errMongoConn},
	}

	for i, tc := range testCases {
		mockMongo.EXPECT().CreateCollection(context.Background(), mongoMigrationCollection).Return(tc.err)
		err := migratorWithMongo.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_MongoGetLastMigration(t *testing.T) {
	migratorWithMongo, mockMongo, mockContainer := mongoSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 0},
		{"connection failed", errMongoConn, 0},
	}

	var migrations []struct {
		Version int64 `bson:"version"`
	}

	filter := map[string]interface{}{}

	for i, tc := range testCases {
		mockMongo.EXPECT().Find(context.Background(), mongoMigrationCollection, filter, &migrations).Return(tc.err)

		resp := migratorWithMongo.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}
