package gofr

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/version"
)

const (
	minutes       = 59
	hrs           = 23
	days          = 31
	months        = 12
	dayOfWeek     = 6
	scheduleParts = 5
)

type CronFunc func(ctx *Context)

// Crontab maintains the job scheduling and runs the jobs at their scheduled time by
// going through them at each tick using a ticker.
type Crontab struct {
	// contains unexported fields
	ticker    *time.Ticker
	jobs      []*job
	container *container.Container

	mu sync.RWMutex
}

type job struct {
	min       map[int]struct{}
	hour      map[int]struct{}
	day       map[int]struct{}
	month     map[int]struct{}
	dayOfWeek map[int]struct{}

	name string
	fn   CronFunc
}

type tick struct {
	min       int
	hour      int
	day       int
	month     int
	dayOfWeek int
}

// NewCron initializes and returns new cron tab.
func NewCron(cntnr *container.Container) *Crontab {
	c := &Crontab{
		ticker:    time.NewTicker(time.Minute),
		container: cntnr,
		jobs:      make([]*job, 0),
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
	matchSpaces = regexp.MustCompile(`\s+`)
	matchN      = regexp.MustCompile(`(.*)/(\d+)`)
	matchRange  = regexp.MustCompile(`^(\d+)-(\d+)$`)
)

// parseSchedule string and creates job struct with filled times to launch, or error if synthax is wrong.
func parseSchedule(s string) (*job, error) {
	var err error

	j := &job{}
	s = matchSpaces.ReplaceAllLiteralString(s, " ")
	s = strings.Trim(s, " ")
	parts := strings.Split(s, " ")

	if len(parts) != scheduleParts {
		return nil, errBadScheduleFormat
	}

	j.min, err = parsePart(parts[0], 0, minutes)
	if err != nil {
		return nil, err
	}

	j.hour, err = parsePart(parts[1], 0, hrs)
	if err != nil {
		return nil, err
	}

	j.day, err = parsePart(parts[2], 1, days)
	if err != nil {
		return nil, err
	}

	j.month, err = parsePart(parts[3], 1, months)
	if err != nil {
		return nil, err
	}

	j.dayOfWeek, err = parsePart(parts[4], 0, dayOfWeek)
	if err != nil {
		return nil, err
	}

	//  day/dayOfWeek combination
	mergeDays(j)

	return j, nil
}

func mergeDays(j *job) {
	switch {
	case len(j.day) < 31 && len(j.dayOfWeek) == 7: // day set, but not dayOfWeek, clear dayOfWeek
		j.dayOfWeek = make(map[int]struct{})
	case len(j.dayOfWeek) < 7 && len(j.day) == 31: // dayOfWeek set, but not day, clear day
		j.day = make(map[int]struct{})
	}
}

// parsePart parse individual schedule part from schedule string.
func parsePart(s string, min, max int) (map[int]struct{}, error) {
	// wildcard pattern
	if s == "*" {
		return getDefaultJobField(min, max, 1), nil
	}

	// */2 1-59/5 pattern
	if matches := matchN.FindStringSubmatch(s); matches != nil {
		return parseSteps(s, matches[1], matches[2], min, max)
	}

	// 1,2,4 or 1,2,10-15,20,30-45 pattern
	return parseRange(s, min, max)
}

func parseSteps(s, match1, match2 string, min, max int) (map[int]struct{}, error) {
	localMin := min
	localMax := max

	if match1 != "" && match1 != "*" {
		if rng := matchRange.FindStringSubmatch(match1); rng != nil {
			localMin, _ = strconv.Atoi(rng[1])
			localMax, _ = strconv.Atoi(rng[2])

			if localMin < min || localMax > max {
				return nil, errOutOfRange{rng[1], s, min, max}
			}
		} else {
			return nil, errParsing{match1, s}
		}
	}

	n, _ := strconv.Atoi(match2)

	return getDefaultJobField(localMin, localMax, n), nil
}

func parseRange(s string, min, max int) (map[int]struct{}, error) {
	r := make(map[int]struct{})
	parts := strings.Split(s, ",")

	for _, x := range parts {
		if rng := matchRange.FindStringSubmatch(x); rng != nil {
			localMin, _ := strconv.Atoi(rng[1])
			localMax, _ := strconv.Atoi(rng[2])

			if localMin < min || localMax > max {
				return nil, errOutOfRange{x, s, min, max}
			}

			r = getDefaultJobField(localMin, localMax, 1)
		} else if i, err := strconv.Atoi(x); err == nil {
			if i < min || i > max {
				return nil, errOutOfRange{i, s, min, max}
			}

			r[i] = struct{}{}
		} else {
			return nil, errParsing{x, s}
		}
	}

	if len(r) == 0 {
		return nil, errParsing{invalidPart: s}
	}

	return r, nil
}

func getDefaultJobField(min, max, incr int) map[int]struct{} {
	r := make(map[int]struct{})

	for i := min; i <= max; i += incr {
		r[i] = struct{}{}
	}

	return r
}

func (c *Crontab) runScheduled(t time.Time) {
	c.mu.Lock()

	n := len(c.jobs)
	jb := make([]*job, n)
	copy(jb, c.jobs)

	c.mu.Unlock()

	for _, j := range jb {
		if j.tick(getTick(t)) {
			go j.run(c.container)
		}
	}
}

func getTick(t time.Time) *tick {
	return &tick{
		min:       t.Minute(),
		hour:      t.Hour(),
		day:       t.Day(),
		month:     int(t.Month()),
		dayOfWeek: int(t.Weekday()),
	}
}

func (j *job) run(cntnr *container.Container) {
	ctx, span := otel.GetTracerProvider().Tracer("gofr-"+version.Framework).
		Start(context.Background(), j.name)
	defer span.End()

	j.fn(&Context{
		Context:   ctx,
		Container: cntnr,
		Request:   noopRequest{},
	})
}

func (j *job) tick(t *tick) bool {
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

// AddJob to cron tab, returns error if the cron syntax can't be parsed or is out of bounds.
func (c *Crontab) AddJob(schedule, jobName string, fn CronFunc) error {
	j, err := parseSchedule(schedule)
	if err != nil {
		return err
	}

	j.name = jobName
	j.fn = fn

	c.mu.Lock()
	c.jobs = append(c.jobs, j)
	c.mu.Unlock()

	return nil
}

var errBadScheduleFormat = errors.New("schedule string must have five components like * * * * *")

// errOutOfRange denotes the errors that occur when a range in schedule is out of scope for the particular time unit.
type errOutOfRange struct {
	rangeVal interface{}
	input    string
	min, max int
}

func (e errOutOfRange) Error() string {
	return fmt.Sprintf("out of range for %s in %s. %s must be in "+
		"range %d-%d", e.rangeVal, e.input, e.rangeVal, e.min, e.max)
}

type errParsing struct {
	invalidPart string
	base        string
}

func (e errParsing) Error() string {
	if e.base != "" {
		return fmt.Sprintf("unable to parse %s part in %s", e.invalidPart, e.base)
	}

	return fmt.Sprintf("unable to parse %s", e.invalidPart)
}

// noopRequest is a non-operating implementation of Request intreface
// this is required to prevent panics while executing cron jobs.
type noopRequest struct {
}

func (b noopRequest) Context() context.Context {
	return context.Background()
}

func (b noopRequest) Param(string) string {
	return ""
}

func (b noopRequest) PathParam(string) string {
	return ""
}

func (b noopRequest) HostName() string {
	return "gofr"
}

func (b noopRequest) Bind(interface{}) error {
	return nil
}
