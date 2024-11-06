package gofr

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestCron_parseSchedule_Success(t *testing.T) {
	testCases := []struct {
		desc     string
		schedule string
		expJob   *job
	}{
		{
			desc:     "success case: all wildcard",
			schedule: "* * * * * *",
			expJob: &job{
				sec:       getDefaultJobField(0, 59, 1),
				min:       getDefaultJobField(0, 59, 1),
				hour:      getDefaultJobField(0, 23, 1),
				day:       getDefaultJobField(1, 31, 1),
				month:     getDefaultJobField(1, 12, 1),
				dayOfWeek: getDefaultJobField(0, 6, 1),
			},
		},
		{
			schedule: "*/3 * * * *",
			expJob: &job{
				min:       getDefaultJobField(0, 59, 3),
				hour:      getDefaultJobField(0, 23, 1),
				day:       getDefaultJobField(1, 31, 1),
				month:     getDefaultJobField(1, 12, 1),
				dayOfWeek: getDefaultJobField(0, 6, 1),
			},
		},
		{
			schedule: "1,5,10 * * * *",
			expJob: &job{
				min:       map[int]struct{}{1: {}, 5: {}, 10: {}},
				hour:      getDefaultJobField(0, 23, 1),
				day:       getDefaultJobField(1, 31, 1),
				month:     getDefaultJobField(1, 12, 1),
				dayOfWeek: getDefaultJobField(0, 6, 1),
			},
		},
		{
			schedule: "*/20 3-5 * * *",
			expJob: &job{
				min:       getDefaultJobField(0, 59, 20),
				hour:      getDefaultJobField(3, 5, 1),
				day:       getDefaultJobField(1, 31, 1),
				month:     getDefaultJobField(1, 12, 1),
				dayOfWeek: getDefaultJobField(0, 6, 1),
			},
		},
		{
			schedule: "*/20 3-5/2 * * *",
			expJob: &job{
				min:       getDefaultJobField(0, 59, 20),
				hour:      getDefaultJobField(3, 5, 2),
				day:       getDefaultJobField(1, 31, 1),
				month:     getDefaultJobField(1, 12, 1),
				dayOfWeek: getDefaultJobField(0, 6, 1),
			},
		},
		{
			schedule: "*/20 3-5/2 22 * *",
			expJob: &job{
				min:       getDefaultJobField(0, 59, 20),
				hour:      getDefaultJobField(3, 5, 2),
				day:       map[int]struct{}{22: {}},
				month:     getDefaultJobField(1, 12, 1),
				dayOfWeek: map[int]struct{}{},
			},
		},
		{
			schedule: "*/20 3-5/2 22 */5 *",
			expJob: &job{
				min:       getDefaultJobField(0, 59, 20),
				hour:      getDefaultJobField(3, 5, 2),
				day:       map[int]struct{}{22: {}},
				month:     getDefaultJobField(1, 12, 5),
				dayOfWeek: map[int]struct{}{},
			},
		},
		{
			schedule: "*/20 3-5/2 * */5 4",
			expJob: &job{
				min:       getDefaultJobField(0, 59, 20),
				hour:      getDefaultJobField(3, 5, 2),
				day:       map[int]struct{}{},
				month:     getDefaultJobField(1, 12, 5),
				dayOfWeek: map[int]struct{}{4: {}},
			},
		},
	}

	for _, tc := range testCases {
		j, err := parseSchedule(tc.schedule)

		require.NoError(t, err)
		assert.Equal(t, *tc.expJob, *j)
	}
}

func TestCron_parseSchedule_Error(t *testing.T) {
	testCases := []struct {
		desc         string
		schedules    []string
		expErrString string
	}{
		{
			desc:         "incorrect number of schedule parts: less",
			schedules:    []string{"* * * * ", "* * * * * * *"},
			expErrString: "schedule string must have five components like * * * * *",
		},
		{
			desc: "incorrect range",
			schedules: []string{
				"1-100 * * * * *",
				"1-200 * * * *",
				"* 0-30 * * *",
				"* * 0-10 * *",
				"* * 1-33 * *",
				"* * * 0-22 *",
				"* * * * 0-7",
				"* * 1-40/2 * *",
				"60 * * * *",
			},
			expErrString: "out of range",
		},
		{
			desc: "unparsable schedule parts",
			schedules: []string{
				"* * ab/2 * *",
				"* 1,2/10 * * *",
				"* * 1,2,3,1-15/10 * *",
				"a b c d e"},
			expErrString: "unable to parse",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			for _, s := range tc.schedules {
				j, err := parseSchedule(s)

				assert.Nil(t, j)
				require.ErrorContains(t, err, tc.expErrString)
			}
		})
	}
}

