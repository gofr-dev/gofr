# Fix Circuit Breaker Concurrency, Error Handling, and Metrics

## Description
This PR addresses critical issues in the Circuit Breaker implementation regarding concurrency safety, error handling logic, and metric reporting.

## Key Changes

### 1. Concurrency Safety & Race Condition Fixes
*   **`pkg/gofr/service/circuit_breaker.go`**: Added necessary locking in `startHealthChecks` and `tryCircuitRecovery`. Previously, `resetCircuit` was being called without holding the lock, leading to potential race conditions on `state` and `failureCount`.
*   **`pkg/gofr/service/metrics_helper.go`**: Implemented a global lock (`metricsMu`) for metric registration to prevent "concurrent map writes" when multiple services initialize metrics simultaneously.

### 2. Robust Edge Case Handling
*   **Thundering Herd Fix**: In `tryCircuitRecovery`, added a check for `cb.state == ClosedState` immediately after acquiring the lock. This prevents queued requests from re-running the health check if another request has already recovered the circuit.
*   **Busy Loop Prevention**: Updated `lastChecked` timestamp *before* attempting a health check in `tryCircuitRecovery`. This ensures that if a health check fails, subsequent requests respect the `interval` wait time instead of immediately retrying in a tight loop.

### 3. Functional Correctness
*   **HTTP 500 Handling**: Updated `executeWithCircuitBreaker` to treat HTTP 500+ status codes as failures. Previously, only network errors (`err != nil`) triggered the circuit breaker, causing it to remain closed during server-side outages.

### 4. Testing & Verification
*   **New Test Case**: Added `TestCircuitBreaker_HTTP500_TripsCircuit` to `pkg/gofr/service/circuit_breaker_test.go` to verify that repeated 500 errors correctly trip the circuit.
*   **Example Update**: Updated `examples/http-server/main.go` to properly read Circuit Breaker configuration (`CB_THRESHOLD`, `RETRY_COUNT`) from environment variables, fixing the integration tests.

## Why these changes are necessary
*   **Stability**: To prevent application crashes (panics) and unpredictable behavior in concurrent environments.
*   **Resilience**: To ensure the Circuit Breaker effectively protects the system against upstream service failures (500 errors), not just network issues.
*   **Performance**: To avoid performance degradation during recovery scenarios (preventing thundering herds and busy loops).
