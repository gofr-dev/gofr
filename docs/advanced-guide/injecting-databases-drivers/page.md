---
description: "Inject any database driver into GoFr that satisfies the framework's interface. Keep build size small by including only the datasources your service needs."
nextjs:
  metadata:
    title: "Injecting Database Drivers in GoFr — Pluggable Datasources"
    description: "Inject any database driver into GoFr that satisfies the framework's interface. Keep build size small by including only the datasources your service needs."
---

# Injecting Database Drivers
Keeping in mind the size of the framework in the final build, it felt counter-productive to keep all the database drivers within
the framework itself. Keeping only the most used MySQL and Redis within the framework, users can now inject databases
in the server that satisfies the base interface defined by GoFr. This helps in reducing the build size and in turn build time
as unnecessary database drivers are not being compiled and added to the build.

> We are planning to provide custom drivers for most common databases, and is in the pipeline for upcoming releases!

## Supported Databases

{% table %}

- Datasource
- Health-Check
- Logs
- Metrics
- Traces
- Version-Migrations

---

-  MySQL
- ✅
- ✅
- ✅
- ✅
- ✅

---

-  REDIS
- ✅
- ✅
- ✅
- ✅
- ✅

---

-  PostgreSQL
- ✅
- ✅
- ✅
- ✅
- ✅

---

-  ArangoDB
- ✅
- ✅
- ✅
- ✅
- ✅

---


-  BadgerDB
- ✅
- ✅
- ✅
- ✅
- 

---

-  Cassandra
- ✅
- ✅
- ✅
- ✅
- ✅

---

-  ClickHouse
- 
- ✅
- ✅
- ✅
- ✅

## Why Pluggable Datasources?

GoFr allows developers to inject only the required database drivers into their applications. This helps reduce application build size and improves build performance by avoiding unnecessary dependencies.

This approach also provides flexibility for integrating custom datasource implementations while maintaining compatibility with GoFr interfaces.

## Supported Databases

> We are planning to provide custom drivers for most common databases, and is in the pipeline for upcoming releases!

---

-  DGraph
- ✅
- ✅
- ✅
- ✅
- 

---

-  MongoDB
- ✅
- ✅
- ✅
- ✅
- ✅

---
-  NATS KV
- ✅
- ✅
- ✅
- ✅
-
---

-  OpenTSDB
- ✅
- ✅
- 
- ✅
-
---

-  ScyllaDB
- ✅
- ✅
- ✅
- ✅
-
---

-  Solr
- 
- ✅
- ✅
- ✅
-
---

-  SQLite
- ✅
- ✅
- ✅
- ✅
- ✅
---

-  SurrealDB
- ✅
- ✅
-
- ✅
-
---

















