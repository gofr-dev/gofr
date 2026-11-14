package cassandra

import (
	"fmt"
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// samePointerMatcher matches only the exact pointer it was built with, so it
// can tell apart "the same destination reached the session" from "an equal
// looking value reached it" the way gomock.Eq's reflect.DeepEqual would.
type samePointerMatcher struct {
	want any
}

func (m samePointerMatcher) Matches(x any) bool { return x == m.want }
func (m samePointerMatcher) String() string     { return fmt.Sprintf("is exactly %v", m.want) }

func samePointer(want any) gomock.Matcher { return samePointerMatcher{want: want} }

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
		}, errBatchNotInitialized},
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
		}, errBatchNotInitialized},
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
		}, &mockStructSlice, errBatchNotInitialized},
	}

	for i, tc := range testCases {
		tc.mockCall()

		applied, err := client.ExecuteBatchCAS(mockBatchName, tc.dest)

		assert.Equalf(t, tc.expRes, tc.dest, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, tc.expErr, err, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equalf(t, applied, tc.expErr == nil, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func Test_ExecuteBatchCASForwardsDest(t *testing.T) {
	client, mockDeps := initTest(t)

	var (
		id   int
		name string
	)

	mockDeps.mockSession.EXPECT().
		executeBatchCAS(mockDeps.mockBatch, samePointer(&id), samePointer(&name)).
		Return(true, nil).Times(1)

	applied, err := client.ExecuteBatchCAS(mockBatchName, &id, &name)

	require.NoError(t, err)
	assert.True(t, applied)
}
