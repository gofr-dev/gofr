package migration

import (
	"context"
	"database/sql"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/container"
)

// installTrackers wraps each non-nil datasource field in ds with a usage-tracking
// proxy. Returns the map that will be populated with datasource names as keys when
// the corresponding proxy's methods are called.
func installTrackers(ds *Datasource) map[string]bool {
	used := make(map[string]bool)

	type tracker struct {
		isSet func() bool
		wrap  func()
	}

	trackers := []tracker{
		{func() bool { return !isNil(ds.SQL) }, func() { ds.SQL = &trackedSQL{inner: ds.SQL, used: used, key: dsSQL} }},
		{func() bool { return !isNil(ds.Redis) }, func() { ds.Redis = &trackedRedis{inner: ds.Redis, used: used, key: dsRedis} }},
		{func() bool { return !isNil(ds.Clickhouse) }, func() {
			ds.Clickhouse = &trackedClickhouse{inner: ds.Clickhouse, used: used, key: dsClickhouse}
		}},
		{func() bool { return !isNil(ds.Oracle) }, func() { ds.Oracle = &trackedOracle{inner: ds.Oracle, used: used, key: dsOracle} }},
		{func() bool { return !isNil(ds.Cassandra) }, func() {
			ds.Cassandra = &trackedCassandra{inner: ds.Cassandra, used: used, key: dsCassandra}
		}},
		{func() bool { return !isNil(ds.Mongo) }, func() { ds.Mongo = &trackedMongo{inner: ds.Mongo, used: used, key: dsMongo} }},
		{func() bool { return !isNil(ds.ArangoDB) }, func() {
			ds.ArangoDB = &trackedArangoDB{inner: ds.ArangoDB, used: used, key: dsArangoDB}
		}},
		{func() bool { return !isNil(ds.SurrealDB) }, func() {
			ds.SurrealDB = &trackedSurrealDB{inner: ds.SurrealDB, used: used, key: dsSurrealDB}
		}},
		{func() bool { return !isNil(ds.DGraph) }, func() { ds.DGraph = &trackedDGraph{inner: ds.DGraph, used: used, key: dsDGraph} }},
		{func() bool { return !isNil(ds.ScyllaDB) }, func() {
			ds.ScyllaDB = &trackedScyllaDB{inner: ds.ScyllaDB, used: used, key: dsScyllaDB}
		}},
		{func() bool { return !isNil(ds.Elasticsearch) }, func() {
			ds.Elasticsearch = &trackedElasticsearch{inner: ds.Elasticsearch, used: used, key: dsElasticsearch}
		}},
		{func() bool { return !isNil(ds.OpenTSDB) }, func() {
			ds.OpenTSDB = &trackedOpenTSDB{inner: ds.OpenTSDB, used: used, key: dsOpenTSDB}
		}},
	}

	for _, t := range trackers {
		if t.isSet() {
			t.wrap()
		}
	}

	return used
}

func markUsed(used map[string]bool, key string) {
	used[key] = true
}

// --- SQL ---

type trackedSQL struct {
	inner SQL
	used  map[string]bool
	key   string
}

func (t *trackedSQL) Query(query string, args ...any) (*sql.Rows, error) {
	markUsed(t.used, t.key)
	return t.inner.Query(query, args...)
}

func (t *trackedSQL) QueryRow(query string, args ...any) *sql.Row {
	markUsed(t.used, t.key)
	return t.inner.QueryRow(query, args...)
}

func (t *trackedSQL) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	markUsed(t.used, t.key)
	return t.inner.QueryRowContext(ctx, query, args...)
}

func (t *trackedSQL) Exec(query string, args ...any) (sql.Result, error) {
	markUsed(t.used, t.key)
	return t.inner.Exec(query, args...)
}

func (t *trackedSQL) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	markUsed(t.used, t.key)
	return t.inner.ExecContext(ctx, query, args...)
}

// --- Redis ---

type trackedRedis struct {
	inner Redis
	used  map[string]bool
	key   string
}

func (t *trackedRedis) Get(ctx context.Context, key string) *goRedis.StringCmd {
	markUsed(t.used, t.key)
	return t.inner.Get(ctx, key)
}

