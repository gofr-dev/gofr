package logjson

import (
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLocations(t *testing.T) []*time.Location {
	t.Helper()

	locs := []*time.Location{
		time.UTC, time.Local,
		time.FixedZone("IST", 5*3600+30*60), time.FixedZone("W", -8*3600), time.FixedZone("LINT", 14*3600),
		time.FixedZone("NPT", 5*3600+45*60), time.FixedZone("S", 3600+30), time.FixedZone("Z0", 0),
	}

	for _, name := range []string{"America/New_York", "Europe/London", "Australia/Lord_Howe"} {
		if l, err := time.LoadLocation(name); err == nil {
			locs = append(locs, l)
		}
	}

	return locs
}

// requireRFC3339Nano checks one call against time's own formatting, once from an empty cache
// (the render path) and once more (the cached path).
func requireRFC3339Nano(t *testing.T, tm time.Time) {
	t.Helper()

	want := tm.AppendFormat([]byte("x"), time.RFC3339Nano)

	lastSecond.Store(nil)
	require.Equal(t, string(want), string(AppendRFC3339Nano([]byte("x"), tm)), "render %v", tm)
	require.Equal(t, string(want), string(AppendRFC3339Nano([]byte("x"), tm)), "cached %v", tm)
}

func TestAppendRFC3339NanoMatchesAppendFormat(t *testing.T) {
	nanos := []int{0, 1, 10, 100, 120000000, 123456789, 999999999, 500000000, 1000}

	for _, loc := range testLocations(t) {
		for _, ns := range nanos {
			for _, tm := range []time.Time{
				time.Date(2026, 11, 5, 9, 0, 0, ns, loc),
				time.Date(1, 1, 1, 0, 0, 0, ns, loc),
				time.Date(1969, 12, 31, 23, 59, 59, ns, loc),
				time.Date(9999, 12, 31, 23, 59, 59, ns, loc),
				time.Date(0, 1, 1, 0, 0, 0, ns, loc),
				time.Date(10000, 1, 1, 0, 0, 0, ns, loc),
				time.Date(-5, 3, 1, 0, 0, 0, ns, loc),
				time.Now().In(loc),
			} {
				requireRFC3339Nano(t, tm)
			}
		}
	}

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // reproducible test inputs, not security.
	locs := testLocations(t)

	for range 20000 {
		loc := locs[rng.IntN(len(locs))]
		requireRFC3339Nano(t, time.Unix(rng.Int64N(400e9)-62e9, rng.Int64N(1e9)).In(loc))
	}
}

// TestAppendRFC3339NanoAcrossSeconds walks the clock forward in steps that cross second, minute,
// day and DST boundaries without resetting the cache, as a running logger does, and checks every
// call, including times in an older second than the cache holds.
func TestAppendRFC3339NanoAcrossSeconds(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		ny = time.FixedZone("EST", -5*3600)
	}

	for _, loc := range append(testLocations(t), ny) {
		lastSecond.Store(nil)

		// 2026-03-08 06:59:58 UTC is just before New York's 2 a.m. spring-forward.
		tm := time.Date(2026, 3, 8, 6, 59, 58, 0, time.UTC).In(loc)

		for i := range 5000 {
			tm = tm.Add(170 * time.Millisecond)

			assert.Equal(t, tm.Format(time.RFC3339Nano), string(AppendRFC3339Nano(nil, tm)), "step %d", i)

			older := tm.Add(-time.Second)
			assert.Equal(t, older.Format(time.RFC3339Nano), string(AppendRFC3339Nano(nil, older)), "older %d", i)
		}
	}
}

// TestAppendRFC3339NanoConcurrent has loggers in different locations format the moving clock at
// once; -race checks the shared cache, and every result is compared with time's own formatting.
func TestAppendRFC3339NanoConcurrent(t *testing.T) {
	lastSecond.Store(nil)

	var wg sync.WaitGroup

	for _, loc := range testLocations(t) {
		wg.Go(func() {
			buf := make([]byte, 0, 64)

			for range 20000 {
				tm := time.Now().In(loc)

				buf = AppendRFC3339Nano(buf[:0], tm)
				if got, want := string(buf), tm.Format(time.RFC3339Nano); got != want {
					t.Errorf("got %s, want %s", got, want)
					return
				}
			}
		})
	}

	wg.Wait()
}

func TestAppendRFC3339NanoDoesNotAllocate(t *testing.T) {
	tm := time.Date(2026, 11, 5, 9, 0, 0, 123456789, time.FixedZone("IST", 5*3600+30*60))
	AppendRFC3339Nano(nil, tm)

	buf := make([]byte, 0, 64)

	assert.Zero(t, testing.AllocsPerRun(100, func() { buf = AppendRFC3339Nano(buf[:0], tm) }))
}

func BenchmarkAppendRFC3339Nano(b *testing.B) {
	tm := time.Now()
	buf := make([]byte, 0, 64)

	b.Run("cached", func(b *testing.B) {
		for b.Loop() {
			buf = AppendRFC3339Nano(buf[:0], tm)
		}
	})

	b.Run("AppendFormat", func(b *testing.B) {
		for b.Loop() {
			buf = tm.AppendFormat(buf[:0], time.RFC3339Nano)
		}
	})
}
