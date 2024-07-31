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
			schedule: "* * * * *",
			expJob: &job{
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
			schedules:    []string{"* * * * ", "* * * * * *"},
			expErrString: "schedule string must have five components like * * * * *",
		},
		{
			desc: "incorrect range",
			schedules: []string{
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
	expTick := &tick{20, 13, 10, 5, 5}

	tM := time.Date(2024, 5, 10, 13, 20, 1, 1, time.Local)

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

	// Populate the job arroy for cron table
	c.jobs = []*job{j}

	out := testutil.StdoutOutputForFunc(func() {
		c.runScheduled(time.Date(2024, 1, 1, 1, 1, 1, 1, time.Local))

		// block the main go routine to let the cron run
		time.Sleep(2 * time.Second)
	})

	assert.Contains(t, out, "hello from cron")
}

func TestJob_tick(t *testing.T) {
	tck := &tick{1, 1, 1, 1, 1}

	testCases := []struct {
		desc string
		job  *job
		exp  bool
	}{
		{
			desc: "min not matching",
			job:  &job{min: map[int]struct{}{2: {}}},
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
			desc: "job scheduled on the tick",
			job: &job{
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
	assert.Equal(t, "", noop.PathParam(""))
	assert.Equal(t, "gofr", noop.HostName())
	require.NoError(t, noop.Bind(nil))
}
