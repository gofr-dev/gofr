# Metrics Reference

GoFr automatically collects and publishes various metrics to help you monitor your application's health and performance. These metrics are exposed in Prometheus format on the metrics port (default: `2121`) at the `/metrics` endpoint.

## Default Metrics

### Application Metrics

| Name | Type | Description |
| :--- | :--- | :--- |
| `app_go_numGC` | gauge | Number of completed Garbage Collector cycles |
| `app_go_routines` | gauge | Number of Go routines running |
| `app_go_sys` | gauge | Number of total bytes of memory |
| `app_sys_memory_alloc` | gauge | Number of bytes allocated for heap objects |
| `app_sys_total_alloc` | gauge | Number of cumulative bytes allocated for heap objects |
| `app_info` | gauge | Info of app and framework version |

### HTTP Server Metrics

| Name | Type | Description |
| :--- | :--- | :--- |
| `app_http_response` | histogram | Response time of HTTP requests in seconds |

### HTTP Service (Client) Metrics

| Name | Type | Description |
| :--- | :--- | :--- |
| `app_http_service_response` | histogram | Response time of HTTP service requests in seconds |
| `app_http_retry_count` | counter | Total number of retry events |
| `app_http_circuit_breaker_open_count` | counter | Total number of circuit breaker open events |

### Database Metrics

#### SQL
| Name | Type | Description |
| :--- | :--- | :--- |
| `app_sql_open_connections` | gauge | Number of open SQL connections |
| `app_sql_inUse_connections` | gauge | Number of inUse SQL connections |
| `app_sql_stats` | histogram | Response time of SQL queries in milliseconds |

#### Redis
| Name | Type | Description |
| :--- | :--- | :--- |
| `app_redis_stats` | histogram | Response time of Redis commands in milliseconds |

### Pub/Sub Metrics

| Name | Type | Description |
| :--- | :--- | :--- |
| `app_pubsub_publish_total_count` | counter | Number of total publish operations |
| `app_pubsub_publish_success_count` | counter | Number of successful publish operations |
| `app_pubsub_subscribe_total_count` | counter | Number of total subscribe operations |
| `app_pubsub_subscribe_success_count` | counter | Number of successful subscribe operations |

## Custom Metrics

You can also publish custom metrics using the `ctx.Metrics()` manager. For more details, see the [Publishing Custom Metrics](/docs/advanced-guide/publishing-custom-metrics) guide.
