# Datastore

GoFr supports a number of datastores. To incorporate these, the respective configs must be set. Please refer to [configs](/docs/v1/references/configs).

The client for any database is established via the gofr context after `gofr.New()` is used to establish the connection by checking the configurations.

## Supported Datastores

{% table %}

- Type
- Datastores

---

- **SQL**
- MySQL\
  MSSQL\
  PostgreSQL\
  CockroachDB

---

- **NoSQL**
- MongoDB \
  Cassandra \
  YCQL-Cassandra \
  DynamoDB \
  ElasticSearch

---

- **Cache**
- Redis

---

- **Search Engine**
- Solr

---

- **Message Queues**
- AWS SNS

{% /table %}

The following datastores can be accessed using context:

1. `c.Redis`
2. `c.DB()` returns a `*sql.DB`
3. `c.MongoDB`
4. `c.Cassandra`
5. `c.Kafka`

Please note that these are populated only if the relevant [configurations](/docs/v1/references/configs) is present in the environment. If the configurations for a datastore are not present, it will default to `nil`.
