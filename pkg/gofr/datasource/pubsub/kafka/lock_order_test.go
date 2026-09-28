package kafka

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// TestHealthSubscribe_NoLockOrderInversion guards the order in which k.mu and
// connMu are taken.
//
// Subscribe holds k.mu across getNewReader, which takes connMu - so k.mu comes
// first, and every other path must use that order too. Health briefly broke it:
// it held connMu for the whole probe and then took k.mu inside
// getReaderStatsAsMap. Go's RWMutex parks new readers once a writer is waiting,
// so a concurrent connMu writer closes the cycle:
//
//	Health   holds connMu(R), wants k.mu
//	Subscribe holds k.mu,     wants connMu(R)  - parked behind the writer
//	writer                    wants connMu(W)  - waits on Health
//
// The subscribe side is modeled rather than called: the real Subscribe builds
// a kafka.Reader against an unresolvable broker inside the lock, which makes
// the same test take tens of seconds and assert on a deadline. What matters
// here is only the acquisition order, so it is reproduced exactly and cheaply.
// The production paths themselves are covered by the *_ConcurrentSubscribe_
// race tests.
func TestHealthSubscribe_NoLockOrderInversion(t *testing.T) {
	const iterations = 2000

	k := &kafkaClient{
		conn: &multiConn{
			conns: []Connection{&MockConn{addr: "127.0.0.1:9092", isHealthy: true, isControl: true}},
		},
		reader: make(map[string]Reader),
		writer: &mockWriter{},
		logger: &mockLogger{},
	}

	for i := range 2 {
		k.reader["seed-"+strconv.Itoa(i)] = &statsReader{}
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup

		wg.Add(3)

		go func() { // Subscribe's order: k.mu -> connMu (via getNewReader)
			defer wg.Done()

			for i := range iterations {
				k.mu.Lock()
				k.connMu.RLock()
				// Rotate over a small key set: the map must keep being written,
				// but Health converts every reader's Stats to a map on each
				// probe, so an ever-growing map makes this O(n^2) work rather
				// than a lock-contention test.
				k.reader["t-"+strconv.Itoa(i%2)] = &statsReader{}
				k.connMu.RUnlock()
				k.mu.Unlock()
			}
		}()

		go func() { // Health's order: must not take k.mu while holding connMu
			defer wg.Done()

			for range iterations {
				_ = k.Health()
			}
		}()

		go func() { // the pending connMu writer that closes the cycle
			defer wg.Done()

			for range iterations {
				func() {
					k.connMu.Lock()
					defer k.connMu.Unlock()
				}()
			}
		}()

		wg.Wait()
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock: Health took k.mu while holding connMu; Subscribe takes them the other way round")
	}
}

type statsReader struct{ Reader }

func (*statsReader) Stats() kafka.ReaderStats { return kafka.ReaderStats{} }
func (*statsReader) Close() error             { return nil }