func TestCron_getDefaultJobField(t *testing.T) {
	testCases := []struct {
		min         int
		max         int
		incr        int
		expOutCount int
	}{
		{1, 10, 1, 10},
		{1, 10, 2, 5},
	}

	for _, tc := range testCases {
		out := getDefaultJobField(tc.min, tc.max, tc.incr)

		assert.Len(t, out, tc.expOutCount)
	}
}

func TestCron_getTick(t *testing.T) {
	expTick := &tick{10, 20, 13, 10, 5, 5}

	tM := time.Date(2024, 5, 10, 13, 20, 10, 1, time.Local)

	tck := getTick(tM)

	assert.Equal(t, expTick, tck)
}

func TestCronTab_AddJob(t *testing.T) {
	fn := func(*Context) {}

	testCases := []struct {
		schedule string
		expErr   error
	}{
		{
			schedule: "* * * * *",
		},
		{
			schedule: "* * * *",
			expErr:   errBadScheduleFormat,
		},
	}

	c := NewCron(nil)

	for _, tc := range testCases {
		err := c.AddJob(tc.schedule, "test-job", fn)

		assert.Equal(t, tc.expErr, err)
	}
}

func TestCronTab_runScheduled(t *testing.T) {
	j := &job{
		sec:       map[int]struct{}{1: {}},
		min:       map[int]struct{}{1: {}},
		hour:      map[int]struct{}{1: {}},
		day:       map[int]struct{}{1: {}},
		month:     map[int]struct{}{1: {}},
		dayOfWeek: map[int]struct{}{1: {}},
		fn:        func(*Context) { fmt.Println("hello from cron") },
	}

	// can make container nil as we are not testing the internal working of
	// dependency function as it is user defined
	c := NewCron(nil)

	// Populate the job array for cron table
	c.jobs = []*job{j}

	out := testutil.StdoutOutputForFunc(func() {
		c.runScheduled(time.Date(2024, 1, 1, 1, 1, 1, 1, time.Local))

		// block the main go routine to let the cron run
		time.Sleep(100 * time.Millisecond)
	})

	assert.Contains(t, out, "hello from cron")
}

func TestJob_tick(t *testing.T) {
	tck := &tick{1, 1, 1, 1, 1, 1}

	testCases := []struct {
		desc string
		job  *job
		exp  bool
	}{
		{
			desc: "min not matching",
			job: &job{
				sec: map[int]struct{}{1: {}},
				min: map[int]struct{}{2: {}},
			},
		},
		{
			desc: "hour not matching",
			job: &job{
				min:  map[int]struct{}{1: {}},
				hour: map[int]struct{}{2: {}},
			},
		},
		{
			desc: "day not matching",
			job: &job{
				min:  map[int]struct{}{1: {}},
				hour: map[int]struct{}{1: {}},
				day:  map[int]struct{}{2: {}},
			},
		},
		{
			desc: "month not matching",
			job: &job{
				min:       map[int]struct{}{1: {}},
				hour:      map[int]struct{}{1: {}},
				day:       map[int]struct{}{1: {}},
				dayOfWeek: map[int]struct{}{1: {}},
				month:     map[int]struct{}{2: {}},
			},
		},
		{
			desc: "weekday not matching",
			job: &job{
				min:       map[int]struct{}{1: {}},
				hour:      map[int]struct{}{1: {}},
				day:       map[int]struct{}{1: {}},
				dayOfWeek: map[int]struct{}{2: {}},
			},
		},
		{
			desc: "sec not matching",
			job: &job{
				sec:       map[int]struct{}{2: {}},
				min:       map[int]struct{}{1: {}},
				hour:      map[int]struct{}{1: {}},
				day:       map[int]struct{}{1: {}},
				month:     map[int]struct{}{1: {}},
				dayOfWeek: map[int]struct{}{1: {}},
			},
		},
		{
			desc: "job scheduled on the tick",
			job: &job{
				sec:       map[int]struct{}{1: {}},
				min:       map[int]struct{}{1: {}},
				hour:      map[int]struct{}{1: {}},
				day:       map[int]struct{}{1: {}},
				month:     map[int]struct{}{1: {}},
				dayOfWeek: map[int]struct{}{1: {}},
			},
			exp: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			out := tc.job.tick(tck)

			assert.Equal(t, tc.exp, out)
		})
	}
}

func Test_noopRequest(t *testing.T) {
	noop := noopRequest{}

	assert.Equal(t, context.Background(), noop.Context())
	assert.Equal(t, "", noop.Param(""))
	assert.Empty(t, noop.PathParam(""))
	assert.Equal(t, "gofr", noop.HostName())
	require.NoError(t, noop.Bind(nil))
	assert.Nil(t, noop.Params("test"))
}