func (t *trackedRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) *goRedis.StatusCmd {
	markUsed(t.used, t.key)
	return t.inner.Set(ctx, key, value, expiration)
}

func (t *trackedRedis) Del(ctx context.Context, keys ...string) *goRedis.IntCmd {
	markUsed(t.used, t.key)
	return t.inner.Del(ctx, keys...)
}

func (t *trackedRedis) Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd {
	markUsed(t.used, t.key)
	return t.inner.Rename(ctx, key, newKey)
}

// --- Clickhouse ---

type trackedClickhouse struct {
	inner Clickhouse
	used  map[string]bool
	key   string
}

func (t *trackedClickhouse) Exec(ctx context.Context, query string, args ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Exec(ctx, query, args...)
}

func (t *trackedClickhouse) Select(ctx context.Context, dest any, query string, args ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Select(ctx, dest, query, args...)
}

func (t *trackedClickhouse) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	markUsed(t.used, t.key)
	return t.inner.AsyncInsert(ctx, query, wait, args...)
}

func (t *trackedClickhouse) HealthCheck(ctx context.Context) (any, error) {
	markUsed(t.used, t.key)
	return t.inner.HealthCheck(ctx)
}

// --- Oracle ---

type trackedOracle struct {
	inner Oracle
	used  map[string]bool
	key   string
}

func (t *trackedOracle) Select(ctx context.Context, dest any, query string, args ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Select(ctx, dest, query, args...)
}

func (t *trackedOracle) Exec(ctx context.Context, query string, args ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Exec(ctx, query, args...)
}

func (t *trackedOracle) Begin() (container.OracleTx, error) {
	markUsed(t.used, t.key)
	return t.inner.Begin()
}

// --- Cassandra ---

type trackedCassandra struct {
	inner Cassandra
	used  map[string]bool
	key   string
}

func (t *trackedCassandra) Exec(query string, args ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Exec(query, args...)
}

func (t *trackedCassandra) NewBatch(name string, batchType int) error {
	markUsed(t.used, t.key)
	return t.inner.NewBatch(name, batchType)
}

func (t *trackedCassandra) BatchQuery(name, stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.BatchQuery(name, stmt, values...)
}

func (t *trackedCassandra) ExecuteBatch(name string) error {
	markUsed(t.used, t.key)
	return t.inner.ExecuteBatch(name)
}

func (t *trackedCassandra) HealthCheck(ctx context.Context) (any, error) {
	markUsed(t.used, t.key)
	return t.inner.HealthCheck(ctx)
}

// --- Mongo ---

type trackedMongo struct {
	inner Mongo
	used  map[string]bool
	key   string
}

func (t *trackedMongo) Find(ctx context.Context, collection string, filter, results any) error {
	markUsed(t.used, t.key)
	return t.inner.Find(ctx, collection, filter, results)
}

func (t *trackedMongo) FindOne(ctx context.Context, collection string, filter, result any) error {
	markUsed(t.used, t.key)
	return t.inner.FindOne(ctx, collection, filter, result)
}

func (t *trackedMongo) InsertOne(ctx context.Context, collection string, document any) (any, error) {
	markUsed(t.used, t.key)
	return t.inner.InsertOne(ctx, collection, document)
}

func (t *trackedMongo) InsertMany(ctx context.Context, collection string, documents []any) ([]any, error) {
	markUsed(t.used, t.key)
	return t.inner.InsertMany(ctx, collection, documents)
}

func (t *trackedMongo) DeleteOne(ctx context.Context, collection string, filter any) (int64, error) {
	markUsed(t.used, t.key)
	return t.inner.DeleteOne(ctx, collection, filter)
}

func (t *trackedMongo) DeleteMany(ctx context.Context, collection string, filter any) (int64, error) {
	markUsed(t.used, t.key)
	return t.inner.DeleteMany(ctx, collection, filter)
}

func (t *trackedMongo) UpdateByID(ctx context.Context, collection string, id, update any) (int64, error) {
	markUsed(t.used, t.key)
	return t.inner.UpdateByID(ctx, collection, id, update)
}

func (t *trackedMongo) UpdateOne(ctx context.Context, collection string, filter, update any) error {
	markUsed(t.used, t.key)
	return t.inner.UpdateOne(ctx, collection, filter, update)
}

