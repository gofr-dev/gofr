package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeClock is a settable clock for the retry budget.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestRetryBudget_Window(t *testing.T) {
	// step is one budget operation: advance the clock, make deposits, then attempt withdrawals.
	type step struct {
		advance   time.Duration
		deposits  int
		withdraws int
		want      int // withdrawals allowed in this step
	}

	tests := []struct {
		desc   string
		budget RetryBudget
		steps  []step
	}{
		{desc: "ratio of first attempts", budget: RetryBudget{Ratio: 0.5, TTL: 2 * time.Second},
			steps: []step{{deposits: 4, withdraws: 3, want: 2}}},
		{desc: "no deposits and no floor allows nothing", budget: RetryBudget{Ratio: 1},
			steps: []step{{withdraws: 1, want: 0}}},
		{desc: "deposits expire after TTL", budget: RetryBudget{Ratio: 0.5, TTL: 2 * time.Second},
			steps: []step{
				{deposits: 4, withdraws: 2, want: 2},
				{advance: 2 * time.Second, withdraws: 1, want: 0},
				{deposits: 2, withdraws: 2, want: 1},
			}},
		{desc: "deposits still count inside TTL", budget: RetryBudget{Ratio: 0.5, TTL: 2 * time.Second},
			steps: []step{
				{deposits: 4},
				{advance: 1500 * time.Millisecond, withdraws: 3, want: 2},
			}},
		{desc: "withdrawals expire after TTL", budget: RetryBudget{Ratio: 0.5, TTL: 2 * time.Second},
			steps: []step{
				{deposits: 2, withdraws: 1, want: 1},
				{advance: time.Second, deposits: 2, withdraws: 2, want: 1},
				{advance: time.Second, withdraws: 1, want: 0}, // first second gone: 2 deposits, 1 withdrawal
				{deposits: 2, withdraws: 1, want: 1},
			}},
		{desc: "MinPerSecond floor over TTL", budget: RetryBudget{MinPerSecond: 2, TTL: 3 * time.Second},
			steps: []step{
				{withdraws: 7, want: 6},
				{advance: 3 * time.Second, withdraws: 7, want: 6},
			}},
		{desc: "TTL defaults to 10s", budget: RetryBudget{MinPerSecond: 1},
			steps: []step{{withdraws: 11, want: 10}}},
		{desc: "floor and ratio add up", budget: RetryBudget{Ratio: 0.1, MinPerSecond: 1, TTL: time.Second},
			steps: []step{{deposits: 20, withdraws: 5, want: 3}}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			clock := &fakeClock{t: time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)}
			b := newRetryBudget(tc.budget, clock.now)

			for i, s := range tc.steps {
				clock.advance(s.advance)

				for range s.deposits {
					b.deposit()
				}

				got := 0

				for range s.withdraws {
					if b.tryWithdraw() {
						got++
					}
				}

				assert.Equal(t, s.want, got, "step %d", i)
			}
		})
	}
}

func TestRetryBudget_Concurrent(t *testing.T) {
	const (
		workers   = 50
		perWorker = 20
		ratio     = 0.2
		floor     = 5
	)

	clock := &fakeClock{t: time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)}
	b := newRetryBudget(RetryBudget{Ratio: ratio, MinPerSecond: floor, TTL: time.Second}, clock.now)

	var (
		wg      sync.WaitGroup
		allowed atomic.Int32
	)

	for range workers {
		wg.Go(func() {
			for range perWorker {
				b.deposit()

				if b.tryWithdraw() {
					allowed.Add(1)
				}
			}
		})
	}

	wg.Wait()

	limit := ratio*workers*perWorker + floor
	assert.InDelta(t, limit, float64(allowed.Load()), 1, "retries must stay within Ratio x first attempts + floor")
}
