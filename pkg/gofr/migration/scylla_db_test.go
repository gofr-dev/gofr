package migration

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

var errScyllaConn = errors.New("error connecting to ScyllaDB")

type panicLogger struct{}

func (*panicLogger) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

func (*panicLogger) Fatal(args ...any) {
	panic(fmt.Sprint(args...))
}

func (*panicLogger) Errorf(_ string, _ ...any)   {}
func (*panicLogger) Error(_ ...any)              {}
func (*panicLogger) Debugf(_ string, _ ...any)   {}
func (*panicLogger) Noticef(_ string, _ ...any)  {}
func (*panicLogger) Debug(_ ...any)              {}
func (*panicLogger) Infof(_ string, _ ...any)    {}
func (*panicLogger) Info(_ ...any)               {}
func (*panicLogger) Notice(_ ...any)             {}
func (*panicLogger) Warn(_ ...any)               {}
func (*panicLogger) Warnf(_ string, _ ...any)    {}
func (*panicLogger) Log(_ ...any)                {}
func (*panicLogger) Logf(_ string, _ ...any)     {}
func (*panicLogger) ChangeLevel(_ logging.Level) {}

type NoopLogger struct{}

func (*NoopLogger) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}
func (*NoopLogger) Fatal(args ...any) {
	panic(fmt.Sprint(args...))
}
func (*NoopLogger) Errorf(_ string, _ ...any)   {}
func (*NoopLogger) Error(_ ...any)              {}
func (*NoopLogger) Debugf(_ string, _ ...any)   {}
func (*NoopLogger) Noticef(_ string, _ ...any)  {}
func (*NoopLogger) Debug(_ ...any)              {}
func (*NoopLogger) Infof(_ string, _ ...any)    {}
func (*NoopLogger) Info(_ ...any)               {}
func (*NoopLogger) Notice(_ ...any)             {}
func (*NoopLogger) Warn(_ ...any)               {}
func (*NoopLogger) Warnf(_ string, _ ...any)    {}
func (*NoopLogger) Log(_ ...any)                {}
func (*NoopLogger) Logf(_ string, _ ...any)     {}
func (*NoopLogger) ChangeLevel(_ logging.Level) {}

func scyllaSetup(t *testing.T) (migrator, *container.MockScyllaDB, *container.Container) {
	t.Helper()

	mockContainer, mocks := container.NewMockContainer(t)
	mockScylla := mocks.ScyllaDB

	mockContainer.Logger = &NoopLogger{}

	ds := Datasource{ScyllaDB: mockContainer.ScyllaDB}
	scylla := scyllaDS{ScyllaDB: mockContainer.ScyllaDB}
	migratorWithScylla := scylla.apply(&ds)

	return migratorWithScylla, mockScylla, mockContainer
}

func TestScyllaCheckAndCreateMigrationTable(t *testing.T) {
	migratorWithScylla, mockScylla, mockContainer := scyllaSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"create table failed", errScyllaConn},
	}

	for i, tc := range testCases {
		mockScylla.EXPECT().
			Exec(gomock.Any()).
			Return(tc.err)

		err := migratorWithScylla.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v] %s failed", i, tc.desc)
	}
}

func TestScyllaGetLastMigration(t *testing.T) {
	migratorWithScylla, mockScylla, mockContainer := scyllaSetup(t)

	testCases := []struct {
		desc      string
		err       error
		versions  []int64
		expectedV int64
	}{
		{"no error with multiple versions", nil, []int64{1, 3, 9, 4}, 9},
		{"query failed", errScyllaConn, nil, 0},
		{"empty result", nil, []int64{}, 0},
	}

	for i, tc := range testCases {
		var rows []migrationRow
		for _, v := range tc.versions {
			rows = append(rows, migrationRow{Version: v})
		}

		mockScylla.EXPECT().
			Query(gomock.Any(), gomock.Any()).
			DoAndReturn(func(dest any, _ string, _ ...any) error {
				if tc.err != nil {
					return tc.err
				}

				ptr := dest.(*[]migrationRow)
				*ptr = rows

				return nil
			})

		got := migratorWithScylla.getLastMigration(mockContainer)

		assert.Equal(t, tc.expectedV, got, "TEST[%v] %s failed", i, tc.desc)
	}
}

func TestScyllaCommitMigration(t *testing.T) {
	migratorWithScylla, mockScylla, mockContainer := scyllaSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"successful insert", nil},
		{"insert fails", errScyllaConn},
	}

	for i, tc := range testCases {
		td := transactionData{
			MigrationNumber: 123,
			StartTime:       time.Now(),
		}

		mockScylla.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(tc.err)

		err := migratorWithScylla.commitMigration(mockContainer, td)

		assert.Equal(t, tc.err, err, "TEST[%v] %s failed", i, tc.desc)
	}
}

func TestScyllaBeginTransaction(t *testing.T) {
	migratorWithScylla, _, mockContainer := scyllaSetup(t)

	data := migratorWithScylla.beginTransaction(mockContainer)

	assert.NotNil(t, data)
}

func TestScyllaMigrator_Rollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockContainer := &container.Container{
		Logger: &panicLogger{},
	}

	mockMigrator := NewMockmigrator(ctrl)

	s := scyllaMigrator{
		ScyllaDB: nil,
		migrator: mockMigrator,
	}

	data := transactionData{MigrationNumber: 123}

	mockMigrator.EXPECT().rollback(mockContainer, data).Times(1)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic from Fatalf, but none occurred")
		}
	}()

	s.rollback(mockContainer, data)
}
