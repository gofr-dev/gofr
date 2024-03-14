# Monitoring Service Health
Health check in microservices refers to a mechanism or process implemented within each service to assess its operational status and readiness to handle requests. It involves regularly querying the service to determine if it is functioning correctly, typically by evaluating its responsiveness and ability to perform essential tasks. Health checks play a critical role in ensuring service availability, detecting failures, preventing cascading issues, and facilitating effective traffic routing in distributed systems.

## GoFr by default registers two endpoints which are :

### 1. Aliveness - /.well-known/alive

It is an endpoint which return the following response when the service is UP with a 200 response code.
```json
{
    "data": {
        "status": "UP"
    }
}
```

It is also used when state of {% new-tab-link title="circuit breaker" href="/docs/advanced-guide/circuit-breaker" /%} is open.

To override the endpoint when registering HTTP Service pass the following option.

```go
&service.HealthConfig{
			HealthEndpoint: "breeds",
		}
```

### 2. Health-Check - /.well-known/health-check

It returns if the service is UP or DOWN along with stats, host, status about the dependent datasources and services.

Sample response of how it appears when all the services, and connected data sources are up.

```json
{
    "data": {
        "anotherService": {
            "status": "UP",
            "details": {
                "host": "localhost:9000"
            }
        },
        "redis": {
            "status": "UP",
            "details": {
                "host": "localhost:2002",
                "stats": {
                    "active_defrag_hits": "0",
                    "active_defrag_key_hits": "0",
                    "active_defrag_key_misses": "0",
                    "active_defrag_misses": "0",
                    "current_active_defrag_time": "0",
                    "current_eviction_exceeded_time": "0",
                    "dump_payload_sanitizations": "0",
                    "evicted_clients": "0",
                    "evicted_keys": "0",
                    "expire_cycle_cpu_milliseconds": "1",
                    "expired_keys": "0",
                    "expired_stale_perc": "0.00",
                    "expired_time_cap_reached_count": "0",
                    "instantaneous_input_kbps": "0.00",
                    "instantaneous_input_repl_kbps": "0.00",
                    "instantaneous_ops_per_sec": "0",
                    "instantaneous_output_kbps": "0.00",
                    "instantaneous_output_repl_kbps": "0.00",
                    "io_threaded_reads_processed": "0",
                    "io_threaded_writes_processed": "0",
                    "keyspace_hits": "0",
                    "keyspace_misses": "0",
                    "latest_fork_usec": "0",
                    "migrate_cached_sockets": "0",
                    "pubsub_channels": "0",
                    "pubsub_patterns": "0",
                    "pubsubshard_channels": "0",
                    "rejected_connections": "0",
                    "reply_buffer_expands": "0",
                    "reply_buffer_shrinks": "1",
                    "slave_expires_tracked_keys": "0",
                    "sync_full": "0",
                    "sync_partial_err": "0",
                    "sync_partial_ok": "0",
                    "total_active_defrag_time": "0",
                    "total_commands_processed": "2",
                    "total_connections_received": "1",
                    "total_error_replies": "2",
                    "total_eviction_exceeded_time": "0",
                    "total_forks": "0",
                    "total_net_input_bytes": "183",
                    "total_net_output_bytes": "257",
                    "total_net_repl_input_bytes": "0",
                    "total_net_repl_output_bytes": "0",
                    "total_reads_processed": "5",
                    "total_writes_processed": "4",
                    "tracking_total_items": "0",
                    "tracking_total_keys": "0",
                    "tracking_total_prefixes": "0",
                    "unexpected_error_replies": "0"
                }
            }
        },
        "sql": {
            "status": "UP",
            "details": {
                "host": "localhost:2001/test",
                "stats": {
                    "maxOpenConnections": 0,
                    "openConnections": 1,
                    "inUse": 0,
                    "idle": 1,
                    "waitCount": 0,
                    "waitDuration": 0,
                    "maxIdleClosed": 0,
                    "maxIdleTimeClosed": 0,
                    "maxLifetimeClosed": 0
                }
            }
        }
    }
}
```
