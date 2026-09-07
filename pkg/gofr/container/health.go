package container

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"gofr.dev/pkg/gofr/datasource"
)

// Keys under which each backend reports into the health map. They are the field names of the
// health response body, so renaming one breaks every consumer parsing it.
const (
	sqlKey    = "sql"
	redisKey  = "redis"
	pubsubKey = "pubsub"
	llmKey    = "llm"

	mongoKey         = "mongo"
	cassandraKey     = "cassandra"
	clickHouseKey    = "clickHouse"
	kvStoreKey       = "kv-store"
	dgraphKey        = "dgraph"
	openTSDBKey      = "opentsdb"
	elasticsearchKey = "elasticsearch"
	oracleKey        = "oracle"
	couchbaseKey     = "couchbase"
	influxKey        = "influx"
)

// aggregateKeyCount is how many keys appHealth adds on top of the per-backend ones: name and
// version. Only the snapshot buffer size depends on it.
const aggregateKeyCount = 2

// errorDetailKey is the key under which a health detail carries its failure reason, matching the
// spelling datasources already use in their own Health details.
const errorDetailKey = "error"

// healthSingleflightKey names the single in-flight round of checks. There is only ever one, so the
// key is a constant; it exists only because singleflight is keyed by design.
const healthSingleflightKey = "health"

// Health probes every configured backend and returns the aggregate as a map[string]any.
//
// The checks run concurrently, so the call takes as long as the slowest backend rather than the sum
// of all of them. Concurrent calls are collapsed into one round of checks, so probe traffic cannot
// stampede a slow backend, and the result is cached for HEALTH_CACHE_TTL when that is configured.
//
// The returned map is a fresh one, so adding, removing or replacing keys cannot disturb the cached
// result or another caller holding it. The values inside are shared, not copied — reading them is
// safe from any number of callers, writing through one is not, and nothing in GoFr does.
func (c *Container) Health(ctx context.Context) any {
	if c.health == nil {
		// A Container built without Create — a zero value, or a test double — has no cache and no
		// singleflight to share. The checks themselves still run, unbounded, as they always have,
		// and WithoutCancel keeps a disconnecting caller from turning them into a partial result.
		healthMap, _ := c.runHealthChecks(context.WithoutCancel(ctx), 0)

		return healthMap
	}

	if cached, ok := c.health.get(); ok {
		return cloneHealth(cached)
	}

	// context.WithoutCancel: one round of checks is shared by every caller waiting on the
	// singleflight, so whichever caller happens to lead must not cancel the work the others are
	// waiting on by disconnecting. Values — trace and correlation ids — still propagate.
	res, _, _ := c.health.group.Do(healthSingleflightKey, func() (any, error) {
		if cached, ok := c.health.get(); ok {
			return cached, nil
		}

		if !c.health.beginRound() {
			// A previous round timed out and its checks are still running against a backend that
			// has not answered. Report that rather than piling a second set of checks onto it.
			return c.stalledHealth(), nil
		}

		healthMap, complete := c.runHealthChecks(context.WithoutCancel(ctx), c.health.timeout)

		// A partial result is never cached: the backends that did not answer are unknown, not healthy.
		if complete {
			c.health.set(healthMap)
		}

		return healthMap, nil
	})

	healthMap, _ := res.(map[string]any)

	return cloneHealth(healthMap)
}

