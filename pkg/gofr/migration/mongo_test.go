package migration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

var errMongoConn = errors.New("error connecting to mongo")
var errMongoInner = errors.New("mongo inner migrator error")
var errMongoDuplicateKey = errors.New("duplicate key")
var errMongoNamespaceExists = errors.New("NamespaceExists: collection already exists")
var errMongoAlreadyExists = errors.New("already exists")

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
		desc                string
		firstCreateCollErr  error
		secondCreateCollErr error
		expectedErr         error
	}{
		{"no error", nil, nil, nil},
		{"first collection already exists", errMongoNamespaceExists, nil, nil},
		{"second collection already exists", nil, errMongoAlreadyExists, nil},
		{"first create collection failed", errMongoConn, nil, errMongoConn},
		{"second create collection failed", nil, errMongoConn, errMongoConn},
	}

	for i, tc := range testCases {
		mockMongo.EXPECT().CreateCollection(gomock.Any(), mongoMigrationCollection).Return(tc.firstCreateCollErr)

		if tc.firstCreateCollErr == nil || isMongoCollectionExistsError(tc.firstCreateCollErr) {
			mockMongo.EXPECT().CreateCollection(gomock.Any(), mongoLockCollection).Return(tc.secondCreateCollErr)
		}

		err := migratorWithMongo.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.expectedErr, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_MongoGetAppliedMigrations(t *testing.T) {
	testCases := []struct {
		desc        string
		versions    []int64
		findErr     error
		baseApplied map[int64]struct{}
		baseErr     error
		expApplied  map[int64]struct{}
		expErr      error
	}{
		{
			desc:        "mongo versions are unioned with inner migrator versions",
			versions:    []int64{1, 3, 4},
			baseApplied: map[int64]struct{}{2: {}},
			expApplied:  map[int64]struct{}{1: {}, 2: {}, 3: {}, 4: {}},
		},
		{
			desc:        "empty collection returns only inner versions",
			versions:    []int64{},
			baseApplied: map[int64]struct{}{5: {}},
			expApplied:  map[int64]struct{}{5: {}},
		},
		{desc: "mongo find error", findErr: errMongoConn, expErr: errMongoConn},
		{desc: "inner migrator error", versions: []int64{2}, baseErr: errMongoInner, expErr: errMongoInner},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockContainer, mocks := container.NewMockContainer(t)
			mockMigrator := NewMockmigrator(ctrl)

			m := mongoMigrator{Mongo: mocks.Mongo, migrator: mockMigrator}

			mocks.Mongo.EXPECT().Find(gomock.Any(), mongoMigrationCollection, make(map[string]any), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ string, _, results any) error {
					if tc.findErr != nil {
						return tc.findErr
					}

					dest := results.(*[]struct {
						Version int64 `bson:"version"`
					})

					for _, v := range tc.versions {
						*dest = append(*dest, struct {
							Version int64 `bson:"version"`
						}{Version: v})
					}

					return nil
				})

			if tc.findErr == nil {
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
		UsedDatasources: map[string]bool{dsMongo: true},
	}

	migrationDoc := map[string]any{
		"version":    td.MigrationNumber,
		"method":     "UP",
		"start_time": td.StartTime,
		"duration":   time.Since(td.StartTime).Milliseconds(),
	}

	for i, tc := range testCases {
		mockMongo.EXPECT().InsertOne(gomock.Any(), mongoMigrationCollection, migrationDoc).Return(mockResult, tc.err)

		err := migratorWithMongo.commitMigration(mockContainer, &td)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_MongoLock(t *testing.T) {
	migratorWithMongo, mockMongo, mockContainer := mongoSetup(t)
	mockResult := struct{}{}

	t.Run("lock acquired", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mockMongo.EXPECT().DeleteOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(int64(0), nil)
		mockMongo.EXPECT().InsertOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(mockResult, nil)

		err := migratorWithMongo.lock(ctx, cancel, mockContainer, "owner-1")
		assert.NoError(t, err)
	})

	t.Run("insert error returns acquisition failed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		mockMongo.EXPECT().DeleteOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(int64(0), nil)
		mockMongo.EXPECT().InsertOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(nil, errMongoConn)

		err := migratorWithMongo.lock(ctx, cancel, mockContainer, "owner-1")
		assert.ErrorIs(t, err, errLockAcquisitionFailed)
	})

	t.Run("duplicate key then context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		mockMongo.EXPECT().DeleteOne(gomock.Any(), mongoLockCollection, gomock.Any()).AnyTimes()
		mockMongo.EXPECT().InsertOne(gomock.Any(), mongoLockCollection, gomock.Any()).
			Return(nil, errMongoDuplicateKey).AnyTimes()

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		err := migratorWithMongo.lock(ctx, cancel, mockContainer, "owner-1")
		assert.ErrorIs(t, err, context.Canceled)
	})
}

