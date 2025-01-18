package cassandra

import (
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_BatchQuery(t *testing.T) {
	client, mockDeps := initTest(t)

	const stmt = "INSERT INTO users (id, name) VALUES(?, ?)"

	values := []any{1, "Test"}

	testCases := []struct {
		desc     string
		mockCall func()
		expErr   error
	}{
		{"batch is initialized", func() {
			mockDeps.mockBatch.EXPECT().Query(stmt, values...)
		}, nil},
		{"batch is not initialized", func() {
			client.cassandra.batches = nil
		}, errBatchNotInitialised},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.BatchQuery(mockBatchName, stmt, values...)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExecuteBatch(t *testing.T) {
	client, mockDeps := initTest(t)

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
		{"batch not initialized", func() {
			client.cassandra.batches = nil
		}, errBatchNotInitialised},
	}

	for i, tc := range testCases {
		tc.mockCall()

		err := client.ExecuteBatch(mockBatchName)

		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
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
	client, mockDeps := initTest(t)

	type testStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	mockStructSlice := make([]testStruct, 0)

	testCases := []struct {
		desc     string
		dest     any
		mockCall func()
		expRes   any
		expErr   error
	}{
		{"success case: struct slice", &mockStructSlice, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(mockDeps.mockBatch, gomock.Any()).Return(true, nil).Times(1)
		}, &mockStructSlice, nil},
		{"failure case: executeBatchCAS returns error", &mockStructSlice, func() {
			mockDeps.mockSession.EXPECT().executeBatchCAS(mockDeps.mockBatch, gomock.Any()).Return(false, assert.AnError).Times(1)
		}, &mockStructSlice, assert.AnError},
		{"failure case: batch not initialized", &mockStructSlice, func() {
			client.cassandra.batches = nil
		}, &mockStructSlice, errBatchNotInitialised},
	}

	for i, tc := range testCases {
		tc.mockCall()

		applied, err := client.ExecuteBatchCAS(mockBatchName, tc.dest)

		assert.Equalf(t, tc.expRes, tc.dest, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, applied, tc.expErr == nil, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
