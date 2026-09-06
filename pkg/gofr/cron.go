package gofr

import (
	"context"
	"errors"
	"fmt"
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
	// stopped is set under mu before done is closed, so a tick that is already
	// inside runScheduled cannot dispatch a new job goroutine after Stop has
	// begun waiting for the in-flight ones.
	stopped bool
	// wg tracks the job goroutines dispatched by runScheduled so Stop can join
	// them instead of leaving them to log/record metrics after their owner
	// (an App, or a test's container) has been torn down.
	wg sync.WaitGroup

	done chan struct{}
	once sync.Once
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
		done:      make(chan struct{}),
	}

	c.registerMetrics()

	go func() {
		for {
			select {
			case t := <-c.ticker.C:
				c.runScheduled(t)
			case <-c.done:
				return
			}
		}
	}()

	return c
}

// Stop halts the scheduler and blocks until every job goroutine it already
// dispatched has returned. Joining matters as much as halting: a job that fired
// on the tick just before Stop keeps logging and recording metrics against a
// container that the caller is about to tear down.
func (c *Crontab) Stop() {
	c.stop(nil)
}

// stop is Stop with an optional abort channel for the join, so an application
// shutdown is never held open past its own deadline by a long-running job.
func (c *Crontab) stop(abort <-chan struct{}) {
	c.once.Do(func() {
		c.ticker.Stop()

		c.mu.Lock()
		c.stopped = true
		c.mu.Unlock()

		close(c.done)
	})

	joined := make(chan struct{})

	go func() {
		c.wg.Wait()
		close(joined)
	}()

	select {
	case <-joined:
	case <-abort:
	}
}

func (c *Crontab) runScheduled(t time.Time) {
	// The lock is held across dispatch so that Stop cannot slip between the
	// stopped check and wg.Add. Dispatch only spawns goroutines, and a job that
	// calls back into AddJob does so from its own goroutine, so this cannot
	// deadlock.
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}

	tk := getTick(t)

	for _, j := range c.jobs {
		if !j.tick(tk) {
			continue
		}

		c.wg.Add(1)

		go func(j *job) {
			defer c.wg.Done()

			j.run(c.container)
		}(j)
	}
}

func (j *job) run(cntnr *container.Container) {
	// A job runs on its own goroutine, detached from whoever scheduled it, so a
	// container that is nil or half-built (no logger) has to be handled here
	// rather than assumed away — every log below would otherwise nil-panic on a
	// background goroutine and take the whole process down.
	if cntnr == nil || cntnr.Logger == nil {
		return
	}

	ctx, span := otel.GetTracerProvider().Tracer("gofr-"+version.Framework).
		Start(context.Background(), j.name)
	defer span.End()

	c := newContext(nil, &noopRequest{}, cntnr)
	c.Context = ctx

	c.Infof("Starting cron job: %s", j.name)

	start := time.Now()

	defer func() {
		duration := time.Since(start)

		if m := cntnr.Metrics(); m != nil {
			m.RecordHistogram(ctx, "app_cron_job_duration", duration.Seconds(), "job", j.name)

			if r := recover(); r != nil {
				c.Errorf("Panic in cron job %s: %v", j.name, r)
				m.IncrementCounter(ctx, "app_cron_job_failures", "job", j.name)
			} else {
				m.IncrementCounter(ctx, "app_cron_job_success", "job", j.name)
			}
		} else if r := recover(); r != nil {
			c.Errorf("Panic in cron job %s: %v", j.name, r)
		}

		c.Infof("Finished cron job: %s in %s", j.name, duration)
	}()

	if m := cntnr.Metrics(); m != nil {
		m.IncrementCounter(ctx, "app_cron_job_total", "job", j.name)
	}

	j.fn(c)
}

func (c *Crontab) registerMetrics() {
	m := c.container.Metrics()
	if m == nil {
		return
	}

	cronJobHistogramBuckets := []float64{.05, .1, .5, 1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600}
	m.NewHistogram(
		"app_cron_job_duration",
		"Duration of cron job execution in seconds",
		cronJobHistogramBuckets...,
	)
	m.NewCounter("app_cron_job_total", "Total number of cron job executions")
	m.NewCounter("app_cron_job_success", "Number of successful cron job executions")
	m.NewCounter("app_cron_job_failures", "Number of failed cron job executions")
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
	rangeVal any
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

func (noopRequest) Bind(any) error {
	return nil
}

func (noopRequest) Params(string) []string {
	return nil
}
