# Metrics

Metrics are numerical measures that help track the performance and health of systems or applications. They include things like response times, error rates, and resource usage. Metrics are crucial for monitoring, alerting, and improving the overall reliability of a system. Common tools like Prometheus and Grafana are used to collect, store, and visualize metrics.

There are 4 types of metrics published by GoFr
1.Counter
2.Gauge
3.Histogram
4.Summary

For more info please refer [link](https://prometheus.io/docs/concepts/metric_types/)

GoFr by default pushes the following metrics.

## Counter Metrics

A counter is a cumulative metric that represents a single monotonically increasing counter whose value can only increase or be reset to zero on restart. For example, you can use a counter to represent the number of requests served, tasks completed, or errors.

**Default Metrics Pushed By GoFr**

| Metric Name                                | Use                                             |
| ------------------------------------------ | ----------------------------------------------- |
| **zs_deprecated_feature_counter**          | Counts deprecated feature usage.                |
| **zs_server_error**                        | Counts internal server errors.                  |
| **zs_external_service_circuit_open_count** | Counts circuit opens for Gofr's HTTP client.    |
| **zs_notifier_receive_count**              | Counts messages subscribed from a topic.        |
| **zs_notifier_success_count**              | Counts successful records subscribed.           |
| **zs_notifier_failure_count**              | Counts failed message subscriptions.            |
| **zs_pubsub_receive_count**                | Counts total subscribe operations for PubSub.   |
| **zs_pubsub_success_count**                | Counts successful subscribe operations.         |
| **zs_pubsub_failure_count**                | Counts failed subscribe operations.             |
| **zs_pubsub_publish_total_count**          | Counts messages published to a topic in PubSub. |
| **zs_pubsub_publish_success_count**        | Counts successfully published messages.         |
| **zs_pubsub_publish_failure_count**        | Counts failed message publications.             |

## Gauge Metrics

A gauge is a metric that represents a single numerical value that can arbitrarily go up and down.

Gauges are typically used for measured values like temperatures or current memory usage, but also "counts" that can go up and down, like the number of concurrent requests.

**Default Metrics Pushed By GoFr**

Gauge metrics represent dynamic values. Gofr pushes the following gauge metrics:

| Metric Name                  | Use                                                 |
| ---------------------------- | --------------------------------------------------- |
| **zs_info**                  | Counts running pods and framework version.          |
| **zs_go_routines**           | Tracks the number of Go routines.                   |
| **zs_sys_memory_alloc**      | Maintains heap memory allocation.                   |
| **zs_sys_total_alloc**       | Tracks cumulative bytes allocated for heap objects. |
| **zs_go_numGC**              | Counts the number of completed Go cycles.           |
| **zs_go_sys**                | Measures the total bytes of memory.                 |
| **zs_sql_idle_connections**  | Gauges SQL idle connections.                        |
| **zs_sql_inUse_connections** | Gauges SQL in-use connections.                      |
| **zs_sql_open_connections**  | Gauges SQL open connections.                        |

## Histogram Metrics

A histogram samples observations (usually things like request durations or response sizes) and counts them in configurable buckets. It also provides a sum of all observed values.  
A histogram with a base metric name of exposes multiple time series during a scrape:

- cumulative counters for the observation buckets, exposed as \_bucket{le=""}
- the total sum of all observed values, exposed as \_sum
- the count of events that have been observed, exposed as \_count (identical to \_bucket{le="+Inf"} above)

Histogram metrics provide insights into the distribution of data within your application.

**Default Metrics Pushed By GoFr**

| Metric Name                  | Use                                                       |
| ---------------------------- | --------------------------------------------------------- |
| **zs_http_response**         | Measures HTTP response time in seconds.                   |
| **zs_http_service_response** | Measures HTTP response time and status for service calls. |
| **zs_redis_stats**           | Histogram for Redis operations.                           |
| **zs_sql_stats**             | Histogram for SQL operations.                             |
| **zs_cql_stats**             | Histogram for CQL operations.                             |
| **zs_dynamodb_stats**        | Histogram for DynamoDB operations.                        |

### Summary Metrics

A Summary samples observations (usually things like request duration and response sizes). While it also provides a total count of observations and a sum of all observed values.  
A summary with a base metric name exposes multiple time series during a scrape:

- streaming φ-quantiles (0 ≤ φ ≤ 1) of observed events, exposed as {quantile="<φ>"}
- the total sum of all observed values, exposed as \_sum
- the count of events that have been observed, exposed as \_count