// runHealthChecks fans out to every configured backend and waits for them. It reports whether every
// check finished; a false means the timeout elapsed first and the map holds only what had been
// recorded by then.
//
// It releases the round claim that Health took with beginRound — split deliberately, because only
// this function knows when the abandoned checks are done, and releasing at the timeout is what the
// guard exists to prevent. The one other caller, Health's no-probe path, has a nil probe, so the
// release is a no-op there rather than a claim it never took.
func (c *Container) runHealthChecks(ctx context.Context, timeout time.Duration) (healthMap map[string]any, complete bool) {
	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	collector := &healthCollector{results: make(map[string]any)}

	var wg sync.WaitGroup

	c.checkPrimaryDatasources(ctx, collector, &wg)
	c.checkExternalDatasources(ctx, collector, &wg)
	c.checkServices(ctx, collector, &wg)

	complete, finished := waitForChecks(ctx, &wg)

	if complete {
		c.health.endRound()
	} else {
		// The abandoned checks are still writing into the collector, so the round is not over even
		// though this caller is done with it. Release it only once they finish, which is what keeps
		// the next probe from starting a second set against the same hung backend. This rides the
		// waiter waitForChecks already started rather than blocking on the same WaitGroup twice.
		go func() {
			<-finished
			c.health.endRound()
		}()
	}

	healthMap, downCount := collector.snapshot()

	if !complete {
		// The backends that did not answer count as one aggregate failure, so the app reports
		// DEGRADED rather than a confident UP built out of a partial map.
		downCount++

		if c.Logger != nil {
			c.Logger.Warnf("health check timed out after %s, reporting %d of the configured backends",
				timeout, len(healthMap))
		}
	}

	c.appHealth(healthMap, downCount)

	return healthMap, complete
}

// waitForChecks blocks until every check has finished or until ctx is done, and reports which of the
// two happened. Checks still outstanding at a timeout are abandoned rather than canceled: the
// health methods of SQL, Redis and PubSub take no context, so there is nothing to cancel them with.
// They keep writing into the collector afterwards, which is why only a snapshot ever leaves it.
// On a timeout it also returns a channel closed when those abandoned checks eventually do finish,
// so the caller can release the round claim then rather than starting a second goroutine of its own
// to wait for the same WaitGroup. It is nil whenever there is nothing left to wait for.
func waitForChecks(ctx context.Context, wg *sync.WaitGroup) (complete bool, finished <-chan struct{}) {
	if ctx.Done() == nil {
		// Nothing can interrupt this round, which is the case whenever no timeout is configured —
		// the default. Wait for it directly rather than making the common path pay for a goroutine
		// and a channel that only the timeout path can use.
		wg.Wait()

		return true, nil
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true, nil
	case <-ctx.Done():
		return false, done
	}
}

func (c *Container) checkPrimaryDatasources(ctx context.Context, collector *healthCollector, wg *sync.WaitGroup) {
	if !isNil(c.SQL) {
		runCheck(wg, collector, sqlKey, func() {
			health := c.SQL.HealthCheck()
			collector.record(sqlKey, health, health.Status == datasource.StatusDown)
		})
	}

	if !isNil(c.Redis) {
		runCheck(wg, collector, redisKey, func() {
			health := c.Redis.HealthCheck()
			collector.record(redisKey, health, health.Status == datasource.StatusDown)
		})
	}

	if c.PubSub != nil {
		runCheck(wg, collector, pubsubKey, func() {
			health := c.PubSub.Health()
			collector.record(pubsubKey, health, health.Status == datasource.StatusDown)
		})
	}

	for name, model := range c.llmModels {
		key := llmKey
		if name != "" {
			key = llmKey + "_" + name
		}

		runCheck(wg, collector, key, func() {
			health := model.HealthCheck(ctx)
			collector.record(key, health, health.Status == datasource.StatusDown)
		})
	}
}

func (c *Container) checkExternalDatasources(ctx context.Context, collector *healthCollector, wg *sync.WaitGroup) {
	datasources := map[string]interface {
		HealthCheck(context.Context) (any, error)
	}{
		mongoKey:         c.Mongo,
		cassandraKey:     c.Cassandra,
		clickHouseKey:    c.Clickhouse,
		kvStoreKey:       c.KVStore,
		dgraphKey:        c.DGraph,
		openTSDBKey:      c.OpenTSDB,
		elasticsearchKey: c.Elasticsearch,
		oracleKey:        c.Oracle,
		couchbaseKey:     c.Couchbase,
		influxKey:        c.InfluxDB,
	}

	for name, ds := range datasources {
		if isNil(ds) {
			continue
		}

		runCheck(wg, collector, name, func() {
			health, err := ds.HealthCheck(ctx)
			collector.record(name, health, err != nil)
		})
	}
}

