package migration

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

var (
	errTopic = errors.New("topic error")
	errQuery = errors.New("query error")
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

func Test_CreateTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPubSub := container.NewMockPubSubProvider(ctrl)
	ds := pubsubDS{client: mockPubSub}

	testCases := []struct {
		desc     string
		topic    string
		mockErr  error
		expected error
	}{
		{"successfully create topic", "test-topic", nil, nil},
		{"error creating topic", "test-topic", errTopic, errTopic},
	}

	for _, tc := range testCases {
		mockPubSub.EXPECT().CreateTopic(t.Context(), tc.topic).Return(tc.mockErr)

		err := ds.CreateTopic(t.Context(), tc.topic)

		assert.Equal(t, tc.expected, err, tc.desc)
	}
}

func Test_DeleteTopic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPubSub := container.NewMockPubSubProvider(ctrl)
	ds := pubsubDS{client: mockPubSub}

	testCases := []struct {
		desc     string
		topic    string
		mockErr  error
		expected error
	}{
		{"successfully delete topic", "test-topic", nil, nil},
		{"error deleting topic", "test-topic", errTopic, errTopic},
	}

	for _, tc := range testCases {
		mockPubSub.EXPECT().DeleteTopic(t.Context(), tc.topic).Return(tc.mockErr)

		err := ds.DeleteTopic(t.Context(), tc.topic)

		assert.Equal(t, tc.expected, err, tc.desc)
	}
}

func Test_Query(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPubSub := container.NewMockPubSubProvider(ctrl)
	ds := pubsubDS{client: mockPubSub}

	testCases := []struct {
		desc     string
		query    string
		args     []any
		mockResp []byte
		mockErr  error
		expected []byte
		err      error
	}{
		{"successful query", "SELECT * FROM test", []any{1, 2}, []byte("result"), nil, []byte("result"), nil},
		{"error in query", "SELECT * FROM test", []any{1, 2}, nil, errQuery, nil, errQuery},
	}

	for _, tc := range testCases {
		mockPubSub.EXPECT().Query(
			gomock.Eq(t.Context()), gomock.Eq(tc.query), gomock.Eq(tc.args[0]), gomock.Eq(tc.args[1]),
		).Return(tc.mockResp, tc.mockErr)

		resp, err := ds.Query(t.Context(), tc.query, tc.args...)

		assert.Equal(t, tc.expected, resp, tc.desc)
		assert.Equal(t, tc.err, err, tc.desc)
	}
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
		mockPubSub.EXPECT().CreateTopic(gomock.Any(), pubsubMigrationTopic).Return(tc.err)

		err := migratorWithPubSub.checkAndCreateMigrationTable(mockContainer)

		assert.Equal(t, tc.err, err, "TEST[%v]\n %v Failed! ", i, tc.desc)
	}
}

func Test_PubSubCommitMigration_Success(t *testing.T) {
	migratorWithPubSub, mockPubSub, mockContainer := pubsubTestSetup(t)

	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	data := transactionData{
		MigrationNumber: 123,
		StartTime:       fixedTime,
	}

	mockPubSub.EXPECT().Publish(gomock.Any(), pubsubMigrationTopic, gomock.Any()).Return(nil)

	err := migratorWithPubSub.commitMigration(mockContainer, data)

	assert.NoError(t, err, "Successful migration commit should not return an error")
}

func Test_PubSubCommitMigration_PublishError(t *testing.T) {
	migratorWithPubSub, mockPubSub, mockContainer := pubsubTestSetup(t)

	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	data := transactionData{
		MigrationNumber: 123,
		StartTime:       fixedTime,
	}

	mockPubSub.EXPECT().Publish(gomock.Any(), pubsubMigrationTopic, gomock.Any()).Return(errTopic)

	err := migratorWithPubSub.commitMigration(mockContainer, data)

	assert.Equal(t, errTopic, err, "Publish error should be returned")
}

func Test_PubSubGetLastMigration(t *testing.T) {
	migratorWithPubSub, mockPubSub, mockContainer := pubsubTestSetup(t)

	testCases := []struct {
		desc           string
		expectedResult int64
		setupMocks     func(*container.MockPubSubProvider)
	}{
		{
			desc:           "successful query with migrations",
			expectedResult: 3,
			setupMocks: func(mockPubSub *container.MockPubSubProvider) {
				mockPubSub.EXPECT().
					Query(gomock.Any(), pubsubMigrationTopic, int64(0), defaultQueryLimit).
					Return([]byte(`{"version":1,"method":"UP","start_time":1625000000000,"duration":100}
{"version":3,"method":"UP","start_time":1625000200000,"duration":150}`), nil)
			},
		},
		{
			desc:           "query error",
			expectedResult: 0,
			setupMocks: func(mockPubSub *container.MockPubSubProvider) {
				mockPubSub.EXPECT().
					Query(gomock.Any(), pubsubMigrationTopic, int64(0), defaultQueryLimit).
					Return(nil, errQuery)
			},
		},
		{
			desc:           "empty result",
			expectedResult: 0,
			setupMocks: func(mockPubSub *container.MockPubSubProvider) {
				mockPubSub.EXPECT().
					Query(gomock.Any(), pubsubMigrationTopic, int64(0), defaultQueryLimit).
					Return([]byte{}, nil)
			},
		},
		{
			desc:           "invalid JSON",
			expectedResult: 0,
			setupMocks: func(mockPubSub *container.MockPubSubProvider) {
				mockPubSub.EXPECT().
					Query(gomock.Any(), pubsubMigrationTopic, int64(0), defaultQueryLimit).
					Return([]byte(`{"invalid json`), nil)
			},
		},
		{
			desc:           "mixed migration methods",
			expectedResult: 1,
			setupMocks: func(mockPubSub *container.MockPubSubProvider) {
				mockPubSub.EXPECT().
					Query(gomock.Any(), pubsubMigrationTopic, int64(0), defaultQueryLimit).
					Return([]byte(`{"version":1,"method":"UP","start_time":1625000000000,"duration":100}
{"version":2,"method":"DOWN","start_time":1625000100000,"duration":120}`), nil)
			},
		},
	}

	for i, tc := range testCases {
		tc.setupMocks(mockPubSub)

		result := migratorWithPubSub.getLastMigration(mockContainer)

		assert.Equal(t, tc.expectedResult, result, "TEST[%v] %v Failed!", i, tc.desc)
	}
}
