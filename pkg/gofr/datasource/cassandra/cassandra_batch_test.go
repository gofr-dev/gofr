package cassandra

import (
	"testing"

	"github.com/gocql/gocql"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
)

func Test_NewBatch(t *testing.T) {
	client, _, mockDeps := initTest(t)

	testCases := []struct {
		desc      string
		batchType int
		mockCall  func()
		expErr    error
	}{
		{"valid log type", LoggedBatch, func() {
			mockDeps.mockSession.EXPECT().newBatch(gocql.BatchType(LoggedBatch)).Return(&cassandraBatch{}).Times(1)
		}, nil},
		{"invalid log type", -1, func() {}, ErrUnsupportedBatchType},
	}

	for i, tc := range testCases {
		tc.mockCall()

		_, err := NewBatch(client, tc.batchType)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_BatchQuery(t *testing.T) {
	_, batchClient, mockDeps := initTest(t)

	const stmt = "INSERT INTO users (id, name) VALUES(?, ?)"

	values := []interface{}{1, "Test"}

	mockDeps.mockBatch.EXPECT().Query(stmt, values...)

	batchClient.BatchQuery(stmt, values...)

	assert.NotNil(t, batchClient.batch, "TEST, Failed.\n")
}

func Test_ExecuteBatch(t *testing.T) {
	_, batchClient, mockDeps := initTest(t)

	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"execute batch success", func() {
			mockDeps.mockSession.EXPECT().executeBatch(mockDeps.mockBatch).Return(nil).Times(1)
		}, nil},
		{"execute batch failure", func() {
			mockDeps.mockSession.EXPECT().executeBatch(mockDeps.mockBatch).Return(errMock).Times(1)
		}, errMock},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := batchClient.ExecuteBatch()

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExecuteBatch_NotInitialised(t *testing.T) {
	_, batchClient, _ := initTest(t)

	batchClient.batch = nil

	err := batchClient.ExecuteBatch()

	assert.Equalf(t, ErrBatchNotInitialised, err, "TEST, Failed")
}

func Test_newBatch(t *testing.T) {
	cassSession := &cassandraSession{session: &gocql.Session{}}

	testCases := []struct {
		desc      string
		batchType gocql.BatchType
	}{
		{"create logged batch", gocql.LoggedBatch},
		{"create unlogged batch", gocql.UnloggedBatch},
		{"create counter batch", gocql.CounterBatch},
	}

	for i, tc := range testCases {
		b := cassSession.newBatch(tc.batchType)

		assert.NotNil(t, b, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_cassandraBatch_Query(t *testing.T) {
	c := &cassandraBatch{batch: &gocql.Batch{}}

	c.Query("test query")

	assert.Equalf(t, 1, c.batch.Size(), "Test Failed")
}

func Test_cassandraBatch_getBatch(t *testing.T) {
	c := &cassandraBatch{batch: &gocql.Batch{}}

	assert.NotNil(t, c.getBatch(), "Test Failed")
}

func Test_ExecuteBatchCAS(t *testing.T) {
	_, batchClient, mockDeps := initTest(t)

	type testStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStructSlice := make([]testStruct, 0)
	mockIntSlice := make([]int, 0)
	mockStruct := testStruct{}
	mockInt := 0

	testCases := []struct {
		desc     string
		dest     interface{}
		mockCall func()
		expRes   interface{}
		expErr   error
	}{
		{"success case: struct slice", &mockStructSlice, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(gomock.Any()).Return(true, mockDeps.mockIter, nil).Times(1)
			mockDeps.mockIter.EXPECT().numRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockDeps.mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockStructSlice, nil},
		{"success case: int slice", &mockIntSlice, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(gomock.Any()).Return(true, mockDeps.mockIter, nil).Times(1)
			mockDeps.mockQuery.EXPECT().iter().Return(mockDeps.mockIter).Times(1)
			mockDeps.mockIter.EXPECT().numRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockIntSlice, nil},
		{"success case: struct", &mockStruct, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(gomock.Any()).Return(true, mockDeps.mockIter, nil).Times(1)
			mockDeps.mockIter.EXPECT().numRows().Return(1).Times(1)
			mockDeps.mockIter.EXPECT().columns().Return([]gocql.ColumnInfo{{Name: "id"}, {Name: "name"}}).Times(1)
			mockDeps.mockIter.EXPECT().scan(gomock.Any()).Times(1)
		}, &mockStruct, nil},
		{"failure case: dest is not pointer", mockInt, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(gomock.Any()).Return(true, mockDeps.mockIter, nil).Times(1)
		}, 0, ErrDestinationIsNotPointer},
		{"failure case: dest is int", &mockInt, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(gomock.Any()).Return(true, mockDeps.mockIter, nil).Times(1)
		}, &mockInt, UnexpectedPointer{target: "int"}},
		{"failure case: executeBatchCAS returns error", &mockStructSlice, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(gomock.Any()).Return(false, nil, assert.AnError).Times(1)
		}, &mockStructSlice, assert.AnError},
	}

	for i, tc := range testCases {
		tc.mockCall()

		applied, err := batchClient.ExecuteBatchCAS(tc.dest)

		assert.Equalf(t, tc.expRes, tc.dest, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Truef(t, applied == (tc.expErr == nil), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
