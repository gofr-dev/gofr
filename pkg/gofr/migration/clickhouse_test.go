package migration

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func clickHouseSetup(t *testing.T) (migrator, *MockClickhouse, *container.Container) {
	t.Helper()

	ctrl := gomock.NewController(t)

	mockContainer, _ := container.NewMockContainer(t)

	mockClickhouse := NewMockClickhouse(ctrl)

	ds := Datasource{Clickhouse: mockClickhouse}

	ch := clickHouseDS{Clickhouse: mockClickhouse}
	mg := ch.apply(&ds)

	mockContainer.Clickhouse = mockClickhouse

	return mg, mockClickhouse, mockContainer
}

func Test_ClickHouseCheckAndCreateMigrationTable(t *testing.T) {
	mg, mockClickhouse, mockContainer := clickHouseSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	for i, tc := range testCases {
		mockClickhouse.EXPECT().Exec(gomock.Any(), CheckAndCreateChMigrationTable).Return(tc.err)

		err := mg.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

// fillClickHouseVersions populates the destination slice passed to Select, whose element type is local to the
// migrator, by setting the Version field of each element via reflection.
func fillClickHouseVersions(dest any, versions []int64) {
	slice := reflect.ValueOf(dest).Elem()

	for _, v := range versions {
		elem := reflect.New(slice.Type().Elem()).Elem()
		elem.FieldByName("Version").SetInt(v)
		slice.Set(reflect.Append(slice, elem))
	}
}

func Test_ClickHouse_getAppliedMigrations(t *testing.T) {
	testCases := []struct {
		desc         string
		versions     []int64
		selectErr    error
		callInner    bool
		innerApplied map[int64]struct{}
		innerErr     error
		expected     map[int64]struct{}
		expErr       error
	}{
		{"unions clickhouse versions with inner versions", []int64{1, 3, 4, 3}, nil, true,
			map[int64]struct{}{2: {}}, nil, map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}}, nil},
		{"empty table returns only the inner set", nil, nil, true,
			map[int64]struct{}{5: {}}, nil, map[int64]struct{}{5: {}}, nil},
		{"connection failed", nil, sql.ErrConnDone, false, nil, nil, nil, sql.ErrConnDone},
		{"inner migrator error", []int64{1}, nil, true, nil, assert.AnError, nil, assert.AnError},
	}

	for i, tc := range testCases {
		ctrl := gomock.NewController(t)
		mockContainer, _ := container.NewMockContainer(t)
		mockClickhouse := NewMockClickhouse(ctrl)
		mockMigrator := NewMockmigrator(ctrl)

		mockContainer.Clickhouse = mockClickhouse

		mg := clickHouseMigrator{Clickhouse: mockClickhouse, migrator: mockMigrator}

		mockClickhouse.EXPECT().Select(gomock.Any(), gomock.Any(), getChGoFrMigrationVersions).
			DoAndReturn(func(_ context.Context, dest any, _ string, _ ...any) error {
				fillClickHouseVersions(dest, tc.versions)

				return tc.selectErr
			})

		if tc.callInner {
			mockMigrator.EXPECT().getAppliedMigrations(mockContainer).Return(tc.innerApplied, tc.innerErr)
		}

		resp, err := mg.getAppliedMigrations(mockContainer)

		if tc.expErr != nil {
			require.ErrorIs(t, err, tc.expErr, "TEST[%v]\n %v Failed! ", i, tc.desc)
			assert.Nil(t, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)

			continue
		}

		require.NoError(t, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
		assert.Equal(t, tc.expected, resp, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ClickHouse_getAppliedMigrations_BaseDatasource(t *testing.T) {
	mg, mockClickhouse, mockContainer := clickHouseSetup(t)

	mockClickhouse.EXPECT().Select(gomock.Any(), gomock.Any(), getChGoFrMigrationVersions).Return(nil)

	resp, err := mg.getAppliedMigrations(mockContainer)

	require.NoError(t, err)
	assert.Equal(t, map[int64]struct{}{}, resp)
}

func Test_ClickHouseCommitMigration(t *testing.T) {
	mg, mockClickhouse, mockContainer := clickHouseSetup(t)

	testCases := []struct {
		desc string
		err  error
	}{
		{"no error", nil},
		{"connection failed", sql.ErrConnDone},
	}

	timeNow := time.Now()

	td := transactionData{
		StartTime:       timeNow,
		MigrationNumber: 10,
		UsedDatasources: map[string]bool{dsClickhouse: true},
	}

	for i, tc := range testCases {
		mockClickhouse.EXPECT().Exec(gomock.Any(), insertChGoFrMigrationRow, td.MigrationNumber,
			"UP", td.StartTime, gomock.Any()).Return(tc.err)

		err := mg.commitMigration(mockContainer, &td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_ClickHouseBeginTransaction(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		mg, _, mockContainer := clickHouseSetup(t)
		mg.beginTransaction(mockContainer)
	})

	assert.Contains(t, logs, "Clickhouse Migrator begin successfully")
}

func Test_ClickHouseCommitMigration_SkipsWhenNotUsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCH := NewMockClickhouse(ctrl)
	mockMigrator := NewMockmigrator(ctrl)

	m := clickHouseMigrator{Clickhouse: mockCH, migrator: mockMigrator}

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

func Test_ClickHouseCommitMigration_NilUsedDatasources(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCH := NewMockClickhouse(ctrl)
	mockMigrator := NewMockmigrator(ctrl)

	m := clickHouseMigrator{Clickhouse: mockCH, migrator: mockMigrator}

	c, _ := container.NewMockContainer(t)

	data := transactionData{
		MigrationNumber: 1,
		StartTime:       time.Now(),
		UsedDatasources: nil, // nil — map lookup returns false, so no insert should happen
	}

	mockMigrator.EXPECT().commitMigration(c, &data).Return(nil)

	err := m.commitMigration(c, &data)
	assert.NoError(t, err)
}
