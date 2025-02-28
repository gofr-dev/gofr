package migration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func arangoSetup(t *testing.T) (migrator, *container.MockArangoDBProvider, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)

	mockArango := mocks.ArangoDB

	ds := Datasource{ArangoDB: mockContainer.ArangoDB}

	arangoDB := arangoDS{client: mockArango}
	migratorWithArango := arangoDB.apply(&ds)

	mockContainer.ArangoDB = mockArango

	return migratorWithArango, mockArango, mockContainer
}

func Test_ArangoCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithArango, mockArango, mockContainer := arangoSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"collection already exists", nil},
	}

	for i, tc := range testCases {
		mockArango.EXPECT().CreateCollection(context.Background(), arangoMigrationDB, arangoMigrationCollection, false).Return(tc.err)

		err := migratorWithArango.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ArangoGetLastMigration(t *testing.T) {
	migratorWithArango, mockArango, mockContainer := arangoSetup(t)

	testCases := []struct {
		desc string
		err  error
		resp int64
	}{
		{"no error", nil, 0},
		{"query failed", context.DeadlineExceeded, 0},
	}

	var lastMigrations []int64

	for i, tc := range testCases {
		mockArango.EXPECT().Query(context.Background(), arangoMigrationDB, getLastArangoMigration, nil, &lastMigrations).Return(tc.err)

		resp := migratorWithArango.getLastMigration(mockContainer)

		assert.Equal(t, tc.resp, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ArangoCommitMigration(t *testing.T) {
	migratorWithArango, mockArango, mockContainer := arangoSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"insert failed", context.DeadlineExceeded},
	}

	timeNow := time.Now()

	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
	}

	for i, tc := range testCases {
		bindVars := map[string]any{
			"version":    td.MigrationNumber,
			"method":     "UP",
			"start_time": td.StartTime,
			"duration":   time.Since(td.StartTime).Milliseconds(),
		}

		mockArango.EXPECT().Query(context.Background(), arangoMigrationDB, insertArangoMigrationRecord, bindVars, gomock.Any()).Return(tc.err)

		err := migratorWithArango.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ArangoBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		migratorWithArango, _, mockContainer := arangoSetup(t)
		migratorWithArango.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "ArangoDB migrator begin successfully")
}

func Test_ArangoMigration_CreateDB(t *testing.T) {
	_, mockArango, _ := arangoSetup(t)

	arangoDB := arangoDS{client: mockArango}

	mockArango.EXPECT().CreateDB(context.Background(), "test").Return(nil)

	err := arangoDB.CreateDB(context.Background(), "test")

	assert.NoError(t, err, "Test_ArangoMigration_CreateDB failed!")
}

func Test_ArangoMigration_DropDB(t *testing.T) {
	_, mockArango, _ := arangoSetup(t)

	arangoDB := arangoDS{client: mockArango}

	mockArango.EXPECT().DropDB(context.Background(), "test").Return(nil)

	err := arangoDB.DropDB(context.Background(), "test")

	assert.NoError(t, err, "Test_ArangoMigration_DropDB failed!")
}

func Test_ArangoMigration_DropCollection(t *testing.T) {
	_, mockArango, _ := arangoSetup(t)

	arangoDB := arangoDS{client: mockArango}

	mockArango.EXPECT().DropCollection(context.Background(), "test",
		"database").Return(nil)

	err := arangoDB.DropCollection(context.Background(), "test", "database")

	assert.NoError(t, err, "Test_ArangoMigration_DropCollection failed!")
}

func Test_ArangoMigration_CreateGraph(t *testing.T) {
	_, mockArango, _ := arangoSetup(t)

	arangoDB := arangoDS{client: mockArango}

	mockArango.EXPECT().CreateGraph(context.Background(), "test",
		"database", nil).Return(nil)

	err := arangoDB.CreateGraph(context.Background(), "test", "database", nil)

	assert.NoError(t, err, "Test_ArangoMigration_CreateGraph failed!")
}

func Test_ArangoMigration_DropGraph(t *testing.T) {
	_, mockArango, _ := arangoSetup(t)

	arangoDB := arangoDS{client: mockArango}

	mockArango.EXPECT().DropGraph(context.Background(), "test",
		"database").Return(nil)

	err := arangoDB.DropGraph(context.Background(), "test", "database")

	assert.NoError(t, err, "Test_ArangoMigration_DropGraph failed!")
}

func Test_ArangoMigration_Exists(t *testing.T) {
	_, mockArango, _ := arangoSetup(t)

	arangoDB := arangoDS{client: mockArango}

	mockArango.EXPECT().Exists(context.Background(), "test",
		"database").Return(true, nil)

	res, err := arangoDB.Exists(context.Background(), "test", "database")

	assert.True(t, res, "Test_ArangoMigration_Exists failed!")
	assert.NoError(t, err, "Test_ArangoMigration_Exists failed!")
}
