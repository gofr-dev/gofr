# Distributed Migration Locking Implementation

This document outlines the implementation details, edge cases, and testing strategies for the distributed migration locking system in GoFr.

## 1. Core Design Principles

The locking mechanism ensures that in a multi-instance environment (e.g., Kubernetes with multiple pods), only one instance executes migrations at any given time across all configured data sources.

### 1.1 Deterministic Ordering (Deadlock Prevention)
When multiple data sources are used (SQL + Redis), pods must acquire locks in the same order to avoid circular wait deadlocks.
- **Implementation**: The `getLockers` function collects all migrators that implement the `Locker` interface and sorts them alphabetically by their `Name()`.
- **Order**: `Redis` -> `SQL`.

### 1.2 Session-Bound SQL Locks
SQL advisory locks must be tied to the specific connection running the migration.
- **Implementation**: `sqlMigrator` starts a dedicated transaction (`lockTx`) during `AcquireLock`. This transaction is held open throughout the migration process, ensuring the database session remains active and the lock is not returned to the connection pool.
- **Cleanup**: The transaction is rolled back in `ReleaseLock`, which automatically releases the session-level advisory lock.

### 1.3 Redis Spin-Lock
Redis locking uses a non-blocking `SETNX` with a TTL.
- **Implementation**: A retry loop with a 500ms backoff is used. If the lock is held, the pod waits and retries up to 10 times (~5 seconds total).
- **TTL**: A 60-second TTL is applied to prevent "zombie" locks if a pod crashes.

---

## 2. Edge Cases Handled

| Case | Handling Strategy |
| :--- | :--- |
| **Partial Acquisition** | If Pod A gets the Redis lock but fails to get the SQL lock, it immediately releases the Redis lock before exiting. |
| **Pod Crash (SIGKILL)** | Redis locks expire via TTL. SQL locks (MySQL/Postgres) are automatically released by the DB when the TCP connection drops. |
| **Connection Pooling** | A dedicated `lockTx` ensures the lock is not "lost" when other queries are run against the pool. |
| **Clock Skew** | TTLs are managed by the server (Redis/DB), not calculated using local pod time. |

---

## 3. Testing Strategy

We use a dedicated Docker Compose environment located in `examples/using-migrations/test-locking/`.

### 3.1 Test Setup
- **Infrastructure**: 1 MySQL, 1 Redis.
- **Application**: 2 Pods (`app1`, `app2`) running the same migration suite.
- **Simulation**: A `time.Sleep` is temporarily added to a migration to simulate a long-running process.

### 3.2 Verification Steps
1. **Concurrency**: Both pods start simultaneously.
2. **Lock Acquisition**: Observe logs to see which pod wins the lock.
3. **Retry Logic**: Verify the second pod logs "Redis lock already held, retrying..." instead of failing immediately.
4. **Mutual Exclusion**: Ensure only one pod logs "running migration X".
5. **Clean Release**: Verify that after the first pod finishes, the second pod can acquire the lock (or see that migrations are already done).

---

## 4. Remaining Modifications & Future Improvements

While the core system is robust, the following enhancements are identified:

### 4.1 Support for Other Data Sources
- **Current**: Mongo, Cassandra, Clickhouse, etc., have no-op lockers.
- **Improvement**: 
    - **Mongo**: Implement locking using a unique index on a `migration_lock` collection.
    - **Cassandra**: Use lightweight transactions (`INSERT ... IF NOT EXISTS`).

### 4.2 Configurable Lock Settings
- **Current**: TTLs and retry counts are hardcoded.
- **Improvement**: Expose these via `configs/.env` (e.g., `MIGRATION_LOCK_TIMEOUT`, `MIGRATION_LOCK_RETRY`).

### 4.3 Global "Skip Lock" Flag
- **Improvement**: Add an environment variable `GOFR_SKIP_MIGRATION_LOCK` for environments where distributed locking is handled externally or not required (e.g., single-node local dev).
