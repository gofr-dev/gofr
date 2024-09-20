package gofr

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/version"
)

const (
	seconds                 = 59
	minutes                 = 59
	hrs                     = 23
	days                    = 31
	months                  = 12
	dayOfWeek               = 6
	scheduleParts           = 5
	schedulePartsWithSecond = 6
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
	sec       map[int]struct{}
	min       map[int]struct{}
	hour      map[int]struct{}
	day       map[int]struct{}
	month     map[int]struct{}
	dayOfWeek map[int]struct{}

	name string
	fn   CronFunc
}

type tick struct {
	sec       int
	min       int
	hour      int
	day       int
	month     int
	dayOfWeek int
}

// NewCron initializes and returns new cron tab.
func NewCron(cntnr *container.Container) *Crontab {
	c := &Crontab{
		ticker:    time.NewTicker(time.Second),
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
		sec:       t.Second(),
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

	if j.sec != nil {
		if _, ok := j.sec[t.sec]; !ok {
			return false
		}
	} else {
		if t.sec != 0 {
			return false
		}
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

// noopRequest is a non-operating implementation of Request interface
// this is required to prevent panics while executing cron jobs.
type noopRequest struct {
}

func (noopRequest) Context() context.Context {
	return context.Background()
}

func (noopRequest) Param(string) string {
	return ""
}

func (noopRequest) PathParam(string) string {
	return ""
}

func (noopRequest) HostName() string {
	return "gofr"
}

func (noopRequest) Bind(interface{}) error {
	return nil
}

func (noopRequest) Params(string) []string {
	return nil
}
