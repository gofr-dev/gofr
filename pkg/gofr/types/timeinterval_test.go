package types

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
)

func TestTimeInterval_Check(t *testing.T) {
	testcases := []struct {
		input TimeInterval
		err   error
	}{
		{"2018-09-18T23:00:19Z/2018-09-19T01:34:37Z", nil},
		{"2020-09-18T16:00:19Z/PT1H", nil},
		{"2019-09-08", errors.InvalidParam{Param: []string{"timeInterval"}}},
		{"2019-02-09T16:00:00Z/23", errors.InvalidParam{Param: []string{"endTime"}}},
		{"2019-02-09T16:00:00/23", errors.InvalidParam{Param: []string{"startTime"}}},
	}

	for i, v := range testcases {
		err := v.input.Check()
		if !reflect.DeepEqual(v.err, err) {
			t.Errorf("[TESTCASE%d]Failed.Got %v\tExpected %v\n", i+1, err, v.err)
		}
	}
}

func Test_GetStartAndEndTime(t *testing.T) {
	tests := []struct {
		timeInterval TimeInterval
		startTime    string
		endTime      string
		expectedErr  error
	}{
		{"2021-09-18T23:00:19Z/2022-09-19T01:34:37Z", "2021-09-18T23:00:19Z", "2022-09-19T01:34:37Z", nil},
		{"2021-02-18T23:00:19+05:30/2022-02-24T17:20:44+05:30", "2021-02-18T23:00:19+05:30", "2022-02-24T17:20:44+05:30", nil},
		{"2021-09-19T01:34:37Z/PT1H", "2021-09-19T01:34:37Z", "2021-09-19T02:34:37Z", nil},
		{"2021-09-19T01:34:37Z/P3Y1M2DT10S", "2021-09-19T01:34:37Z", "2024-10-21T01:34:47Z", nil},
		{"2021-02-09T16:18:30/23", "0001-01-01T00:00:00Z", "0001-01-01T00:00:00Z",
			errors.InvalidParam{Param: []string{"startTime"}}},
		{"2021-02-09T16:17:00Z/23", "0001-01-01T00:00:00Z", "0001-01-01T00:00:00Z",
			errors.InvalidParam{Param: []string{"endTime"}}},
		{"2021-09-08", "0001-01-01T00:00:00Z", "0001-01-01T00:00:00Z",
			errors.InvalidParam{Param: []string{"timeInterval"}}},
	}

	for i, tc := range tests {
		startTime, endTime, err := tc.timeInterval.GetStartAndEndTime()
		assert.Equal(t, tc.expectedErr, err, i)
		assert.Equal(t, tc.startTime, startTime.Format(time.RFC3339), i)
		assert.Equal(t, tc.endTime, endTime.Format(time.RFC3339), i)
	}
}

func Test_GetEndTime(t *testing.T) {
	tests := []struct {
		startTime string
		duration  string
		endTime   string
	}{
		{"2021-09-19T01:34:37Z", "P3Y1M2DT10S", "2024-10-21T01:34:47Z"},
		{"2021-02-18T23:00:19+05:30", "P3Y1M2DT10S", "2024-03-20T23:00:29+05:30"},
		{"2021-09-19T01:34:37Z", "PT1H", "2021-09-19T02:34:37Z"},
	}

	for i, tc := range tests {
		startTime, _ := time.Parse(time.RFC3339, tc.startTime)
		endTime := getEndTime(startTime, tc.duration)
		assert.Equal(t, tc.endTime, endTime.Format(time.RFC3339), i)
	}
}
