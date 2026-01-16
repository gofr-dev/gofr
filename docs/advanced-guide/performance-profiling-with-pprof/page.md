# Real-Time Debugging & Performance Profiling with pprof

## Real World News: Performance Issues & The Role of Observability

Performance bottlenecks in high-scale systems can lead to significant financial losses and user dissatisfaction. While observability tools like logs and metrics are essential, they often fall short in diagnosing the *root cause* of complex, intermittent issues.

### Case Studies & Engineering Blogs

1.  **Uber: Saving 70k Cores with GC Tuning & PGO**
    *   **Issue:** As Uber's Go monorepo grew, default Garbage Collection (GC) settings and lack of compiler optimizations led to inefficient resource usage across 30+ mission-critical services.
    *   **Role of Profiling:** Extensive CPU and Heap profiling identified that a significant portion of CPU time was spent in GC cycles and unoptimized code paths.
    *   **Outcome:** By tuning the `GOGC` parameter and implementing Profile-Guided Optimization (PGO), Uber saved an estimated **70,000 CPU cores**.
    *   **Read the full story:** [Uber Engineering: How We Saved 70K Cores Across 30 Mission-Critical Services](https://www.uber.com/blog/how-we-saved-70k-cores-across-30-mission-critical-services/)

2.  **Discord: Investigating Latency Spikes (The Limits of Go)**
    *   **Issue:** Discord's "Read States" service experienced massive latency spikes every 2 minutes.
    *   **Role of Profiling:** Profiling revealed that Go's Garbage Collector was pausing execution to scan a massive LRU cache (millions of objects). While `pprof` correctly identified the GC pressure, the fundamental issue was the memory model for their specific use case.
    *   **Outcome:** This deep insight led them to switch that specific service to Rust (which has no GC), eliminating the spikes. This demonstrates how profiling drives architectural decisions.
    *   **Read the full story:** [Discord: Why Discord is switching from Go to Rust](https://discord.com/blog/why-discord-is-switching-from-go-to-rust)

3.  **Cloudflare: Debugging Complex Memory Leaks**
    *   **Issue:** A Cloudflare service was leaking memory slowly over time, eventually crashing.
    *   **Role of Profiling:** Using `pprof` heap profiles, they compared memory snapshots over time (`base` vs `current`) to identify exactly which objects were accumulating.
    *   **Outcome:** They pinpointed a subtle leak in a library and fixed it, stabilizing the edge service.
    *   **Read the full story:** [Cloudflare: Debugging Go Memory Leaks](https://blog.cloudflare.com/debugging-go-memory-leaks/)

### Research Papers

*   **Google-Wide Profiling: A Continuous Profiling Infrastructure for Data Centers**
    *   *Authors:* Gang Ren, Eric Tune, Tipp Moseley, Yixin Shi, Silvius Rus, Robert Hundt
    *   *Summary:* This foundational paper describes "Google-Wide Profiling" (GWP), the infrastructure that inspired tools like `pprof`. It explains how continuous, low-overhead sampling can be used safely in production data centers to optimize thousands of applications.
    *   *Link:* [Read the Paper (Google Research)](https://research.google/pubs/pub36575/)

---

## 1. Introduction: Why Real-Time Debugging Matters

**Objective:** Understand the limitations of traditional tools and the role of pprof in production debugging.

### Challenges in High-Scale Systems
*   **Latency Sensitivity:** At 50,000+ requests/second, a mere 10ms delay per request can cascade into system-wide congestion.
*   **Intermittent Issues:** Problems like goroutine leaks or transient CPU spikes often disappear before logs or standard metrics can capture them effectively.

### Limitations of Logs & Metrics
*   **Post-Mortem Analysis:**
    *   Logs tell you *what* failed (e.g., HTTP 500 errors).
    *   Metrics (like Prometheus) tell you *when* latency spiked.
*   **Blind Spots:**
    *   They cannot pinpoint *why* a specific function consumed 80% CPU during a specific 30-second window.
    *   They often lack visibility into internal runtime states like garbage collection pressure or specific mutex contention.

### Why pprof?
*   **Live Profiling:** Attach to a running process without needing restarts.
*   **Granular Insights:**
    *   **CPU:** Identify hot functions with nanosecond precision.
    *   **Heap:** Track memory allocation by call stack.
    *   **Goroutines:** Detect leaks or excessive concurrency.
*   **Production-Safe:** Designed to have minimal overhead (typically 1–2% CPU) during the profiling window.

---

## 2. Real-World Scenario: Identifying the Problem with Grafana

**Objective:** Correlate high-level metrics with targeted profiling.

### Grafana Dashboard Analysis
*   **Observed Symptoms:**
    *   **Latency Increase:** 95th percentile API latency jumps from **50ms → 800ms**.
    *   **Resource Utilization:** CPU remains at 40%, but GC pauses spike to **500ms**.
    *   **Uneven Load:** One service instance is handling 70% of the traffic.

### Limitations of Metrics
High latency is a symptom, not a cause. It could stem from:
*   CPU-bound tasks (e.g., heavy JSON parsing).
*   Lock contention (e.g., mutexes in a shared cache).
*   External dependencies (e.g., slow database queries).

### Decision Flow
```mermaid
graph LR
    A[Metrics] --> B[High API Latency]
    B --> C[Start Profiling]
    C --> D[pprof CPU/Heap Profile]
    D --> E[Root Cause]
```

---

## 3. Using pprof for Real-Time Debugging

**Objective:** Profile a live service securely and interpret results.

### Step 1: Integrate pprof
GoFr **automatically enables** pprof by default on the metrics server. You do **not** need to manually import `net/http/pprof` or start a separate HTTP server.

*   **Default Port:** `2121`
*   **Configuration:** You can change the port using the `METRICS_PORT` environment variable.

### Step 2: Secure Access
Since pprof is exposed on the metrics port (default `2121`), it is separate from your main application traffic (default `8000`).
*   **Kubernetes:** Use `kubectl port-forward` to access `:2121` locally.
*   **Firewall:** Ensure port `2121` is not exposed to the public internet.

### Step 3: Profile Types

| Profile | Command | Use Case |
| :--- | :--- | :--- |
| **CPU** | `go tool pprof http://localhost:2121/debug/pprof/profile?seconds=30` | CPU-intensive functions |
| **Heap** | `go tool pprof http://localhost:2121/debug/pprof/heap` | Memory leaks |
| **Goroutines** | `go tool pprof http://localhost:2121/debug/pprof/goroutine` | Goroutine leaks |
| **Mutex** | `go tool pprof http://localhost:2121/debug/pprof/mutex` | Lock contention |

### What Happens During 30-Second Profiling?
*   **CPU Sampling:** Collects stack traces at 100Hz (100 samples/second).
*   **Heap Analysis:** Snapshots in-use memory allocations.
*   **Block Profiling:** Tracks goroutines waiting on locks/synchronization.

---

## 4. Analyzing Data & Fixing the Demo Project

**Objective:** Diagnose and resolve issues in the "BuggyCache" project.

### Issue 1: CPU Overutilization
*   **Profile:** `pprof` CPU profile
*   **Finding:**
    ```text
    Flat  Flat%   Function
    42s   84%     main.fib (recursive Fibonacci)
    ```
*   **Fix:** Replace recursive Fibonacci with an iterative approach.
    ```go
    func fib(n int) int {
      a, b := 0, 1
      for i := 0; i < n; i++ {
        a, b = b, a+b
      }
      return a
    }
    ```

### Issue 2: Memory Leak
*   **Profile:** `pprof` heap profile
*   **Finding:**
    ```text
    1.2GB  98%  main.memoryLeakHandler
    ```
*   **Fix:** Replace global slice with a bounded cache.
    ```go
    var cache = freecache.NewCache(100 * 1024 * 1024) // 100MB limit
    ```

### Issue 3: Goroutine Leak
*   **Profile:** `pprof` goroutine profile
*   **Finding:** `5000+ goroutines` stuck in `time.Sleep`.
*   **Fix:** Add cancellation using `context.Context`.
    ```go
    ctx, cancel := context.WithCancel(r.Context())
    defer cancel()
    go func(ctx context.Context) {
      select {
      case <-ctx.Done(): return // Terminate on request cancellation
      // ...
      }
    }(ctx)
    ```

### Issue 4: Mutex Deadlock
*   **Profile:** `pprof` mutex profile
*   **Finding:** Mutex contention at `main.deadlockHandler`.
*   **Fix:** Remove redundant lock.
    ```go
    func deadlockHandler(...) {
      mu.Lock()
      defer mu.Unlock()
      // Critical section (no nested locks)
    }
    ```

---

## 5. Optimizing JSON Parsing & Validating Impact

**Objective:** Reduce CPU usage by optimizing serialization.

### Case Study: json.Unmarshal Overhead
*   **Problem:** Reflection-based parsing of 10KB payloads consumed **40% CPU**.
*   **Solution:** Use `jsoniter` for zero-allocation parsing.

```go
import "github.com/json-iterator/go"

var j = jsoniter.ConfigCompatibleWithStandardLibrary
func parseJSON(data []byte, v interface{}) error {
  return j.Unmarshal(data, v)
}
```

### Result
*   **CPU usage:** ↓ 30%
*   **Latency:** ↓ from 800ms → 550ms

### Validation with Grafana

| Metric | Before | After |
| :--- | :--- | :--- |
| **CPU Usage** | 75% | 45% |
| **P99 Latency** | 800ms | 550ms |
| **GC Pauses** | 500ms | 200ms |

---

## 6. Best Practices

1.  **Profile in Production:** Staging environments often fail to replicate real-world traffic patterns and data diversity.
2.  **Short Profiling Windows:** Limit CPU profiles to **30–60 seconds** to minimize overhead on the running service.
3.  **Combine with Tracing:** Use OpenTelemetry to correlate profiles with specific requests or traces.

---

## Final Code & Resources

*   **Official Docs:** [golang.org/pkg/net/http/pprof](https://pkg.go.dev/net/http/pprof)
*   **Profiling Guide:** [The Go Blog: Profiling Go Programs](https://go.dev/blog/pprof)