func Test_MongoUnlock(t *testing.T) {
	migratorWithMongo, mockMongo, mockContainer := mongoSetup(t)

	t.Run("success", func(t *testing.T) {
		mockMongo.EXPECT().DeleteOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(int64(1), nil)

		err := migratorWithMongo.unlock(mockContainer, "owner-1")
		assert.NoError(t, err)
	})

	t.Run("delete error", func(t *testing.T) {
		mockMongo.EXPECT().DeleteOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(int64(0), errMongoConn)

		err := migratorWithMongo.unlock(mockContainer, "owner-1")
		assert.ErrorIs(t, err, errLockReleaseFailed)
	})

	t.Run("deleted count zero", func(t *testing.T) {
		mockMongo.EXPECT().DeleteOne(gomock.Any(), mongoLockCollection, gomock.Any()).Return(int64(0), nil)

		err := migratorWithMongo.unlock(mockContainer, "owner-1")
		assert.ErrorIs(t, err, errLockReleaseFailed)
	})
}

func Test_MongoStartRefresh(t *testing.T) {
	_, mockMongo, mockContainer := mongoSetup(t)

	type refreshTestCase struct {
		desc              string
		modified          int64
		updateErr         error
		repeatRefreshCall bool
		cancelAfterTick   bool
		expectCtxCanceled bool
	}

	startRefreshAndWait := func(t *testing.T, mg mongoMigrator, ctx context.Context, cancel context.CancelFunc) chan struct{} {
		t.Helper()

		done := make(chan struct{})

		go func() {
			mg.startRefresh(ctx, cancel, mockContainer, "owner-1")
			close(done)
		}()

		time.Sleep(15 * time.Millisecond)

		return done
	}

	waitForRefreshExit := func(t *testing.T, done chan struct{}, failureMsg string) {
		t.Helper()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal(failureMsg)
		}
	}

	testCases := []refreshTestCase{
		{
			desc:            "refresh succeeds with modified documents",
			modified:        1,
			cancelAfterTick: true,
		},
		{
			desc:      "refresh fails with update error",
			modified:  0,
			updateErr: errMongoConn,
		},
		{
			desc:              "refresh detects lock theft when modified count is zero",
			modified:          0,
			expectCtxCanceled: true,
		},
		{
			desc:            "refresh exits when context is canceled",
			modified:        1,
			cancelAfterTick: true,
		},
		{
			desc:              "refresh continues on successful updates",
			modified:          1,
			repeatRefreshCall: true,
			cancelAfterTick:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			expectedCall := mockMongo.EXPECT().UpdateMany(gomock.Any(), mongoLockCollection, gomock.Any(), gomock.Any()).
				Return(tc.modified, tc.updateErr)
			if tc.repeatRefreshCall {
				expectedCall.AnyTimes()
			} else {
				expectedCall.Times(1)
			}

			mg := mongoMigrator{Mongo: mockMongo, migrator: &Datasource{}, testInterval: 10 * time.Millisecond}
			done := startRefreshAndWait(t, mg, ctx, cancel)

			if tc.cancelAfterTick {
				cancel()
			}

			waitForRefreshExit(t, done, "startRefresh did not exit as expected")

			if tc.expectCtxCanceled {
				select {
				case <-ctx.Done():
				default:
					t.Error("expected context to be canceled when lock is stolen")
				}
			}
		})
	}
}

func Test_MongoCommitMigration_SkipsWhenNotUsed(t *testing.T) {
	migratorWithMongo, _, mockContainer := mongoSetup(t)

	td := transactionData{
		StartTime:       time.Now(),
		MigrationNumber: 10,
		UsedDatasources: map[string]bool{},
	}

	err := migratorWithMongo.commitMigration(mockContainer, &td)
	assert.NoError(t, err)
}