func (t *trackedMongo) UpdateMany(ctx context.Context, collection string, filter, update any) (int64, error) {
	markUsed(t.used, t.key)
	return t.inner.UpdateMany(ctx, collection, filter, update)
}

func (t *trackedMongo) Drop(ctx context.Context, collection string) error {
	markUsed(t.used, t.key)
	return t.inner.Drop(ctx, collection)
}

func (t *trackedMongo) CreateCollection(ctx context.Context, name string) error {
	markUsed(t.used, t.key)
	return t.inner.CreateCollection(ctx, name)
}

func (t *trackedMongo) StartSession() (any, error) {
	markUsed(t.used, t.key)
	return t.inner.StartSession()
}

// --- ArangoDB ---

type trackedArangoDB struct {
	inner ArangoDB
	used  map[string]bool
	key   string
}

func (t *trackedArangoDB) CreateDB(ctx context.Context, database string) error {
	markUsed(t.used, t.key)
	return t.inner.CreateDB(ctx, database)
}

func (t *trackedArangoDB) DropDB(ctx context.Context, database string) error {
	markUsed(t.used, t.key)
	return t.inner.DropDB(ctx, database)
}

func (t *trackedArangoDB) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	markUsed(t.used, t.key)
	return t.inner.CreateCollection(ctx, database, collection, isEdge)
}

func (t *trackedArangoDB) DropCollection(ctx context.Context, database, collection string) error {
	markUsed(t.used, t.key)
	return t.inner.DropCollection(ctx, database, collection)
}

func (t *trackedArangoDB) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	markUsed(t.used, t.key)
	return t.inner.CreateGraph(ctx, database, graph, edgeDefinitions)
}

func (t *trackedArangoDB) DropGraph(ctx context.Context, database, graph string) error {
	markUsed(t.used, t.key)
	return t.inner.DropGraph(ctx, database, graph)
}

// --- SurrealDB ---

type trackedSurrealDB struct {
	inner SurrealDB
	used  map[string]bool
	key   string
}

func (t *trackedSurrealDB) Query(ctx context.Context, query string, vars map[string]any) ([]any, error) {
	markUsed(t.used, t.key)
	return t.inner.Query(ctx, query, vars)
}

func (t *trackedSurrealDB) CreateNamespace(ctx context.Context, namespace string) error {
	markUsed(t.used, t.key)
	return t.inner.CreateNamespace(ctx, namespace)
}

func (t *trackedSurrealDB) CreateDatabase(ctx context.Context, database string) error {
	markUsed(t.used, t.key)
	return t.inner.CreateDatabase(ctx, database)
}

func (t *trackedSurrealDB) DropNamespace(ctx context.Context, namespace string) error {
	markUsed(t.used, t.key)
	return t.inner.DropNamespace(ctx, namespace)
}

func (t *trackedSurrealDB) DropDatabase(ctx context.Context, database string) error {
	markUsed(t.used, t.key)
	return t.inner.DropDatabase(ctx, database)
}

// --- DGraph ---

type trackedDGraph struct {
	inner DGraph
	used  map[string]bool
	key   string
}

func (t *trackedDGraph) ApplySchema(ctx context.Context, schema string) error {
	markUsed(t.used, t.key)
	return t.inner.ApplySchema(ctx, schema)
}

func (t *trackedDGraph) AddOrUpdateField(ctx context.Context, fieldName, fieldType, directives string) error {
	markUsed(t.used, t.key)
	return t.inner.AddOrUpdateField(ctx, fieldName, fieldType, directives)
}

func (t *trackedDGraph) DropField(ctx context.Context, fieldName string) error {
	markUsed(t.used, t.key)
	return t.inner.DropField(ctx, fieldName)
}

// --- ScyllaDB ---

type trackedScyllaDB struct {
	inner ScyllaDB
	used  map[string]bool
	key   string
}

func (t *trackedScyllaDB) Query(dest any, stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Query(dest, stmt, values...)
}

func (t *trackedScyllaDB) QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.QueryWithCtx(ctx, dest, stmt, values...)
}

func (t *trackedScyllaDB) Exec(stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.Exec(stmt, values...)
}

