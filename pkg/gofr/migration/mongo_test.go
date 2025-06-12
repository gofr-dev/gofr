package migration

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
		mockMongo.EXPECT().CreateCollection(gomock.Any(), mongoMigrationCollection).Return(tc.err)
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

	filter := make(map[string]any)

	for i, tc := range testCases {
		mockMongo.EXPECT().Find(gomock.Any(), mongoMigrationCollection, filter, &migrations).Return(tc.err)

		resp := migratorWithMongo.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_MongoCommitMigration(t *testing.T) {
	migratorWithMongo, mockMongo, mockContainer := mongoSetup(t)

	// mockResult is not the same result type as that returned by InsertOne method in mongoDB,
	// but has been used only for mocking the test for migrations in mongoDB.
	mockResult := struct{}{}

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", errMongoConn},
	}

	timeNow := time.Now()

	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}

	migrationDoc := map[string]any{
		"version":    td.MigrationNumber,
		"method":     "UP",
		"start_time": td.StartTime,
		"duration":   time.Since(td.StartTime).Milliseconds(),
	}

	for i, tc := range testCases {
		mockMongo.EXPECT().InsertOne(gomock.Any(), mongoMigrationCollection, migrationDoc).Return(mockResult, tc.err)

		err := migratorWithMongo.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}