func (c *Container) checkServices(ctx context.Context, collector *healthCollector, wg *sync.WaitGroup) {
	for name, svc := range c.Services {
		runCheck(wg, collector, name, func() {
			health := svc.HealthCheck(ctx)
			collector.record(name, health, health.Status == datasource.StatusDown)
		})
	}
}

// runCheck runs one backend check on its own goroutine, registered with wg.
//
// The recover is load-bearing, not defensive habit. When these checks ran inline they were under
// the panic-recovery middleware of whichever handler called Health, so a driver that panicked cost
// one request; from a goroutine of our own the same panic takes the process down with it. A backend
// that panics is reported DOWN, which is what it is.
func runCheck(wg *sync.WaitGroup, collector *healthCollector, name string, check func()) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		defer func() {
			if r := recover(); r != nil {
				collector.record(name, datasource.Health{
					Status:  datasource.StatusDown,
					Details: map[string]any{errorDetailKey: fmt.Sprintf("health check panicked: %v", r)},
				}, true)
			}
		}()

		check()
	}()
}

// healthCollector gathers the results of the concurrent checks. Everything is guarded by mu because
// a check abandoned at a timeout keeps writing here after the caller has taken its snapshot.
type healthCollector struct {
	mu        sync.Mutex
	results   map[string]any
	downCount int
}

func (h *healthCollector) record(name string, result any, down bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.results[name] = result

	if down {
		h.downCount++
	}
}

// snapshot copies what has been recorded so far. The copy is what leaves the collector; the
// collector's own map stays private to whichever checks are still writing into it.
func (h *healthCollector) snapshot() (results map[string]any, downCount int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	results = make(map[string]any, len(h.results)+aggregateKeyCount)
	for name, result := range h.results {
		results[name] = result
	}

	return results, h.downCount
}

// cloneHealth copies the health map so that neither the cached result nor a result shared between
// singleflight waiters can be re-keyed by one caller underneath another.
//
// The copy is one level deep, which is the level that matters: the map is what a caller is liable to
// add to. The values stay shared — some are pointers, and each datasource decides its own type, so
// copying them would mean a type switch that silently stops copying whatever is added next.
func cloneHealth(healthMap map[string]any) map[string]any {
	out := make(map[string]any, len(healthMap))
	for name, result := range healthMap {
		out[name] = result
	}

	return out
}

// appHealth writes the aggregate keys onto the map. The "status" key is read back out by
// healthHandler in package gofr, which serves it as the entire public body — renaming it here means
// changing that reader too.
func (c *Container) appHealth(healthMap map[string]any, downCount int) {
	healthMap["name"] = c.GetAppName()
	healthMap["version"] = c.GetAppVersion()

	if downCount == 0 {
		healthMap["status"] = datasource.StatusUp
	} else {
		healthMap["status"] = datasource.StatusDegraded
	}
}

func isNil(i any) bool {
	// Get the value of the interface
	val := reflect.ValueOf(i)

	// If the interface is not assigned or is nil, return true
	return !val.IsValid() || val.IsNil()
}

// stalledHealth is the body returned while a previous round's checks are still outstanding. It
// carries no per-backend keys: what those backends are doing is exactly what is unknown, and
// reporting the last round's values would present stale results as fresh ones.
func (c *Container) stalledHealth() map[string]any {
	healthMap := make(map[string]any, aggregateKeyCount+1)

	// downCount is 1 rather than 0 so the aggregate is DEGRADED: the app is serving, but at least
	// one dependency has not answered within the timeout.
	c.appHealth(healthMap, 1)

	return healthMap
}
