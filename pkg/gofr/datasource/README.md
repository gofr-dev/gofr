# Datasource 


GoFr provides following features to ensure robust and observable interactions with various data sources:

1. Health Checks

A mechanism for a datasource to self-report its operational status.
New datasources require implementing the HealthCheck() method with the signature:
```go
HealthCheck() datasource.Health
```

This method should return the current health status of the datasource.

2. Retry Mechanism

GoFr attempts to re-establish connections if lost during application runtime.
New datasources should be verified for built-in retry mechanisms. If absent, implement a mechanism for automatic reconnection.

3. Metrics

Datasources should expose relevant metrics for performance monitoring.
The specific metrics to be implemented depend on the datasource type. Discussions are required to determine the appropriate metrics for each new datasource.

4. Logging

GoFr supports level-based logging with the PrettyPrint interface.
New datasources should implement logging with the following levels:
- DEBUG: Logs connection attempts with critical details.
- INFO: Logs successful connection establishment.
- WARN: Logs connection retrying

> Additional logs can be added to enhance debugging and improving user experience.

5. Tracing
    
GoFr supports tracing for all the datasouces, for example for SQL it traces the request using `github.com/XSAM/otelsql`.
If any official package or any widely used package is not available we have to implement our own, but in scope of a different ISSUE.


All logs should include:
- Timestamp
- Request ID (Correlation ID)
- Time taken to execute the query
- Datasource name (consistent with other logs)

## Implementing New Datasources

GoFr offers built-in support for popular datasources like SQL (MySQL, PostgreSQL, SQLite), Redis, and Pub/Sub (MQTT, Kafka, Google as backend). Including additional functionalities within the core GoFr binary would increase the application size unnecessarily.

Therefore, GoFr utilizes a pluggable approach for new datasources by separating implementation in the following way:

- Interface Definition:

   Create an interface with required methods within the datasource package.
   Register the interface with the container (similar to Mongo in https://github.com/tfogo/mongodb-go-tutorial).


- Method Registration:

   Create a method in gofr.go (similar to the existing one) that accepts the newly defined interface.


- Separate Repository:

   Develop a separate repository to implement the interface for the new datasource.
   This approach ensures that the new datasource dependency is only loaded when utilized, minimizing binary size for GoFr applications. It also empowers users to create custom implementations beyond the defaults provided by GoFr.

## Supported Datasources

| Datasource | Health-Check | Logs | Metrics | Traces | As Driver |
|------------|-----------|------|---------|--------|-----------|
| MySQL      | ✅         | ✅    | ✅       | ✅      |           |
| REDIS      | ✅         | ✅    | ✅       | ✅      |           |
| PostgreSQL | ✅         | ✅    | ✅       | ✅      |           |
| MongoDB    | ✅         | ✅    | ✅       |        | ✅         |
| SQLite     | ✅         | ✅    | ✅       | ✅      |           |
| Cassandra  | ✅         | ✅    | ✅       |        | ✅         |
| Clickhouse |           | ✅    | ✅       |        | ✅         |


