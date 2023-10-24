package gofr

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//nolint:gocognit // need to check for multiple fields
func TestSchedule(t *testing.T) {
	var schTest = []struct {
		s   string
		cnt [5]int
	}{
		{"* * * * *", [5]int{60, 24, 31, 12, 7}},
		{"*/2 * * * *", [5]int{30, 24, 31, 12, 7}},
		{"*/10 * * * *", [5]int{6, 24, 31, 12, 7}},
		{"* * * * */2", [5]int{60, 24, 0, 12, 4}},
		{"5,8,9 */2 2,3 * */2", [5]int{3, 12, 2, 12, 4}},
		{"* 5-11 2-30/2 * *", [5]int{60, 7, 15, 12, 0}},
		{"1,2,5-8 * * */3 *", [5]int{6, 24, 31, 4, 7}},
	}

	for _, sch := range schTest {
		j, err := parseSchedule(sch.s)
		if err != nil {
			t.Error(err)
		}

		if len(j.min) != sch.cnt[0] {
			t.Error(sch.s, "min count expected to be", sch.cnt[0], "result", len(j.min), j.min)
		}

		if len(j.hour) != sch.cnt[1] {
			t.Error(sch.s, "hour count expected to be", sch.cnt[1], "result", len(j.hour), j.hour)
		}

		if len(j.day) != sch.cnt[2] {
			t.Error(sch.s, "day count expected to be", sch.cnt[2], "result", len(j.day), j.day)
		}

		if len(j.month) != sch.cnt[3] {
			t.Error(sch.s, "month count expected to be", sch.cnt[3], "result", len(j.month), j.month)
		}

		if len(j.dayOfWeek) != sch.cnt[4] {
			t.Error(sch.s, "dayOfWeek count expected to be", sch.cnt[4], "result", len(j.dayOfWeek), j.dayOfWeek)
		}
	}
}

// TestScheduleError tests crontab syntax which should not be accepted
func TestScheduleError(t *testing.T) {
	var schErrorTest = []string{
		"* * * * * *",
		"0-70 * * * *",
		"* 0-30 * * *",
		"* * 0-10 * *",
		"* * 0,1,2 * *",
		"* * 1-40/2 * *",
		"* * ab/2 * *",
		"* * * 1-15 *",
		"* * * * 7,8,9",
		"1 2 3 4 5 6",
		"* 1,2/10 * * *",
		"* * 1,2,3,1-15/10 * *",
		"a b c d e",
	}

	for _, s := range schErrorTest {
		if _, err := parseSchedule(s); err == nil {
			t.Error(s, "should be error", err)
		}
	}
}

func Test_setEmptyStructs(t *testing.T) {
	tcs := []struct {
		min  int
		max  int
		incr int
		len  int
	}{
		{0, 59, 1, 60},
		{0, 23, 1, 24},
	}

	for _, tc := range tcs {
		if got := setEmptyStructs(tc.min, tc.max, tc.incr); len(got) != tc.len {
			t.Errorf("FAILED, Expected: %v, Got: %v", tc.len, len(got))
		}
	}
}

func Test_job_tick(t *testing.T) {
	temp := map[int]struct{}{1: {}}

	type fields struct {
		min       map[int]struct{}
		hour      map[int]struct{}
		day       map[int]struct{}
		month     map[int]struct{}
		dayOfWeek map[int]struct{}
		fn        func()
	}

	type args struct {
		t tick
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{"minute false", fields{}, args{}, false},
		{"hour false", fields{min: temp}, args{t: tick{min: 1}}, false},
		{"day false", fields{min: temp, hour: temp}, args{t: tick{min: 1, hour: 1}}, false},
		{"month false", fields{min: temp, hour: temp, day: temp, dayOfWeek: temp},
			args{t: tick{min: 1, hour: 1, dayOfWeek: 1, day: 1}}, false},
		{"true", fields{min: temp, hour: temp, day: temp, dayOfWeek: temp, month: temp},
			args{t: tick{min: 1, hour: 1, dayOfWeek: 1, day: 1, month: 1}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := job{
				min:       tt.fields.min,
				hour:      tt.fields.hour,
				day:       tt.fields.day,
				month:     tt.fields.month,
				dayOfWeek: tt.fields.dayOfWeek,
				fn:        tt.fields.fn,
			}

			if got := j.tick(tt.args.t); got != tt.want {
				t.Errorf("tick() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTick(t *testing.T) {
	args := time.Time{}

	exp := tick{min: 0, hour: 0, day: 1, month: 1, dayOfWeek: 1}

	resp := getTick(args)

	assert.Equal(t, exp, resp, "Test case failed")
}

func TestCrontab_AddJob(t *testing.T) {
	c := Crontab{}
	testcases := []struct {
		desc     string
		schedule string
		expErr   error
	}{
		{"with schedule string", "* * * * *", nil},
		{"without schedule string", "", errors.New("schedule string must have five components like * * * * *")},
	}

	for i, tc := range testcases {
		err := c.AddJob(tc.schedule, func() {})

		assert.Equal(t, tc.expErr, err, "Test case [%d] failed.", i)
	}
}
