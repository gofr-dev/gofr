package gofr

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Crontab represents a job scheduling system that allows you to schedule and manage
// recurring tasks based on a cron-like syntax.
//
// The Crontab struct holds the necessary components for managing scheduled jobs.
// It uses a time.Ticker for periodic execution and maintains a list of scheduled jobs
// along with a mutex for concurrent access.
type Crontab struct {
	// contains unexported fields
	ticker *time.Ticker
	jobs   []job
	mu     sync.RWMutex
}

// job in cron table
type job struct {
	min       map[int]struct{}
	hour      map[int]struct{}
	day       map[int]struct{}
	month     map[int]struct{}
	dayOfWeek map[int]struct{}

	fn func()
}

// tick is individual tick that occurs each minute
type tick struct {
	min       int
	hour      int
	day       int
	month     int
	dayOfWeek int
}

// NewCron initializes and returns new cron table
func NewCron() *Crontab {
	c := &Crontab{
		ticker: time.NewTicker(time.Minute),
	}

	go func() {
		for t := range c.ticker.C {
			c.runScheduled(t)
		}
	}()

	return c
}

// this will compile the regex once instead of compiling it each time when it is being called.
var (
	matchSpaces      = regexp.MustCompile(`\s+`)
	matchN           = regexp.MustCompile(`(.*)/(\d+)`)
	matchRange       = regexp.MustCompile(`^(\d+)-(\d+)$`)
	ErrBadCronFormat = errors.New("schedule string must have five components like * * * * *")
)

// parseSchedule creates job struct with filled times to launch, or error if syntax is wrong
func parseSchedule(s string) (j job, err error) {
	const (
		maxMinute = 59
		maxHour   = 23
		maxDay    = 31
		maxMonth  = 12
		maxWeek   = 6
	)

	s = matchSpaces.ReplaceAllLiteralString(s, " ")

	parts := strings.Split(s, " ")

	const cronParts = 5

	if len(parts) != cronParts {
		return job{}, ErrBadCronFormat
	}

	j.min, err = parseParts(parts[0], 0, maxMinute)
	if err != nil {
		return j, err
	}

	j.hour, err = parseParts(parts[1], 0, maxHour)
	if err != nil {
		return j, err
	}

	j.day, err = parseParts(parts[2], 1, maxDay)
	if err != nil {
		return j, err
	}

	j.month, err = parseParts(parts[3], 1, maxMonth)
	if err != nil {
		return j, err
	}

	j.dayOfWeek, err = parseParts(parts[4], 0, maxWeek)
	if err != nil {
		return j, err
	}

	// day/dayOfWeek combination
	setDayFields(&j)

	return j, nil
}

func setDayFields(j *job) {
	switch {
	case len(j.day) < 31 && len(j.dayOfWeek) == 7: // day set, but not dayOfWeek, clear dayOfWeek
		j.dayOfWeek = make(map[int]struct{})
	case len(j.dayOfWeek) < 7 && len(j.day) == 31: // dayOfWeek set, but not day, clear day
		j.day = make(map[int]struct{})
	}
}

func setEmptyStructs(min, max, incr int) map[int]struct{} {
	r := make(map[int]struct{})

	for i := min; i <= max; i += incr {
		r[i] = struct{}{}
	}

	return r
}

func checkOutOfRange(min, max int, s, match string) error {
	if rng := matchRange.FindStringSubmatch(match); rng != nil {
		localMin, _ := strconv.Atoi(rng[1])
		localMax, _ := strconv.Atoi(rng[2])

		if localMin < min || localMax > max {
			return fmt.Errorf("out of range for %s in %s. %s must be in range %d-%d", rng[1], s, rng[1], min, max)
		}
	} else {
		return fmt.Errorf("unable to parse %s part in %s", match, s)
	}

	return nil
}

func recalculateMinMax(match string) (localMin, localMax int) {
	if rng := matchRange.FindStringSubmatch(match); rng != nil {
		localMin, _ = strconv.Atoi(rng[1])
		localMax, _ = strconv.Atoi(rng[2])
	}

	return
}

