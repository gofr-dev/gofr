package gofr

import (
	"context"
	"errors"
	"fmt"
	"gofr.dev/pkg/gofr/container"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CronFunc func(ctx *Context)

// Crontab maintains the job scheduling and runs the jobs at their scheduled time by
// going through them at each tick using a ticker
type Crontab struct {
	// contains unexported fields
	ticker    *time.Ticker
	jobs      []*job
	contianer *container.Container
	stopCh    chan struct{}

	sync.RWMutex
}

type job struct {
	min       map[int]struct{}
	hour      map[int]struct{}
	day       map[int]struct{}
	month     map[int]struct{}
	dayOfWeek map[int]struct{}

	fn CronFunc
}

type tick struct {
	min       int
	hour      int
	day       int
	month     int
	dayOfWeek int
}

// NewCron initializes and returns new cron tab
func NewCron(container *container.Container) *Crontab {
	c := &Crontab{
		ticker:    time.NewTicker(time.Minute),
		contianer: container,
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

// parseSchedule string and creates job struct with filled times to launch, or error if synthax is wrong
func parseSchedule(s string) (*job, error) {
	var err error
	j := &job{}
	s = matchSpaces.ReplaceAllLiteralString(s, " ")
	parts := strings.Split(s, " ")
	if len(parts) != 5 {
		return j, errors.New("Schedule string must have five components like * * * * *")
	}

	j.min, err = parsePart(parts[0], 0, 59)
	if err != nil {
		return j, err
	}

	j.hour, err = parsePart(parts[1], 0, 23)
	if err != nil {
		return j, err
	}

	j.day, err = parsePart(parts[2], 1, 31)
	if err != nil {
		return j, err
	}

	j.month, err = parsePart(parts[3], 1, 12)
	if err != nil {
		return j, err
	}

	j.dayOfWeek, err = parsePart(parts[4], 0, 6)
	if err != nil {
		return j, err
	}

	//  day/dayOfWeek combination
	switch {
	case len(j.day) < 31 && len(j.dayOfWeek) == 7: // day set, but not dayOfWeek, clear dayOfWeek
		j.dayOfWeek = make(map[int]struct{})
	case len(j.dayOfWeek) < 7 && len(j.day) == 31: // dayOfWeek set, but not day, clear day
		j.day = make(map[int]struct{})
	}

	return j, nil
}

// parsePart parse individual schedule part from schedule string
func parsePart(s string, min, max int) (map[int]struct{}, error) {
	// wildcard pattern
	if s == "*" {
		return getDefaultJobField(min, max, 1), nil
	}

	// */2 1-59/5 pattern
	if matches := matchN.FindStringSubmatch(s); matches != nil {
		localMin := min
		localMax := max
		if matches[1] != "" && matches[1] != "*" {
			if rng := matchRange.FindStringSubmatch(matches[1]); rng != nil {
				localMin, _ = strconv.Atoi(rng[1])
				localMax, _ = strconv.Atoi(rng[2])
				if localMin < min || localMax > max {
					return nil, fmt.Errorf("Out of range for %s in %s. %s must be in range %d-%d", rng[1], s, rng[1], min, max)
				}
			} else {
				return nil, fmt.Errorf("Unable to parse %s part in %s", matches[1], s)
			}
		}
		n, _ := strconv.Atoi(matches[2])
		return getDefaultJobField(localMin, localMax, n), nil
	}

	// 1,2,4 or 1,2,10-15,20,30-45 pattern
	parts := strings.Split(s, ",")
	var r map[int]struct{}
	for _, x := range parts {
		if rng := matchRange.FindStringSubmatch(x); rng != nil {
			localMin, _ := strconv.Atoi(rng[1])
			localMax, _ := strconv.Atoi(rng[2])

			if localMin < min || localMax > max {
				return nil, fmt.Errorf("Out of range for %s in %s. %s must be in range %d-%d", x, s, x, min, max)
			}

			r = getDefaultJobField(localMin, localMax, 1)
		} else if i, err := strconv.Atoi(x); err == nil {
			if i < min || i > max {
				return nil, fmt.Errorf("Out of range for %d in %s. %d must be in range %d-%d", i, s, i, min, max)
			}

			r[i] = struct{}{}
		} else {
			return nil, fmt.Errorf("Unable to parse %s part in %s", x, s)
		}
	}

	if len(r) == 0 {
		return nil, fmt.Errorf("Unable to parse %s", s)
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
	c.Lock()

	n := len(c.jobs)
	jb := make([]*job, n)
	copy(jb, c.jobs)

	c.Unlock()

	for _, j := range jb {
		if j.tick(getTick(t)) {
			go j.fn(&Context{
				Context:   context.Background(),
				Container: c.contianer,
			})
		}
	}
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

// AddJob to cron tab, returns error if the cron syntax can't be parsed or is out of bounds
func (c *Crontab) AddJob(schedule string, fn CronFunc) error {
	j, err := parseSchedule(schedule)
	if err != nil {
		return err
	}

	j.fn = fn

	c.Lock()

	c.jobs = append(c.jobs, j)

	c.Unlock()

	return nil
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