func TestCron_parseRange(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]struct{}
		min      int
		max      int
		hasError bool
	}{
		{
			name:  "Valid Range",
			input: "1-5",
			expected: map[int]struct{}{
				1: {}, 2: {}, 3: {}, 4: {}, 5: {},
			},
			min: 1, max: 10,
			hasError: false,
		},
		{
			name:     "Out of Range",
			input:    "1-12",
			expected: nil,
			min:      1, max: 10,
			hasError: true,
		},
		{
			name:     "Invalid Input",
			input:    "a-b",
			expected: nil,
			min:      1, max: 10,
			hasError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := parseRange(test.input, test.min, test.max)
			if test.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, output, len(test.expected))

			assert.Equal(t, test.expected, output)
		})
	}
}

func TestCron_parseRange_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]struct{}
		min      int
		max      int
		hasError bool
	}{
		{
			name:  "Lower Boundary",
			input: "1-1",
			expected: map[int]struct{}{
				1: {},
			},
			min:      1,
			max:      10,
			hasError: false,
		},
		{
			name:  "Upper Boundary",
			input: "10-10",
			expected: map[int]struct{}{
				10: {},
			},
			min:      1,
			max:      10,
			hasError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := parseRange(test.input, test.min, test.max)
			if test.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.expected, output, "Expected: %v, got: %v", test.expected, output)
		})
	}
}

func TestCron_parsePart_InputFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]struct{}
		min      int
		max      int
		hasError bool
	}{
		{
			name:  "Valid Input with Multiple Values",
			input: "1,5,7",
			expected: map[int]struct{}{
				1: {}, 5: {}, 7: {},
			},
			min:      1,
			max:      10,
			hasError: false,
		},
		{
			name:     "Invalid Input Format",
			input:    "1,a,3",
			expected: nil,
			min:      1,
			max:      10,
			hasError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := parsePart(test.input, test.min, test.max)
			if test.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.expected, output)
		})
	}
}

func TestCron_parseRange_ErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		min      int
		max      int
		hasError bool
	}{
		{
			name:     "Empty String Input",
			input:    "",
			min:      1,
			max:      10,
			hasError: true,
		},
		{
			name:     "Out of Range Input",
			input:    "15-20",
			min:      1,
			max:      10,
			hasError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseRange(test.input, test.min, test.max)
			if test.hasError {
				require.Error(t, err, "Expected an error for input: %s", test.input)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCron_parseRange_SuccessCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]struct{}
		min      int
		max      int
	}{
		{
			name:  "Full Range",
			input: "1-10",
			expected: map[int]struct{}{
				1: {}, 2: {}, 3: {}, 4: {}, 5: {}, 6: {}, 7: {}, 8: {}, 9: {}, 10: {},
			},
			min: 1, max: 10,
		},
		{
			name:  "Partial Range",
			input: "5-7",
			expected: map[int]struct{}{
				5: {}, 6: {}, 7: {},
			},
			min: 1, max: 10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := parseRange(test.input, test.min, test.max)
			require.NoError(t, err)

			assert.Equal(t, test.expected, output)
		})
	}
}

func TestCron_parsePart(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]struct{}
		min      int
		max      int
		hasError bool
	}{
		{
			name:  "Single Value",
			input: "5",
			expected: map[int]struct{}{
				5: {},
			},
			min:      1,
			max:      10,
			hasError: false,
		},
		{
			name:  "Valid Multiple Values",
			input: "1,3,5",
			expected: map[int]struct{}{
				1: {}, 3: {}, 5: {},
			},
			min:      1,
			max:      10,
			hasError: false,
		},
		{
			name:     "Invalid Value",
			input:    "15",
			expected: nil,
			min:      1,
			max:      10,
			hasError: true,
		},
		{
			name:     "Invalid Format",
			input:    "1,2,a",
			expected: nil,
			min:      1,
			max:      10,
			hasError: true,
		},
	}

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := parsePart(test.input, test.min, test.max)
			if test.hasError {
				require.Error(t, err, "TEST[%d] - Expected error but got none", i)
			} else {
				require.NoError(t, err, "TEST[%d] - Expected no error but got: %v", i, err)
			}

			assert.Len(t, output, len(test.expected), "TEST[%d] - Expected length: %v, got: %v", i, len(test.expected), len(output))

			assert.Equal(t, test.expected, output, "TEST[%d] - Expected: %v, got: %v", i, test.expected, output)
		})
	}
}