func (t *trackedScyllaDB) ExecWithCtx(ctx context.Context, stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.ExecWithCtx(ctx, stmt, values...)
}

func (t *trackedScyllaDB) ExecCAS(dest any, stmt string, values ...any) (bool, error) {
	markUsed(t.used, t.key)
	return t.inner.ExecCAS(dest, stmt, values...)
}

func (t *trackedScyllaDB) NewBatch(name string, batchType int) error {
	markUsed(t.used, t.key)
	return t.inner.NewBatch(name, batchType)
}

func (t *trackedScyllaDB) NewBatchWithCtx(ctx context.Context, name string, batchType int) error {
	markUsed(t.used, t.key)
	return t.inner.NewBatchWithCtx(ctx, name, batchType)
}

func (t *trackedScyllaDB) BatchQuery(name, stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.BatchQuery(name, stmt, values...)
}

func (t *trackedScyllaDB) BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error {
	markUsed(t.used, t.key)
	return t.inner.BatchQueryWithCtx(ctx, name, stmt, values...)
}

func (t *trackedScyllaDB) ExecuteBatchWithCtx(ctx context.Context, name string) error {
	markUsed(t.used, t.key)
	return t.inner.ExecuteBatchWithCtx(ctx, name)
}

// --- Elasticsearch ---

type trackedElasticsearch struct {
	inner Elasticsearch
	used  map[string]bool
	key   string
}

func (t *trackedElasticsearch) CreateIndex(ctx context.Context, index string, settings map[string]any) error {
	markUsed(t.used, t.key)
	return t.inner.CreateIndex(ctx, index, settings)
}

func (t *trackedElasticsearch) DeleteIndex(ctx context.Context, index string) error {
	markUsed(t.used, t.key)
	return t.inner.DeleteIndex(ctx, index)
}

func (t *trackedElasticsearch) IndexDocument(ctx context.Context, index, id string, document any) error {
	markUsed(t.used, t.key)
	return t.inner.IndexDocument(ctx, index, id, document)
}

func (t *trackedElasticsearch) GetDocument(ctx context.Context, index, id string) (map[string]any, error) {
	markUsed(t.used, t.key)
	return t.inner.GetDocument(ctx, index, id)
}

func (t *trackedElasticsearch) UpdateDocument(ctx context.Context, index, id string, update map[string]any) error {
	markUsed(t.used, t.key)
	return t.inner.UpdateDocument(ctx, index, id, update)
}

func (t *trackedElasticsearch) DeleteDocument(ctx context.Context, index, id string) error {
	markUsed(t.used, t.key)
	return t.inner.DeleteDocument(ctx, index, id)
}

func (t *trackedElasticsearch) Bulk(ctx context.Context, operations []map[string]any) (map[string]any, error) {
	markUsed(t.used, t.key)
	return t.inner.Bulk(ctx, operations)
}

func (t *trackedElasticsearch) Search(ctx context.Context, indices []string, query map[string]any) (map[string]any, error) {
	markUsed(t.used, t.key)
	return t.inner.Search(ctx, indices, query)
}

func (t *trackedElasticsearch) HealthCheck(ctx context.Context) (any, error) {
	markUsed(t.used, t.key)
	return t.inner.HealthCheck(ctx)
}

// --- OpenTSDB ---

type trackedOpenTSDB struct {
	inner OpenTSDB
	used  map[string]bool
	key   string
}

func (t *trackedOpenTSDB) PutDataPoints(ctx context.Context, data any, queryParam string, res any) error {
	markUsed(t.used, t.key)
	return t.inner.PutDataPoints(ctx, data, queryParam, res)
}

func (t *trackedOpenTSDB) PostAnnotation(ctx context.Context, annotation, res any) error {
	markUsed(t.used, t.key)
	return t.inner.PostAnnotation(ctx, annotation, res)
}

func (t *trackedOpenTSDB) PutAnnotation(ctx context.Context, annotation, res any) error {
	markUsed(t.used, t.key)
	return t.inner.PutAnnotation(ctx, annotation, res)
}

func (t *trackedOpenTSDB) DeleteAnnotation(ctx context.Context, annotation, res any) error {
	markUsed(t.used, t.key)
	return t.inner.DeleteAnnotation(ctx, annotation, res)
}
