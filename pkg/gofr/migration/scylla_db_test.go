package migration

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

var errScyllaConn = errors.New("error connecting to ScyllaDB")
var errScyllaInner = errors.New("scylla inner migrator error")

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

func TestScyllaGetAppliedMigrations(t *testing.T) {
	testCases := []struct {
		desc        string
		versions    []int64
		queryErr    error
		baseApplied map[int64]struct{}
		baseErr     error
		expApplied  map[int64]struct{}
		expErr      error
	}{
		{
			desc:        "scylla versions are unioned with inner migrator versions",
			versions:    []int64{1, 3, 4},
			baseApplied: map[int64]struct{}{2: {}},
			expApplied:  map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}},
		},
		{
			desc:        "empty scylla table returns only inner versions",
			versions:    []int64{},
			baseApplied: map[int64]struct{}{5: {}},
			expApplied:  map[int64]struct{}{5: {}},
		},
		{desc: "scylla query error", queryErr: errScyllaConn, expErr: errScyllaConn},
		{desc: "inner migrator error", versions: []int64{2}, baseErr: errScyllaInner, expErr: errScyllaInner},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockContainer, mocks := container.NewMockContainer(t)
			mockMigrator := NewMockmigrator(ctrl)

			m := scyllaMigrator{ScyllaDB: mocks.ScyllaDB, migrator: mockMigrator}

			mocks.ScyllaDB.EXPECT().Query(gomock.Any(), gomock.Any()).
				DoAndReturn(func(dest any, _ string, _ ...any) error {
					if tc.queryErr != nil {
						return tc.queryErr
					}

					rows := make([]migrationRow, 0, len(tc.versions))
					for _, v := range tc.versions {
						rows = append(rows, migrationRow{Version: v})
					}

					*(dest.(*[]migrationRow)) = rows

					return nil
				})

			if tc.queryErr == nil {
				mockMigrator.EXPECT().getAppliedMigrations(mockContainer).Return(tc.baseApplied, tc.baseErr)
			}

			applied, err := m.getAppliedMigrations(mockContainer)

			assert.Equal(t, tc.expApplied, applied)

			if tc.expErr != nil {
				require.ErrorIs(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}
		})
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
			UsedDatasources: map[string]bool{dsScyllaDB: true},
		}

		mockScylla.EXPECT().
			Exec(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(tc.err)

		err := migratorWithScylla.commitMigration(mockContainer, &td)

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
	t.Cleanup(ctrl.Finish)

	mockContainer := &container.Container{
		Logger: &panicLogger{},
	}

	mockMigrator := NewMockmigrator(ctrl)

	s := scyllaMigrator{
		ScyllaDB: nil,
		migrator: mockMigrator,
	}

	data := transactionData{MigrationNumber: 123}

	mockMigrator.EXPECT().rollback(mockContainer, &data).Times(1)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic from Fatalf, but none occurred")
		}
	}()

	s.rollback(mockContainer, &data)
}

func TestScyllaCommitMigration_SkipsWhenNotUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockScylla := NewMockScyllaDB(ctrl)
	mockMigrator := NewMockmigrator(ctrl)

	m := scyllaMigrator{ScyllaDB: mockScylla, migrator: mockMigrator}

	c, _ := container.NewMockContainer(t)

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		UsedDatasources: map[string]bool{},
	}

	mockMigrator.EXPECT().commitMigration(c, &data).Return(nil)

	err := m.commitMigration(c, &data)
	assert.NoError(t, err)
}

func TestScyllaMigrator_LockUnlockName(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)
	mockMigrator := NewMockmigrator(ctrl)

	m := scyllaMigrator{migrator: mockMigrator}

	mockMigrator.EXPECT().lock(gomock.Any(), gomock.Any(), mockContainer, "owner-1").Return(errScyllaConn)
	mockMigrator.EXPECT().unlock(mockContainer, "owner-1").Return(errScyllaConn)

	require.ErrorIs(t, m.lock(t.Context(), func() {}, mockContainer, "owner-1"), errScyllaConn)
	require.ErrorIs(t, m.unlock(mockContainer, "owner-1"), errScyllaConn)
	assert.Equal(t, "ScyllaDB", m.name())
}
