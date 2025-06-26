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

