func matchRangeAndSlashes(s string, min, max int, matches []string) (map[int]struct{}, error) {
	localMin := min
	localMax := max

	if matches[1] != "" && matches[1] != "*" {
		if err := checkOutOfRange(min, max, s, matches[1]); err != nil {
			return nil, err
		}

		localMin, localMax = recalculateMinMax(matches[1])
	}

	n, _ := strconv.Atoi(matches[2])

	return setEmptyStructs(localMin, localMax, n), nil
}

func parseRange(min, max int, x, s string, rng []string, r map[int]struct{}) error {
	localMin, _ := strconv.Atoi(rng[1])
	localMax, _ := strconv.Atoi(rng[2])

	if localMin < min || localMax > max {
		return fmt.Errorf("out of range for %s in %s. %s must be in range %d-%d", x, s, x, min, max)
	}

	for i := localMin; i <= localMax; i++ {
		r[i] = struct{}{}
	}

	return nil
}

func parseCommas(s string, min, max int) (map[int]struct{}, error) {
	r := make(map[int]struct{})

	parts := strings.Split(s, ",")
	for _, x := range parts {
		rng := matchRange.FindStringSubmatch(x)
		if rng != nil {
			err := parseRange(min, max, x, s, rng, r)
			if err != nil {
				return nil, err
			}
		} else {
			err := parseCommaRangeCheck(x, s, max, min, r)
			if err != nil {
				return nil, err
			}
		}
	}

	return r, nil
}

func parseCommaRangeCheck(x, s string, max, min int, r map[int]struct{}) error {
	if i, err := strconv.Atoi(x); err == nil {
		err := checkRange(max, min, i, s)
		if err != nil {
			return err
		}

		r[i] = struct{}{}
	} else {
		return fmt.Errorf("unable to parse %s part in %s", x, s)
	}

	return nil
}

func checkRange(max, min, i int, s string) error {
	if i < min || i > max {
		return fmt.Errorf("out of range for %d in %s. %d must be in range %d-%d", i, s, i, min, max)
	}

	return nil
}

// parseParts parse individual schedule part from schedule string
func parseParts(s string, min, max int) (map[int]struct{}, error) {
	if s == "*" {
		return setEmptyStructs(min, max, 1), nil
	}

	// */2 1-59/5 pattern
	if matches := matchN.FindStringSubmatch(s); matches != nil {
		return matchRangeAndSlashes(s, min, max, matches)
	}

	// 1,2,4  or 1,2,10-15,20,30-45 pattern
	r, err := parseCommas(s, min, max)
	if err != nil {
		return nil, err
	}

	if len(r) == 0 {
		return nil, fmt.Errorf("unable to parse %s", s)
	}

	return r, nil
}

func (c *Crontab) runScheduled(t time.Time) {
	c.mu.Lock()

	n := len(c.jobs)
	jb := make([]job, n)
	copy(jb, c.jobs)

	c.mu.Unlock()

	for _, j := range jb {
		if j.tick(getTick(t)) {
			go j.fn()
		}
	}
}

// tick decides should the job be launched at the tick
func (j job) tick(t tick) bool {
	if _, ok := j.min[t.min]; !ok {
		return false
	}

	if _, ok := j.hour[t.hour]; !ok {
		return false
	}

	// cumulative day and dayOfWeek, as it should be
	_, day := j.day[t.day]
	_, dayOfWeek := j.dayOfWeek[t.dayOfWeek]

	if !day && !dayOfWeek {
		return false
	}

	if _, ok := j.month[t.month]; !ok {
		return false
	}

	return true
}

// AddJob to cron table, returns error if the cron syntax can't be parsed or is out of bounds
func (c *Crontab) AddJob(schedule string, fn func()) error {
	j, err := parseSchedule(schedule)
	if err != nil {
		return err
	}

	j.fn = fn

	c.mu.Lock()

	c.jobs = append(c.jobs, j)

	c.mu.Unlock()

	return nil
}

// getTick returns the tick struct from time
func getTick(t time.Time) tick {
	return tick{
		min:       t.Minute(),
		hour:      t.Hour(),
		day:       t.Day(),
		month:     int(t.Month()),
		dayOfWeek: int(t.Weekday()),
	}
}
