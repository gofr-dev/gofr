package service

import (
	"math"
	"sync"
	"time"
)

const (
	defaultRetryBudgetTTL          = 10 * time.Second
	defaultRetryBudgetRatio        = 0.2
	defaultRetryBudgetMinPerSecond = 10
)

// RetryBudget caps how many retries a RetryConfig may make, as a share of the calls made to the service.
//
// Over the last TTL, a retry is allowed while
//
//	retries + 1 <= Ratio × first attempts + MinPerSecond × TTL in whole seconds
//
// Ratio keeps retries proportional to traffic: with Ratio 0.2, retries add at most 20% load to an
// upstream that is failing every call. MinPerSecond lets a service with little traffic still retry.
//
// TTL is counted in whole seconds, rounded up, and at least 1: a TTL of 300ms counts as 1s, both for
// the window and for MinPerSecond × TTL.
//
// A Ratio that is negative or NaN counts as 0. A budget with both Ratio and MinPerSecond 0 would allow
// no retry at all, so it uses Ratio 0.2 and MinPerSecond 10 instead. To make no retries, leave
// RetryConfig.MaxRetries at 0.
//
// A retry is taken from the budget before the wait for it, so concurrent calls never all wait for the one
// retry left; a wait cut short by the caller's context gives it back.
//
// Each service a RetryConfig is added to keeps its own counts; one RetryBudget value can be shared.
type RetryBudget struct {
	// Ratio is the number of retries allowed per first attempt, e.g. 0.2.
	Ratio float64
	// MinPerSecond is the number of retries allowed per second regardless of traffic.
	MinPerSecond int
	// TTL is how long first attempts and retries are counted. Default 10s.
	TTL time.Duration
}

// retryBudget is the per-service state of a RetryBudget: counts in one-second slots, kept for TTL.
type retryBudget struct {
	mu    sync.Mutex
	ratio float64
	floor float64
	slots []budgetSlot
	now   func() time.Time
	start time.Time
}

type budgetSlot struct {
	second      int64
	deposits    int
	withdrawals int
}

func newRetryBudget(cfg RetryBudget, now func() time.Time) *retryBudget {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultRetryBudgetTTL
	}

	seconds := max(int(math.Ceil(ttl.Seconds())), 1)

	// "> 0" rather than max(Ratio, 0), which keeps a NaN: no retry would then ever fit.
	ratio, minPerSecond := 0.0, max(cfg.MinPerSecond, 0)
	if cfg.Ratio > 0 {
		ratio = cfg.Ratio
	}

	if ratio == 0 && minPerSecond == 0 {
		ratio, minPerSecond = defaultRetryBudgetRatio, defaultRetryBudgetMinPerSecond
	}

	return &retryBudget{
		ratio: ratio,
		floor: float64(minPerSecond * seconds),
		slots: make([]budgetSlot, seconds),
		now:   now,
		start: now(),
	}
}

// deposit records a first attempt.
func (b *retryBudget) deposit() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.current().deposits++
}

// allows reports whether the budget would allow a retry now, without recording one.
func (b *retryBudget) allows() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.hasRoom(b.current())
}

// tryWithdraw records a retry and reports true if the budget allows it; otherwise it records nothing.
// second is the slot the retry was recorded in, for refund.
func (b *retryBudget) tryWithdraw() (second int64, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	slot := b.current()
	if !b.hasRoom(slot) {
		return 0, false
	}

	slot.withdrawals++

	return slot.second, true
}

// refund takes back a retry recorded in second that was not made. A retry already out of the window
// has nothing left to take back.
func (b *retryBudget) refund(second int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if s := &b.slots[second%int64(len(b.slots))]; s.second == second && s.withdrawals > 0 {
		s.withdrawals--
	}
}

// hasRoom reports whether one more retry fits, counting the slots of the last TTL up to current.
func (b *retryBudget) hasRoom(current *budgetSlot) bool {
	sec := current.second

	var deposits, withdrawals int

	for i := range b.slots {
		if s := &b.slots[i]; sec-s.second < int64(len(b.slots)) {
			deposits += s.deposits
			withdrawals += s.withdrawals
		}
	}

	return float64(withdrawals+1) <= b.ratio*float64(deposits)+b.floor
}

// current returns the slot of the current second, clearing it if it last held an older second. Seconds
// are counted from the budget's creation, on the monotonic clock, so a wall-clock step does not move them.
func (b *retryBudget) current() *budgetSlot {
	sec := int64(b.now().Sub(b.start) / time.Second)

	s := &b.slots[sec%int64(len(b.slots))]
	if s.second != sec {
		*s = budgetSlot{second: sec}
	}

	return s
}
